# Product Security Model

**Status:** Proposed controls; not a security certification or implemented guarantee.

## 1. Assets and trust boundaries

Assets include administrator credentials/sessions, enrollment tokens, TLS private keys, site/policy configuration, active revisions, audit/security events, backups, and upstream services. Trust boundaries exist between visitors and public listeners, admins and management listener, browser and API, API and Nginx activation, container and mounted `/data`, and WAF and upstream networks.

## 2. Authentication and authorization

Use one-time short-lived bootstrap enrollment, hash bootstrap tokens, disable enrollment after first Admin, and never ship a fixed password. Store passwords with a reviewed password-hashing implementation such as Argon2id. Use opaque server-side sessions, Secure/HttpOnly/SameSite cookies, expiration, rotation, revocation, login throttling, and generic credential errors.

Protect mutations with CSRF tokens and exact Origin checks. Enforce Admin/Operator/Viewer authorization in the API. Protect the last enabled Admin from accidental removal. Require reauthentication for high-impact account, backup, and restore actions.

## 3. Network and request safety

Management listener is separate from site listeners and loopback-bound by default on the host. Do not trust arbitrary forwarded client-IP headers. Restrict upstreams to administrator-approved CIDRs and block metadata, management, and unintended loopback targets at resolution/connection time. Preserve TLS verification for upstreams by default.

Bound body sizes, upload sizes, query ranges, timeouts, pagination, queues, event sizes, and disk use. Parse only supported content types. Explain when requests exceed inspection limits; never claim full inspection for partial or failed parsing.

## 4. Configuration execution

Generate Nginx/ModSecurity configuration from typed, validated inputs and controlled templates. Do not accept arbitrary shell commands or unrestricted snippets. Test candidate config, serialize activation, use immutable revisions/journaled activation, verify readiness, and recover last-known-good state before declaring success.

The Go service should not need root. Do not require privileged mode, host networking, or Docker socket access. Restrict file ownership and permissions for `/data`, especially private keys, session material, and backups.

## 5. Data protection

Do not persist request/response bodies by default. Redact headers, cookies, query values, and rule messages before storage/export. Never expose private keys, password hashes, enrollment tokens, session tokens, or backup passphrases via normal API responses/logs. Encrypt backups with vetted authenticated encryption; do not invent cryptography. Protect backup downloads and require recent authentication.

Set explicit retention, safe disk-pressure behavior, and consistent SQLite backup/restore. Delete expired temporary restore staging data. Rotate any exposed secret; deleting it from source control is not enough.

## 6. Threats requiring testing

- Unauthorized management access or protected-host routing to management handlers.
- CSRF, session fixation/theft, brute-force and role bypass.
- SSRF, DNS rebinding, metadata endpoint access, and trusted-proxy spoofing.
- Config injection, path traversal, malformed archives, unsafe upload, and certificate key leakage.
- Race conditions between edits/apply/rollback/restore and crash during activation.
- Log flooding, parser/resource exhaustion, disk exhaustion, and sensitive-data leakage.
- Dependency compromise and Nginx/connector/CRS incompatibility.

## 7. Security verification and release

Threat model and tests must be reviewed before release. Scan dependencies and final image, pin versions/digests, publish notices/SBOM, document supported versions and vulnerability reporting, and re-test the exact image digest. No security property should be claimed without evidence. See [repository vulnerability reporting](../../SECURITY.md) and [testing strategy](../testing/testing.md).
