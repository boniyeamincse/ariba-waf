# Ariba Shield WAF — Management API

**Status:** Accounts/sessions/roles, Admin user management, desired website/policy CRUD, proxy CIDR settings, configuration status, and extended auth endpoints (refresh, permissions, activity, revoke-all) are implemented with SQLite and tests. Activation and other operational APIs remain proposed.
**API version:** `v1`  
**Implementation target:** Go API, embedded SQLite, one-container release.

This page is the API entry point in the development documentation. The detailed draft contract, including example payloads, validation limits, role matrix, error codes, events, and backup/restore behavior, is maintained in [the API specification](../api.md). Update both documents when the API contract changes; once the documentation reorganization is complete, consolidate the detailed specification here and remove the temporary legacy location.

The host-run service binds to `127.0.0.1:9000` by default. The [local Docker service](../operations/management-container.md) explicitly opts into a container listener on `0.0.0.0:9000`, published only on host `127.0.0.1:19000`. Internal `GET /health/live` and `GET /health/ready` handlers remain separate from `/api/v1`; readiness returns `503 NOT_READY` because WAF activation is not connected. This is not a production-ready API.

## Implemented foundation

| Method | Route under `/api/v1` | Access |
| --- | --- | --- |
| GET | `/setup/status`, `/auth/csrf` | Public |
| POST | `/setup/enroll` | One-time local CLI token; requires `Idempotency-Key` |
| POST | `/auth/login` | Public; throttled |
| POST | `/auth/refresh` | Authenticated user; rotates session |
| GET | `/auth/me`, `/auth/sessions`, `/auth/permissions`, `/auth/activity` | Authenticated user; own identity/sessions/permissions/activity |
| POST | `/auth/logout`, `/auth/password`, `/auth/reauthenticate`, `/auth/sessions/revoke-all` | Authenticated user |
| DELETE | `/auth/sessions/{id}` | Session owner |
| GET / POST | `/users` | Admin |
| GET / PATCH / DELETE | `/users/{id}` | Admin; PATCH/DELETE require `If-Match` |
| GET | `/websites`, `/websites/{id}`, `/policies`, `/policies/{id}` | Any authenticated role |
| POST | `/websites`, `/policies` | Admin/Operator |
| PATCH / DELETE | `/websites/{id}`, `/policies/{id}` | Admin/Operator; require `If-Match` |
| GET / PATCH | `/settings/proxy` | Admin; PATCH requires `If-Match` |
| GET | `/configuration/status` | Any authenticated role |
| GET | `/events`, `/metrics/overview` | Any authenticated role |
| GET | `/audit-logs` | Admin |

## Implemented vs proposed matrix

Use this matrix to separate verified routes from planned scope.

| API scope | Current state | Source of truth |
| --- | --- | --- |
| Setup and core auth (`/setup/status`, `/auth/csrf`, `/setup/enroll`, `/auth/login`, `/auth/logout`, `/auth/me`) | Implemented and tested | Code routes + management tests |
| Extended auth (`/auth/refresh`, `/auth/permissions`, `/auth/activity`, `/auth/sessions`, `/auth/sessions/revoke-all`, `/auth/sessions/{id}`, `/auth/password`, `/auth/reauthenticate`) | Implemented and tested | Code routes + management tests |
| Users (`/users`, `/users/{id}`) | Implemented and tested | Code routes + management tests |
| Desired traffic config (`/websites*`, `/policies*`, `/settings/proxy`, `/configuration/status`) | Implemented and tested | Code routes + management tests |
| Activation workflow (`/configuration/revisions*`, `/configuration/apply`, `/configuration/rollback`, `/jobs/{id}`) | Proposed (not implemented) | Draft contract in `docs/api.md` |
| Event and metrics APIs (`/events`, `/metrics/overview`, `/audit-logs`) | Implemented baseline read endpoints | Code routes + management tests |
| Extended event/metrics APIs (`/events/{id}`, `/metrics/timeseries`) | Proposed (not implemented) | Draft contract in `docs/api.md` |
| Certificates and trust bundles (`/certificates*`, `/trust-bundles*`) | Proposed (not implemented) | Draft contract in `docs/api.md` |
| Backup and restore (`/backups*`, `/restore-*`) | Proposed (not implemented) | Draft contract in `docs/api.md` |
| Email recovery, email verification, 2FA auth endpoints | Proposed (not implemented) | Draft contract and milestone docs |

