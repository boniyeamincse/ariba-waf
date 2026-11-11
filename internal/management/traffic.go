package management

import (
	"context"
	"crypto/hmac"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type resourceKind string

const (
	policies resourceKind = "policies"
	websites resourceKind = "websites"
)

func (s *Server) registerTrafficRoutes(mux *http.ServeMux) {
	for _, kind := range []resourceKind{policies, websites} {
		mux.HandleFunc("GET /api/v1/"+string(kind), func(w http.ResponseWriter, r *http.Request) { s.listTraffic(w, r, kind) })
		mux.HandleFunc("GET /api/v1/"+string(kind)+"/{id}", func(w http.ResponseWriter, r *http.Request) { s.getTraffic(w, r, kind) })
		mux.HandleFunc("POST /api/v1/"+string(kind), func(w http.ResponseWriter, r *http.Request) { s.writeTraffic(w, r, kind, false) })
		mux.HandleFunc("PATCH /api/v1/"+string(kind)+"/{id}", func(w http.ResponseWriter, r *http.Request) { s.writeTraffic(w, r, kind, true) })
		mux.HandleFunc("DELETE /api/v1/"+string(kind)+"/{id}", func(w http.ResponseWriter, r *http.Request) { s.deleteTraffic(w, r, kind) })
	}
	mux.HandleFunc("GET /api/v1/settings/proxy", s.getProxy)
	mux.HandleFunc("PATCH /api/v1/settings/proxy", s.patchProxy)
	mux.HandleFunc("GET /api/v1/configuration/status", s.configurationStatus)
}

type configurationStamp struct {
	DesiredGeneration int64   `json:"desiredGeneration"`
	AppliedRevisionID *string `json:"appliedRevisionId"`
	PendingChanges    bool    `json:"pendingChanges"`
}

func readStamp(ctx context.Context, tx *sql.Tx) (configurationStamp, error) {
	var state configurationStamp
	var applied *int64
	err := tx.QueryRowContext(ctx, "SELECT desired_generation,applied_revision_id,applied_generation FROM configuration_state WHERE id=1").Scan(&state.DesiredGeneration, &state.AppliedRevisionID, &applied)
	state.PendingChanges = applied == nil && state.DesiredGeneration > 0 || applied != nil && *applied != state.DesiredGeneration
	return state, err
}

func advanceConfiguration(ctx context.Context, tx *sql.Tx, actor, action, id string) (configurationStamp, error) {
	if _, err := tx.ExecContext(ctx, "UPDATE configuration_state SET desired_generation=desired_generation+1 WHERE id=1"); err != nil {
		return configurationStamp{}, err
	}
	if err := audit(ctx, tx, actor, action, id); err != nil {
		return configurationStamp{}, err
	}
	return readStamp(ctx, tx)
}

type trafficResource struct {
	ID      string
	Version int
	Body    string
	Created int64
}

func (v trafficResource) etag() string { return fmt.Sprintf(`"%s-v%d"`, v.ID, v.Version) }
func (v trafficResource) response(state configurationStamp) (map[string]any, error) {
	var fields map[string]any
	if err := json.Unmarshal([]byte(v.Body), &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, errors.New("invalid stored resource")
	}
	fields["id"] = v.ID
	fields["version"] = v.Version
	fields["desiredGeneration"] = state.DesiredGeneration
	fields["appliedRevisionId"] = state.AppliedRevisionID
	fields["pendingChanges"] = state.PendingChanges
	return fields, nil
}

func readTraffic(ctx context.Context, tx *sql.Tx, kind resourceKind, id string) (trafficResource, error) {
	var v trafficResource
	err := tx.QueryRowContext(ctx, "SELECT id,version,body,created_at FROM "+string(kind)+" WHERE id=?", id).Scan(&v.ID, &v.Version, &v.Body, &v.Created)
	return v, err
}

func requireVersion(w http.ResponseWriter, r *http.Request, tag string) bool {
	if r.Header.Get("If-Match") == "" {
		failure(w, r, 428, "PRECONDITION_REQUIRED")
		return false
	}
	if r.Header.Get("If-Match") != tag {
		failure(w, r, 412, "VERSION_CONFLICT")
		return false
	}
	return true
}

func storageFailure(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		failure(w, r, 404, "NOT_FOUND")
	} else {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
	}
	return true
}

