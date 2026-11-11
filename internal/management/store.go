package management

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store owns one local SQLite database. Its parent directory must be private.
type Store struct{ db *sql.DB }

// Open creates or opens a persistent database and applies pending migrations.
func Open(path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return nil, err
	}
	info, err := os.Stat(filepath.Dir(abs))
	if err != nil {
		return nil, err
	}
	if info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("database directory must have mode 0700")
	}
	// Reject links rather than following them when opening credential storage.
	if info, err := os.Lstat(abs); err == nil && !info.Mode().IsRegular() {
		return nil, errors.New("database must be a regular file")
	}
	f, err := os.OpenFile(abs, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: abs}
	db, err := sql.Open("sqlite", u.String()+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)&_txlock=immediate")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error                   { return s.db.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) migrate() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > 2 {
		return fmt.Errorf("unsupported database schema version %d", version)
	}
	if version == 0 {
		_, err = tx.Exec(`
CREATE TABLE users (
 id TEXT PRIMARY KEY, username TEXT NOT NULL UNIQUE COLLATE NOCASE,
 display_name TEXT NOT NULL, password_hash TEXT NOT NULL,
 role TEXT NOT NULL CHECK(role IN ('admin','operator','viewer')),
 enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)), version INTEGER NOT NULL DEFAULT 1,
 created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE TABLE enrollment_tokens (hash TEXT PRIMARY KEY, expires_at INTEGER NOT NULL);
CREATE TABLE sessions (
 id TEXT PRIMARY KEY, hash TEXT NOT NULL UNIQUE, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 created_at INTEGER NOT NULL, last_seen INTEGER NOT NULL, expires_at INTEGER NOT NULL,
 reauthenticated_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX sessions_user ON sessions(user_id);
CREATE TABLE secrets (name TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE enrollment_retries (key_hash TEXT PRIMARY KEY, request_hash TEXT NOT NULL, user_id TEXT NOT NULL, expires_at INTEGER NOT NULL);
CREATE TABLE login_attempts (id INTEGER PRIMARY KEY, account TEXT NOT NULL, ip TEXT NOT NULL, at INTEGER NOT NULL);
CREATE INDEX login_attempts_time ON login_attempts(at);
CREATE TABLE audit_entries (id INTEGER PRIMARY KEY, actor_id TEXT, action TEXT NOT NULL, target_id TEXT, at INTEGER NOT NULL);
PRAGMA user_version = 1;`)
		if err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT INTO secrets VALUES ('csrf', ?)", randomToken()); err != nil {
			return err
		}
	}
	if version < 2 {
		if _, err := tx.Exec(`
CREATE TABLE configuration_state (
 id INTEGER PRIMARY KEY CHECK(id=1), desired_generation INTEGER NOT NULL,
 applied_revision_id TEXT, applied_generation INTEGER
);
INSERT INTO configuration_state VALUES (1,0,NULL,NULL);
CREATE TABLE proxy_settings (
 id INTEGER PRIMARY KEY CHECK(id=1), version INTEGER NOT NULL, body TEXT NOT NULL CHECK(json_valid(body))
);
INSERT INTO proxy_settings VALUES (1,1,'{"allowedCidrs":[],"trustedProxyCidrs":[]}');
CREATE TABLE policies (
 id TEXT PRIMARY KEY, version INTEGER NOT NULL, body TEXT NOT NULL CHECK(json_valid(body)),
 created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX policies_order ON policies(created_at,id);
CREATE TABLE websites (
 id TEXT PRIMARY KEY, version INTEGER NOT NULL, body TEXT NOT NULL CHECK(json_valid(body)),
 policy_id TEXT NOT NULL REFERENCES policies(id) ON DELETE RESTRICT,
 created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX websites_order ON websites(created_at,id);
CREATE INDEX websites_policy ON websites(policy_id);
CREATE TABLE website_domains (
 site_id TEXT NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
 domain TEXT NOT NULL, enabled INTEGER NOT NULL CHECK(enabled IN (0,1)),
 PRIMARY KEY(site_id,domain)
);
CREATE UNIQUE INDEX active_domains ON website_domains(domain) WHERE enabled=1;
PRAGMA user_version = 2;`); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// CreateEnrollmentToken is a local-only operation. It replaces any unused token.
func (s *Store) CreateEnrollmentToken(ctx context.Context) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil {
		return "", err
	}
	if count != 0 {
		return "", errors.New("initial setup is already complete")
	}
	token := randomToken()
	if _, err := tx.ExecContext(ctx, "DELETE FROM enrollment_tokens"); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO enrollment_tokens VALUES (?, ?)", tokenHash(token), time.Now().Add(10*time.Minute).Unix()); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return token, nil
}

type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	Enabled     bool   `json:"enabled"`
	Version     int    `json:"version"`
}

const userColumns = "id, username, display_name, role, enabled, version"

type scanner interface{ Scan(...any) error }

func scanUser(row scanner) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Enabled, &u.Version)
	return u, err
}

func audit(ctx context.Context, tx *sql.Tx, actor, action, target string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO audit_entries(actor_id, action, target_id, at) VALUES (?, ?, ?, ?)", actor, action, target, time.Now().Unix())
	return err
}