When code and docs diverge, treat registered routes in the backend as authoritative for implementation status, then update this page and the draft contract.

All unsafe requests require the configured exact `Origin` and `X-CSRF-Token`. Setup creates the first Admin without logging in. User creation supports `admin`, `operator`, and `viewer`; operators/viewers can use their own authentication/session routes but cannot manage users. Role changes, disable/delete, and password changes revoke affected sessions. `POST /auth/sessions/revoke-all` revokes every session for the authenticated account and clears the current cookie. The last enabled Admin cannot be removed. User lists use signed cursors, creation-time/ID ordering, a default limit of 50, and `meta.nextCursor`. Responses include request IDs and omit password/token hashes.

For local development:

1. Run `go run ./cmd/ariba -enrollment-token` and retain the printed token privately. It expires in 10 minutes; rerunning replaces the previous unused token.
2. Run `go run ./cmd/ariba -dev-http -origin http://localhost:3000`. Use the frontend prototype at `http://localhost:3000` or an HTTP client against `http://127.0.0.1:9000` while sending `Origin: http://localhost:3000` on mutations.
3. GET `/api/v1/auth/csrf`, retain the response cookie, and read `data.csrfToken`.
4. POST `/api/v1/setup/enroll` with JSON `enrollmentToken`, `username`, `displayName`, and a 12–128 character `password`; include `Idempotency-Key` and `X-CSRF-Token`. Retrying the same token/key/body within 24 hours returns the original result; changed content conflicts. Enrollment creates no session.
5. POST `/api/v1/auth/login` with `username` and `password`, retaining cookies. GET `/api/v1/auth/csrf` again for the new session-bound token. Refresh this token after reauthentication too.
6. GET `/api/v1/auth/me` to verify identity. Admins can create users and GET a user to obtain its ETag before PATCH/DELETE.

Sessions expire after 30 idle minutes or 12 hours and are limited to 10 live sessions per account. Reauthentication rotates the session and grants five minutes of recent-auth state. `POST /auth/refresh` rotates the session and renews the full session window for active users. Login/password verification allows five failed attempts per normalized account or direct peer IP in 15 minutes, with a global bound of 100 outstanding failures; forwarding headers do not change this identity. Two concurrent management requests are admitted; excess requests return `429`. Account audit records persist locally and are available as a bounded per-user history through `GET /auth/activity`; retention automation is not implemented yet. Database, token, and cookie files must remain private.

Email-token recovery (`/auth/forgot-password`, `/auth/reset-password`), email verification endpoints, and 2FA endpoints are not implemented in the current backend milestone.

## Save a desired website configuration

After logging in and obtaining a session-bound CSRF token:

1. GET `/settings/proxy` and use its ETag in `If-Match` on PATCH `/settings/proxy` with `{"allowedCidrs":["10.24.20.0/24"]}`. The initial allowlist is empty; use only networks approved for your actual upstreams. `trustedProxyCidrs` is also a bounded CIDR array, stored as desired state; it does not enable trusted forwarding headers on the running account API.
2. POST `/policies` with `{"name":"Default Detection"}`. Defaults are DetectionOnly, CRS paranoia 1, anomaly thresholds 5/4, request body limits 10 MiB/1 MiB, reject oversize/parser errors, response inspection disabled, and rate limiting disabled (configured rate 10/sec, burst 20). These are saved settings, not active enforcement.
3. Use the returned policy ID in POST `/websites`:

```json
{
  "name": "ERP Demo",
  "domains": ["erp.example.test"],
  "policyId": "<replace-with-returned-policy-id>",
  "upstreams": [{"scheme": "http", "host": "10.24.20.50", "port": 8080}]
}
```

