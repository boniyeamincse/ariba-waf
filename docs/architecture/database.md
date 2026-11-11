# Proposed Database Design

**Status:** Partial. Schema version 1 implements accounts/sessions, enrollment, CSRF secret, login attempts, and audit entries. Version 2 adds desired policies/websites, normalized domain bindings, proxy settings, and configuration generation. Both transactional migrations live in `internal/management/store.go`; newer unknown schemas are rejected. The remaining entities below are proposed.
**Store:** SQLite, single instance, local persistent filesystem.

## 1. Design constraints

Use SQLite with WAL mode, bounded transactions, busy timeout, schema migrations, and serialized writes where needed. Never mount one SQLite database into multiple active replicas. Store large private artifacts (certificates, keys, encrypted backups) in restricted files under `/data`; store opaque references and metadata in the database.

The current foundation uses the pure-Go `modernc.org/sqlite` driver, a single connection per process, immediate write transactions, a 5-second busy timeout, foreign keys, WAL, and `synchronous=FULL`. Migrations use `PRAGMA user_version` and preserve existing account/session data. Database directories must be private (`0700`), and database files use `0600`. Desired definitions are validated typed JSON with versions; website policy references use foreign keys, and enabled domain bindings have a partial unique index. Generation, resource writes, and audit records commit together. Configuration revisions, activation, event storage, and backup remain unimplemented.

All timestamps are UTC. Use opaque IDs, foreign keys, uniqueness constraints, explicit retention, and auditability. Never store plaintext passwords, enrollment tokens, session tokens, or backup passphrases.

## 2. Proposed entities

| Entity | Purpose | Important relationships |
| --- | --- | --- |
| `users` | Username, display name, role, enabled state, password hash, version | Owns sessions and audit actions |
| `sessions` | Token hash, user, created/last-seen/expiry/revocation metadata | Belongs to one user |
| `enrollment_tokens` | One-time token hash and expiry/consumption state | Bootstrap only; disabled after initial setup |
| `websites` | Desired site, domains, listeners, upstreams, policy/certificate references, version | References policies and certificates |
| `policies` | Typed engine/CRS/inspection/rate-limit settings, version | May be referenced by multiple sites |
| `exclusions` | Site-scoped rule/target/path/reason/expiry | References a website; activation is revisioned |
| `certificates` | Certificate metadata, ACME auto-renewal state, and private-key storage reference | Referenced by sites; never returns private key |
| `trust_bundles` | CA certificate metadata and storage reference | Referenced by upstream TLS settings |
| `configuration_revisions` | Immutable desired-state snapshot, checksum, generation, validation result | Parent of activation history |
| `activation_jobs` | Durable apply/rollback stage, outcome, recovery state | References target and prior revisions |
| `events` | Sampled and high-severity security incidents and rule match records | Deduplicated by stable transaction identity |
| `hourly_metrics` | Aggregated time-series counters (site, bucket, total, matches, blocks, 4xx/5xx) | Low-overhead reporting for dashboard charts |
| `ingestion_cursors` | Log file identity, offset, rotation state | Supports safe resumption/deduplication |
| `audit_entries` | Actor, action, target, time, redacted change summary | User may be null for system actor |
| `settings` | Versioned typed settings such as trusted proxies and retention | Some settings require activation |
| `idempotency_records` | Principal/operation/key digest and bounded response metadata | Expire after a defined period; no secrets |
| `backups` | Encrypted artifact metadata and retention state | References file; never stores passphrase |
| `restore_jobs` | Upload/validation/restore stage and recovery checkpoint | Serialized against activation and writes |

Names and field-level types are proposals; final DDL must be derived from approved API models and migration tests.

## 3. SQLite runtime configuration & pragmas

For enterprise durability and zero write-starvation under load:
```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA busy_timeout = 5000;
PRAGMA foreign_keys = ON;
PRAGMA auto_vacuum = INCREMENTAL;
```

## 4. State separation

Persist desired site/policy state separately from immutable revision snapshots and applied revision metadata. A write increments the affected record version and global desired generation transactionally. A revision snapshots one generation. Only a verified activation advances applied revision. Rollback changes applied state but does not silently rewrite desired definitions.

## 5. Event storage, high-throughput metrics, and privacy

To avoid SQLite lock contention during attack traffic (e.g., 10,000 req/sec blocked by CRS):
1. **Tier 1 (High-Volume Rollups):** An in-memory ring buffer aggregates request counts, rule matches, and blocks into `hourly_metrics` records. Written in batch every 10–30 seconds.
2. **Tier 2 (High-Severity Incidents):** Only confirmed blocks, high-anomaly detections, and sampled transactions are written to the `events` table with redacted headers/parameters.
3. **Tier 3 (Raw Audit Logs):** Full debug logs are appended directly to structured JSONL files on `/data/logs/` with rolling size-based retention (e.g. 100MB per file, max 10 files) bypassing database transactions.

Apply retention in bounded batches. Disk pressure must trigger visible status and safe retention behavior, not unbounded growth or silent deletion of all audit history.


## 6. Migration and backup rules

- Migrations are ordered, versioned, transactional where possible, and tested from empty and prior supported schemas.
- Interrupted migration behavior is tested before releases.
- Use SQLite online backup facilities or a stopped consistent snapshot; copying only the main database file while WAL is active is not sufficient.
- Back up database and matching configuration/secrets as one consistent set; encrypt backups and test restore into a clean volume.
- Restore validates schema compatibility and paths before extraction, and has a recovery checkpoint.

## 7. Open decisions

Choose the SQLite driver, migration mechanism, exact indexes, encryption-at-rest strategy for sensitive local files, retention scheduler, and database compatibility window. Review file permissions and volume ownership for the non-root runtime.

