# Testing Strategy

**Status:** Partial implementation tests exist; release/CI verification remains incomplete.

Run `go test ./...` and `go vet ./...` from the repository root. Existing tests cover health probes, synthetic Coraza/CRS traffic, the local HTTPS demo, and the SQLite management foundation. Management tests exercise enrollment races/retries/expiry, persistence, login/logout, Origin/CSRF checks, input limits, roles, ETags, last-Admin protection, session expiry/revocation, and password changes. Use `go test -race ./...` for concurrency checks. These checks do not establish release-image or production readiness.

Desired-configuration tests cover migration from schema v1, site/policy CRUD and persistence, nested PATCH behavior, IDNA domain conflicts, referenced-policy deletion, generation/audit atomicity, signed pagination, role restrictions, concurrent domain writes, and destination validation with an injected resolver. No real DNS or upstream service is required for these tests. Runtime destination pinning, certificate management, and activation remain unverified.

The local management image has a separate packaging check: build `Dockerfile.api` as `ariba-shield-management-api:local` (add `--network=host` when the Docker build DNS cannot resolve `proxy.golang.org`), then run `python3 tests/integration/management_container.py`. It creates an isolated Compose project/volume and verifies health, runtime restrictions, bootstrap/login and persistence through container replacement. This is independent of the integrated release-image test plan below; see [the container guide](../operations/management-container.md).

## 1. Test layers

| Layer | Scope | Required evidence |
| --- | --- | --- |
| Unit | Typed validation, policy semantics, redaction, metric aggregation, state transitions | Deterministic tests with boundary and invalid cases |
| API/contract | Routes, schemas, auth/roles, pagination, errors, idempotency, ETags | Contract tests against real Go handlers and SQLite |
| Integration | Go + SQLite + Nginx + connector + CRS + demo upstream | Disposable integrated environment with real proxy traffic |
| Browser | Setup/login, roles, site/policy forms, apply status, events | End-to-end tests against real API behavior; no production mock data |
| Security | CSRF, Origin, SSRF, trust boundaries, upload/archive, secret redaction | Negative tests, threat scenarios, reviewed safe fixtures |
| Operations | Restart, disk pressure, log rotation, backup/restore, upgrade | Fault injection and fresh-volume recovery evidence |
| Release image | Runtime identity, module compatibility, routing, lifecycle | Tests against the exact image digest intended for publication |

## 2. Critical functional scenarios

- Healthy benign request reaches the correct upstream.
- Controlled CRS match is recorded but not blocked in DetectionOnly.
- Controlled match is blocked in Blocking; ordinary request remains allowed.
- `off`, body limits, parser failures, multipart, and partial-inspection semantics match the documented API/UI.
- Site management changes desired state only until a revision is successfully applied.
- Invalid candidate, reload failure, readiness mismatch, or crash preserves/restores last-known-good configuration.
- Events deduplicate across partial reads and rotation; counters separate transactions, matches, blocks, and non-WAF rejections.
- Certificate mismatch/expiry/SAN, upstream TLS failure, forbidden destinations, and untrusted forwarded headers fail safely.
- Backup restores to a clean volume; failed restore recovers checkpoint and revokes sessions as designed.

Use controlled test cases in an isolated environment. Never test attacks against systems without explicit authorization.

## 3. Security and resilience scenarios

Test anonymous access, each role, CSRF/Origin mismatch, rate limits, session expiration/revocation, concurrent stale updates, idempotency replay, DNS re-resolution, metadata/loopback denial, oversized payloads, queue saturation, log redaction, upload limits, archive path traversal, disk pressure, and restart at every activation stage.

## 4. Test data

Fixtures must be synthetic and sanitized. Exclude real customer requests, cookies, credentials, private keys, or production hostnames. Use reserved example domains/IPs. Protect generated local test keys and databases from Git.

## 5. Release gates

A release candidate must pass the agreed unit, contract, integration, security, operations, and final-image suites. Report exact commands, tool versions, image digest, environment, failures, and skipped tests. A test skipped due to missing infrastructure is **unverified**, not passed. Track unresolved failures and documented exceptions with an owner and rationale.

## 6. Pending test infrastructure

Go tests currently use the standard library. Choose frontend test frameworks, container orchestration for CI, supported architectures, static/security scanners, and performance environment. Do not write “all tests pass” before test commands and evidence exist.