func (s *Server) readTransaction(w http.ResponseWriter, r *http.Request) (*sql.Tx, bool) {
	tx, err := s.store.db.BeginTx(r.Context(), nil)
	if storageFailure(w, r, err) {
		return nil, false
	}
	return tx, true
}

func (s *Server) trafficIdentity(w http.ResponseWriter, r *http.Request) (identity, bool) {
	i, ok := s.authenticate(w, r, false)
	if ok && i.User.Role == "viewer" {
		failure(w, r, 403, "FORBIDDEN")
		return i, false
	}
	return i, ok
}

func (s *Server) getTraffic(w http.ResponseWriter, r *http.Request, kind resourceKind) {
	if _, ok := s.authenticate(w, r, false); !ok {
		return
	}
	tx, ok := s.readTransaction(w, r)
	if !ok {
		return
	}
	defer tx.Rollback()
	v, err := readTraffic(r.Context(), tx, kind, r.PathValue("id"))
	if storageFailure(w, r, err) {
		return
	}
	state, err := readStamp(r.Context(), tx)
	if storageFailure(w, r, err) {
		return
	}
	if storageFailure(w, r, tx.Commit()) {
		return
	}
	data, err := v.response(state)
	if storageFailure(w, r, err) {
		return
	}
	w.Header().Set("ETag", v.etag())
	respond(w, r, 200, data)
}

func (s *Server) page(w http.ResponseWriter, r *http.Request, scope string) (int, int64, string, bool) {
	limit := 50
	var since int64
	var after string
	for key, values := range r.URL.Query() {
		if (key != "limit" && key != "cursor") || len(values) != 1 {
			failure(w, r, 400, "INVALID_QUERY")
			return 0, 0, "", false
		}
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 200 {
			failure(w, r, 400, "INVALID_QUERY")
			return 0, 0, "", false
		}
		limit = n
	}
	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		parts := strings.Split(cursor, ".")
		if len(cursor) > 256 || len(parts) != 2 || !hmac.Equal([]byte(parts[1]), []byte(s.proof(scope+":"+parts[0]))) {
			failure(w, r, 400, "INVALID_QUERY")
			return 0, 0, "", false
		}
		decoded, err := base64.RawURLEncoding.DecodeString(parts[0])
		fields := strings.Split(string(decoded), ":")
		if err != nil || len(fields) != 2 {
			failure(w, r, 400, "INVALID_QUERY")
			return 0, 0, "", false
		}
		since, err = strconv.ParseInt(fields[0], 10, 64)
		if err != nil || since < 0 {
			failure(w, r, 400, "INVALID_QUERY")
			return 0, 0, "", false
		}
		after = fields[1]
	}
	return limit, since, after, true
}