New sites default to enabled with an HTTP listener, verified upstream TLS using system trust, and connect/read timeouts of 5/60 seconds. Creation requires a DetectionOnly policy. Later desired changes may select Off or Blocking. PATCH merges nested objects and preserves omitted fields; arrays replace the previous array. Unknown fields and null scalar values are rejected. Certificate IDs may be null; non-null certificate/trust-bundle references and HTTPS site listeners are rejected until certificate management exists. HTTPS upstreams are supported as desired definitions with TLS verification enabled and system trust. Response-body inspection is rejected as unsupported.

Writes increment resource versions and the global `desiredGeneration` with their audit entry in one transaction. Failed writes do not advance it. Website/policy reads include `desiredGeneration`, `appliedRevisionId`, and `pendingChanges`. GET `/configuration/status` also returns `currentJob: null` and `activationExecutorAvailable: false`. No configuration is applied, the Coraza demo remains independent, and readiness stays unavailable. Policy deletion rejects references, including disabled websites. Enabled domains are unique after IDNA/case/trailing-dot normalization; disabled sites may share domains but must pass the conflict check when enabled.

Upstream save-time checks permit only HTTP/HTTPS, bounded ports/timeouts, and approved CIDRs. All DNS answers must pass; mixed allowed/forbidden answers and unresolved names are rejected. Resolution has a shared three-second deadline per write and a maximum of 64 addresses per hostname. Settings allow at most 64 CIDRs per array. Loopback, link-local, multicast, selected metadata/service addresses, and IPv6 translation/tunnel prefixes remain forbidden even with a broad allowlist. Ports 9000/9443 are conservatively reserved for management. Proxy settings changes are rejected if they invalidate any existing desired upstream. No upstream connection is opened by these endpoints.

Save-time validation is only one stage. The future activation/proxy integration must repeat destination checks and connect to a validated address on each connection, retry, and re-resolution; see [OWASP's SSRF prevention guidance](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html#domain-name). These endpoints do not establish runtime DNS-rebinding protection or live routing.

## 1. Purpose and boundaries

The management API configures Ariba Shield WAF and serves its dashboard. It is distinct from the protected websites' APIs. Public website listeners must never forward requests to the management API; `/api` on a protected virtual host belongs to that upstream application.

The API is served to the browser on the management HTTPS origin under `/api/v1`. In local development, the Next.js dev server proxies `/api/v1` to the Go API on loopback. The internal Go listener is not published as a host port in the release container.

| Environment | Browser-facing API | Internal API listener |
| --- | --- | --- |
| Local development | `http://localhost:3000/api/v1` | `http://127.0.0.1:9000/api/v1` |
| Packaged container | `https://localhost:9443/api/v1` | `http://127.0.0.1:9000/api/v1`, loopback only |

## 2. API groups

The table below describes the full target scope; only routes listed in the implemented foundation above exist. The detailed draft in [the API specification](../api.md) defines the proposed methods and payloads.

| Group | Responsibilities |
| --- | --- |
| Setup and authentication | Bootstrap status, one-time enrollment, login/logout, sessions, password changes, CSRF token |
| Users | Admin-managed accounts, roles, enable/disable, session revocation, admin-assisted password reset |
| Websites | Domains, listeners, upstreams, upstream dry-run check, TLS policy, certificates, assigned protection policy |
| Policies, exclusions and IP rules | Detection mode, CRS thresholds, inspection limits, rate limits, site-scoped exceptions, static CIDR allow/deny rules |
| Certificates and trust bundles | Upload and validate site certificates/keys, expiry filtering, and upstream trust material |
| Configuration | Desired generation, config diff preview, immutable revisions, validation, apply, rollback, durable jobs |
| Events and metrics | Redacted security events, bounded exports, request/block/detection metrics, ingestion freshness |
| System settings | Health, version, trusted proxies, destination allowlists, retention, notification sinks |
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

The foundation tests verify account and session behavior, not this entire checklist. Remaining API sections describe design proposals. Update status and endpoint details only after source, contract tests, and runtime verification provide evidence.
