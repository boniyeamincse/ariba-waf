package management

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func proxyResponse(spec proxySpec, version int, state configurationStamp) map[string]any {
	return map[string]any{"allowedCidrs": spec.AllowedCIDRs, "trustedProxyCidrs": spec.TrustedProxyCIDRs, "version": version,
		"desiredGeneration": state.DesiredGeneration, "appliedRevisionId": state.AppliedRevisionID, "pendingChanges": state.PendingChanges}
}

func (s *Server) getProxy(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, true); !ok {
		return
	}
	tx, ok := s.readTransaction(w, r)
	if !ok {
		return
	}
	defer tx.Rollback()
	spec, version, err := loadProxy(r.Context(), tx)
	if storageFailure(w, r, err) {
		return
	}
	state, err := readStamp(r.Context(), tx)
	if storageFailure(w, r, err) || storageFailure(w, r, tx.Commit()) {
		return
	}
	w.Header().Set("ETag", fmt.Sprintf(`"proxy-v%d"`, version))
	respond(w, r, 200, proxyResponse(spec, version, state))
}

func (s *Server) patchProxy(w http.ResponseWriter, r *http.Request) {
	i, ok := s.authenticate(w, r, true)
	if !ok {
		return
	}
	tx, ok := s.adminTransaction(w, r, i)
	if !ok {
		return
	}
	defer tx.Rollback()
	spec, version, err := loadProxy(r.Context(), tx)
	if storageFailure(w, r, err) || !requireVersion(w, r, fmt.Sprintf(`"proxy-v%d"`, version)) {
		return
	}
	spec, ok = decodeSpec(w, r, spec, true)
	if !ok {
		return
	}
	if code := normalizeProxy(&spec); code != "" {
		failure(w, r, 422, code)
		return
	}
	rows, err := tx.QueryContext(r.Context(), "SELECT body FROM websites")
	if storageFailure(w, r, err) {
		return
	}
	var sites []websiteSpec
	for rows.Next() {
		var raw string
		var site websiteSpec
		if err = rows.Scan(&raw); err != nil {
			break
		}
		if err = json.Unmarshal([]byte(raw), &site); err != nil {
			break
		}
		sites = append(sites, site)
	}
	rowErr := rows.Err()
	rows.Close()
	if storageFailure(w, r, err) || storageFailure(w, r, rowErr) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	for _, site := range sites {
		if code := s.validateUpstreams(ctx, site.Upstreams, spec); code != "" {
			failure(w, r, 409, "UPSTREAM_POLICY_CONFLICT")
			return
		}
	}
	body, _ := json.Marshal(spec)
	version++
	_, err = tx.ExecContext(r.Context(), "UPDATE proxy_settings SET body=?,version=? WHERE id=1", string(body), version)
	if storageFailure(w, r, err) {
		return
	}
	state, err := advanceConfiguration(r.Context(), tx, i.User.ID, "settings.proxy", "proxy")
	if storageFailure(w, r, err) || storageFailure(w, r, tx.Commit()) {
		return
	}
	w.Header().Set("ETag", fmt.Sprintf(`"proxy-v%d"`, version))
	respond(w, r, 200, proxyResponse(spec, version, state))
}
