# Ariba Shield WAF — Management API

**Status:** Proposed development contract; routes are not implemented or runtime-verified.  
**API version:** `v1`  
**Implementation target:** Go API, embedded SQLite, one-container release.

This page is the API entry point in the development documentation. The detailed draft contract, including example payloads, validation limits, role matrix, error codes, events, and backup/restore behavior, is maintained in [the API specification](../api.md). Update both documents when the API contract changes; once the documentation reorganization is complete, consolidate the detailed specification here and remove the temporary legacy location.

## 1. Purpose and boundaries

The management API configures Ariba Shield WAF and serves its dashboard. It is distinct from the protected websites' APIs. Public website listeners must never forward requests to the management API; `/api` on a protected virtual host belongs to that upstream application.

The API is served to the browser on the management HTTPS origin under `/api/v1`. In local development, the Next.js dev server proxies `/api/v1` to the Go API on loopback. The internal Go listener is not published as a host port in the release container.

| Environment | Browser-facing API | Internal API listener |
| --- | --- | --- |
| Local development | `http://localhost:3000/api/v1` | `http://127.0.0.1:9000/api/v1` |
| Packaged container | `https://localhost:9443/api/v1` | `http://127.0.0.1:9000/api/v1`, loopback only |

## 2. API groups

All paths below are proposed route families, not evidence of implemented routes. The detailed draft in [the API specification](../api.md) defines the proposed methods and payloads.

| Group | Responsibilities |
| --- | --- |
| Setup and authentication | Bootstrap status, one-time enrollment, login/logout, sessions, password changes, CSRF token |
| Users | Admin-managed accounts, roles, enable/disable, session revocation |
| Websites | Domains, listeners, upstreams, TLS policy, certificates, assigned protection policy |
| Policies and exclusions | Detection mode, CRS thresholds, inspection limits, rate limits, site-scoped exceptions |
| Certificates and trust bundles | Upload and validate site certificates/keys and upstream trust material |
| Configuration | Desired generation, immutable revisions, validation, apply, rollback, durable jobs |
| Events and metrics | Redacted security events, request/block/detection metrics, ingestion freshness |
| System settings | Health, version, trusted proxies, destination allowlists, retention |
| Backup and restore | Protected backup jobs, encrypted artifact download, validation, restore and recovery |

## 3. Shared contract

- Use versioned `/api/v1` routes and JSON with `camelCase` fields.
- Use UTC RFC 3339 timestamps; return opaque server-generated IDs.
- Bound request bodies, query windows, pagination, event sizes, and job output. Reject unknown JSON fields.
- Use a consistent success envelope (`data`, optional `meta`) and sanitized error envelope (`error`, `meta`). Include a request ID for support and audit correlation.
- Use cursor pagination with stable ordering. Event time ranges are `from` inclusive and `to` exclusive.
- Mutable resources expose a version/ETag. Require `If-Match` to prevent lost updates; use `428` when missing and `412` for stale versions.
- Use idempotency keys for operations that create enrollment, apply/rollback, backup, or restore jobs.
- Return `202 Accepted` and a durable job identifier for long-running work; clients poll the job resource rather than treating enqueue as completion.
- Do not expose shell commands, filesystem paths, private keys, password hashes, session secrets, raw request bodies, or unredacted security data.

Exact response fields, status/error codes, and limits remain proposed until checked against implementation and tests. Do not silently relax the documented contract to match an unreviewed implementation.

## 4. Authentication and authorization

Use opaque server-side sessions in Secure, HttpOnly, SameSite cookies; do not store bearer credentials in browser local storage. All state-changing browser requests require CSRF validation and an allowed Origin. GET requests must not change state. Avoid credentialed wildcard CORS.

The proposed roles are:

| Capability | Admin | Operator | Viewer |
| --- | --- | --- | --- |
| Read configuration, events, and metrics | Yes | Yes | Yes |
| Edit websites, policies, and exclusions | Yes | Yes | No |
| Validate/apply/rollback traffic configuration | Yes | Yes | No |
| Manage users, secrets, trust settings, and retention | Yes | No | No |
| Create/download backups and restore | Yes | No | No |

Authorization is enforced by the API, not just hidden in the dashboard. Setup enrollment is a one-time bootstrap path and must be disabled after initial account creation. Require recent reauthentication for especially sensitive operations such as restore and private-key-containing backup download.

## 5. Configuration behavior

Website, policy, certificate-reference, exclusion, and proxy-setting writes update desired state only. They do not prove that Nginx is serving that state. The API must distinguish desired generation, validated revision, applied revision, and current activation job.

A configuration apply must render and test a complete candidate, serialize activation, atomically switch to it, request a graceful reload, and verify readiness. Report success only after the running revision is verified. On failure, restore and verify the previous known-good configuration. Never turn arbitrary browser input into shell arguments.

In split local development, the Go process cannot reload Nginx in another container. The API must report the activation executor as unavailable rather than simulate success. Exercise automated apply and rollback in the integrated one-container runtime.

## 6. Event and metric semantics

A rule match is not automatically a blocked request. Keep unique transactions, detected transactions, rule matches, confirmed WAF blocks, and separate transport/limit rejections distinct. Include source freshness and collector lag; show stale/unavailable measurements rather than fabricated zeroes.

Return only redacted event data. Do not retain request or response bodies by default. Keep event collection bounded and ensure ingestion failures do not block public traffic.

## 7. Implementation and verification checklist

Before calling this contract implemented, verify at least:

1. Route and schema inventory matches the checked-in source contract and handlers.
2. Anonymous access, role restrictions, CSRF, Origin checks, session expiry, and rate limits are tested.
3. Concurrent updates correctly enforce ETags, generation checks, and idempotency.
4. Protected website routes cannot reach the management listener.
5. Upstream destinations are checked against explicit allowlists and forbidden network ranges.
6. Invalid candidate configuration cannot replace the last known-good active revision.
7. Apply, rollback, interrupted activation, and recovery are tested in the final container image.
8. Event/metric counters are based on unique transactions and accurately report ingestion lag.
9. Errors and logs do not disclose credentials, keys, or unredacted request values.
10. Backup and restore are verified against a clean instance, including failure recovery.

The current API documents are design proposals, not evidence that these checks or endpoints have been implemented. Update status and endpoint details only after source, contract tests, and runtime verification provide evidence.