func (s *Server) listTraffic(w http.ResponseWriter, r *http.Request, kind resourceKind) {
	if _, ok := s.authenticate(w, r, false); !ok {
		return
	}
	limit, since, after, ok := s.page(w, r, string(kind))
	if !ok {
		return
	}
	tx, ok := s.readTransaction(w, r)
	if !ok {
		return
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(r.Context(), "SELECT id,version,body,created_at FROM "+string(kind)+" WHERE created_at>? OR (created_at=? AND id>?) ORDER BY created_at,id LIMIT ?", since, since, after, limit+1)
	if storageFailure(w, r, err) {
		return
	}
	items := []trafficResource{}
	for rows.Next() {
		var v trafficResource
		if err = rows.Scan(&v.ID, &v.Version, &v.Body, &v.Created); err != nil {
			break
		}
		items = append(items, v)
	}
	rowErr := rows.Err()
	rows.Close()
	if storageFailure(w, r, err) || storageFailure(w, r, rowErr) {
		return
	}
	state, err := readStamp(r.Context(), tx)
	if storageFailure(w, r, err) {
		return
	}
	if storageFailure(w, r, tx.Commit()) {
		return
	}
	var next any
	if len(items) > limit {
		items = items[:limit]
		v := items[limit-1]
		payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%s", v.Created, v.ID)))
		next = payload + "." + s.proof(string(kind)+":"+payload)
	}
	data := []map[string]any{}
	for _, v := range items {
		fields, err := v.response(state)
		if storageFailure(w, r, err) {
			return
		}
		data = append(data, fields)
	}
	respondWithMeta(w, r, 200, data, map[string]any{"nextCursor": next, "desiredGeneration": state.DesiredGeneration})
}

func (s *Server) writeTraffic(w http.ResponseWriter, r *http.Request, kind resourceKind, patch bool) {
	i, ok := s.trafficIdentity(w, r)
	if !ok {
		return
	}
	tx, ok := s.roleTransaction(w, r, i, false)
	if !ok {
		return
	}
	defer tx.Rollback()
	v := trafficResource{ID: randomToken(), Version: 1}
	if patch {
		var err error
		v, err = readTraffic(r.Context(), tx, kind, r.PathValue("id"))
		if storageFailure(w, r, err) || !requireVersion(w, r, v.etag()) {
			return
		}
		v.Version++
	}
	var body []byte
	var site websiteSpec
	if kind == policies {
		base := defaultPolicy()
		if patch && storageFailure(w, r, json.Unmarshal([]byte(v.Body), &base)) {
			return
		}
		p, ok := decodeSpec(w, r, base, patch)
		if !ok {
			return
		}
		if code := validatePolicy(p); code != "" {
			failure(w, r, 422, code)
			return
		}
		body, _ = json.Marshal(p)
	} else {
		base := defaultWebsite()
		if patch && storageFailure(w, r, json.Unmarshal([]byte(v.Body), &base)) {
			return
		}
		var decoded bool
		site, decoded = decodeSpec(w, r, base, patch)
		if !decoded {
			return
		}
		if code := normalizeWebsite(&site); code != "" {
			failure(w, r, 422, code)
			return
		}
		if !s.validateSiteReferences(w, r, tx, site, !patch) {
			return
		}
		body, _ = json.Marshal(site)
	}
	v.Body = string(body)
	if !s.persistTraffic(w, r, tx, kind, v, site, patch) {
		return
	}
	verb := "create"
	if patch {
		verb = "update"
	}
	state, err := advanceConfiguration(r.Context(), tx, i.User.ID, string(kind)+"."+verb, v.ID)
	if storageFailure(w, r, err) || storageFailure(w, r, tx.Commit()) {
		return
	}
	status := 200
	if !patch {
		status = 201
		w.Header().Set("Location", "/api/v1/"+string(kind)+"/"+v.ID)
	}
	data, err := v.response(state)
	if storageFailure(w, r, err) {
		return
	}
	w.Header().Set("ETag", v.etag())
	respond(w, r, status, data)
}

func (s *Server) validateSiteReferences(w http.ResponseWriter, r *http.Request, tx *sql.Tx, site websiteSpec, creating bool) bool {
	p, err := readTraffic(r.Context(), tx, policies, site.PolicyID)
	if errors.Is(err, sql.ErrNoRows) {
		failure(w, r, 422, "POLICY_NOT_FOUND")
		return false
	}
	if storageFailure(w, r, err) {
		return false
	}
	var spec policySpec
	if storageFailure(w, r, json.Unmarshal([]byte(p.Body), &spec)) {
		return false
	}
	if creating && spec.EngineMode != "detectionOnly" {
		failure(w, r, 422, "INITIAL_POLICY_MUST_DETECT")
		return false
	}
	settings, _, err := loadProxy(r.Context(), tx)
	if storageFailure(w, r, err) {
		return false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if code := s.validateUpstreams(ctx, site.Upstreams, settings); code != "" {
		failure(w, r, 422, code)
		return false
	}
	return true
}

func (s *Server) persistTraffic(w http.ResponseWriter, r *http.Request, tx *sql.Tx, kind resourceKind, v trafficResource, site websiteSpec, patch bool) bool {
	if kind == websites && site.Enabled {
		for _, domain := range site.Domains {
			var n int
			err := tx.QueryRowContext(r.Context(), "SELECT count(*) FROM website_domains WHERE domain=? AND enabled=1 AND site_id<>?", domain, v.ID).Scan(&n)
			if storageFailure(w, r, err) {
				return false
			}
			if n > 0 {
				failure(w, r, 409, "DOMAIN_CONFLICT")
				return false
			}
		}
	}
	var err error
	if kind == policies {
		if patch {
			_, err = tx.ExecContext(r.Context(), "UPDATE policies SET body=?,version=? WHERE id=?", v.Body, v.Version, v.ID)
		} else {
			_, err = tx.ExecContext(r.Context(), "INSERT INTO policies(id,version,body) VALUES (?,?,?)", v.ID, v.Version, v.Body)
		}
	} else {
		if patch {
			_, err = tx.ExecContext(r.Context(), "UPDATE websites SET body=?,version=?,policy_id=? WHERE id=?", v.Body, v.Version, site.PolicyID, v.ID)
		} else {
			_, err = tx.ExecContext(r.Context(), "INSERT INTO websites(id,version,body,policy_id) VALUES (?,?,?,?)", v.ID, v.Version, v.Body, site.PolicyID)
		}
	}
	if storageFailure(w, r, err) {
		return false
	}
	if kind == websites {
		if _, err = tx.ExecContext(r.Context(), "DELETE FROM website_domains WHERE site_id=?", v.ID); storageFailure(w, r, err) {
			return false
		}
		for _, domain := range site.Domains {
			if _, err = tx.ExecContext(r.Context(), "INSERT INTO website_domains VALUES (?,?,?)", v.ID, domain, site.Enabled); storageFailure(w, r, err) {
				return false
			}
		}
	}
	return true
}

func (s *Server) deleteTraffic(w http.ResponseWriter, r *http.Request, kind resourceKind) {
	i, ok := s.trafficIdentity(w, r)
	if !ok {
		return
	}
	tx, ok := s.roleTransaction(w, r, i, false)
	if !ok {
		return
	}
	defer tx.Rollback()
	v, err := readTraffic(r.Context(), tx, kind, r.PathValue("id"))
	if storageFailure(w, r, err) || !requireVersion(w, r, v.etag()) {
		return
	}
	if kind == policies {
		var n int
		if storageFailure(w, r, tx.QueryRowContext(r.Context(), "SELECT count(*) FROM websites WHERE policy_id=?", v.ID).Scan(&n)) {
			return
		}
		if n > 0 {
			failure(w, r, 409, "RESOURCE_IN_USE")
			return
		}
	}
	_, err = tx.ExecContext(r.Context(), "DELETE FROM "+string(kind)+" WHERE id=?", v.ID)
	if storageFailure(w, r, err) {
		return
	}
	_, err = advanceConfiguration(r.Context(), tx, i.User.ID, string(kind)+".delete", v.ID)
	if storageFailure(w, r, err) || storageFailure(w, r, tx.Commit()) {
		return
	}
	w.WriteHeader(204)
}

func (s *Server) configurationStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, false); !ok {
		return
	}
	tx, ok := s.readTransaction(w, r)
	if !ok {
		return
	}
	defer tx.Rollback()
	state, err := readStamp(r.Context(), tx)
	if storageFailure(w, r, err) || storageFailure(w, r, tx.Commit()) {
		return
	}
	respond(w, r, 200, struct {
		configurationStamp
		CurrentJob                  any  `json:"currentJob"`
		ActivationExecutorAvailable bool `json:"activationExecutorAvailable"`
	}{configurationStamp: state})
}
