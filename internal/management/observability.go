package management

import (
	"database/sql"
	"net/http"
	"time"
)

type eventRecord struct {
	ID         string `json:"id"`
	HappenedAt string `json:"happenedAt"`
	Action     string `json:"action"`
	Message    string `json:"message"`
}

type auditRecord struct {
	ID       int64  `json:"id"`
	ActorID  string `json:"actorId,omitempty"`
	Action   string `json:"action"`
	TargetID string `json:"targetId,omitempty"`
	At       string `json:"at"`
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, false); !ok {
		return
	}

	// Event ingestion pipeline is pending. Keep the route stable for clients.
	respond(w, r, 200, []eventRecord{})
}

func (s *Server) metricsOverview(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, false); !ok {
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	respond(w, r, 200, map[string]any{
		"window": map[string]string{
			"from": time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339),
			"to":   now,
		},
		"counters": map[string]int64{
			"requests":   0,
			"detections": 0,
			"blocks":     0,
		},
		"freshness": map[string]any{
			"status":      "unavailable",
			"lastUpdated": nil,
		},
	})
}

func (s *Server) auditLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, true); !ok {
		return
	}

	rows, err := s.store.db.QueryContext(
		r.Context(),
		"SELECT id, actor_id, action, target_id, at FROM audit_entries ORDER BY id DESC LIMIT 200",
	)
	if err != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}
	defer rows.Close()

	items := []auditRecord{}
	for rows.Next() {
		var item auditRecord
		var actor sql.NullString
		var target sql.NullString
		var at int64
		if err := rows.Scan(&item.ID, &actor, &item.Action, &target, &at); err != nil {
			failure(w, r, 503, "STORAGE_UNAVAILABLE")
			return
		}
		if actor.Valid {
			item.ActorID = actor.String
		}
		if target.Valid {
			item.TargetID = target.String
		}
		item.At = time.Unix(at, 0).UTC().Format(time.RFC3339)
		items = append(items, item)
	}
	if rows.Err() != nil {
		failure(w, r, 503, "STORAGE_UNAVAILABLE")
		return
	}

	respond(w, r, 200, items)
}
