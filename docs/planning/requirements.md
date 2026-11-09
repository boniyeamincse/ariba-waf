# Product Requirements

**Status:** Proposed; not approved or implemented.  
**Product:** Ariba Shield WAF.  
**Audience:** Product owner, engineering, security reviewers, and testers.

## 1. Problem and goal

Small teams need an inspectable reverse-proxy WAF that can be installed and managed without assembling a separate database, frontend, and backend service. Ariba Shield WAF proposes a single-container product with a browser interface for protecting HTTP/HTTPS sites.

The system must let an administrator register an upstream, observe detections safely, tune policy, and enable enforcement without losing the last known-good configuration.

## 2. Scope

### In scope for the first release proposal

- One image, one running container, and one persistent `/data` volume.
- Nginx with the inspection engine selected by the M1 prototype (ModSecurity v3 or Coraza) and a pinned OWASP CRS v4 release.
- Go management API, static Next.js dashboard, and embedded SQLite.
- One-time bootstrap, account/session management, Admin/Operator/Viewer permissions.
- Website/upstream definitions, certificates, structured policies and site-scoped exclusions.
- DetectionOnly and Blocking modes, bounded inspection, event visibility and honest health/freshness indicators.
- Validated configuration revisions, serialized activation, reload verification, rollback, and recovery after restart.
- Local backup and restore, documentation, test evidence, and third-party notices.

### Out of scope for the first release

- Operating-system firewall, VPN, general-purpose router, antivirus, or volumetric DDoS mitigation.
- High availability, shared multi-replica SQLite state, automatic clustering, or guaranteed throughput.
- Public SaaS control plane, mandatory paid service, or external account dependency.
- Arbitrary shell execution, Docker socket access, arbitrary Nginx snippets, and unreviewed raw rule editing.
- Automatic ACME issuance, GeoIP/reputation subscriptions, scheduled reports, and remote upgrades unless separately approved.

## 3. User roles and primary journey

- **Admin:** Owns initial setup, users, security-critical settings, certificates, backups and restore.
- **Operator:** Manages sites and policy and can validate/apply/rollback configuration.
- **Viewer:** Reads status, configuration, metrics, and redacted events.

Journey: install the future image → open management UI → consume a one-time bootstrap token → add an upstream in DetectionOnly → inspect representative traffic and events → tune structured policy → validate/apply → test known-safe and controlled detection cases → enable Blocking → monitor health and retain a tested backup.

## 4. Functional requirements

| ID | Requirement | Priority | Acceptance evidence |
| --- | --- | --- | --- |
| FR-01 | Route HTTP/HTTPS site traffic through Nginx and the inspection engine to an approved upstream. | P0 | Final-image integration test for healthy, denied, and upstream-failure cases. |
| FR-02 | Keep management routes separate from protected website routes. | P0 | Routing tests prove protected `/api` cannot reach management handlers. |
| FR-03 | Bootstrap exactly one initial Admin using a short-lived, single-use token; no default password. | P0 | Concurrent enrollment, expiry, retry, and post-setup rejection tests. |
| FR-04 | Enforce role checks, safe sessions, CSRF, Origin validation, login limits, and audit changes in the API. | P0 | API security tests for each role and anonymous/forged requests. |
| FR-05 | Store sites, policies, revision metadata, users, events, and jobs in persistent local storage. | P0 | Container replacement test using the same volume. |
| FR-06 | Support DetectionOnly and Blocking semantics with visible limits and accurate event attribution. | P0 | Controlled benign/detection/blocking and inspection-limit tests. |
| FR-07 | Validate complete candidate configuration before activation and preserve last-known-good state on failure. | P0 | Invalid candidate, reload failure, crash/restart, and rollback tests. |
| FR-08 | Restrict upstream destinations to approved networks and reject metadata/management/unintended loopback targets. | P0 | DNS and address-range policy tests, including re-resolution. |
| FR-09 | Provide redacted event queries and distinct detected, matched, blocked, and rejected counters. | P1 | Fixture-based deduplication and metric reconciliation tests. |
| FR-10 | Support certificate upload/validation and safe assignment to HTTPS sites. | P1 | Key mismatch, SAN mismatch, expiration, and upstream TLS tests. |
| FR-11 | Create encrypted consistent backups and validate/restore them into a clean instance. | P1 | Fresh-volume restore and injected-failure recovery tests. |
| FR-12 | Automated certificate issuance and renewal via ACME (Let's Encrypt / ZeroSSL) for HTTP-01 challenge. | P1 | Staging/production ACME flow, domain validation, and graceful reload tests. |
| FR-13 | Layer 7 adaptive rate-limiting and burst controls per site, IP, and URI prefix. | P1 | Rate limiter tests verifying burst thresholds, 429 responses, and exemption lists. |
| FR-14 | High-throughput asynchronous event ingestion with in-memory ring buffer to prevent SQLite contention. | P0 | Flood injection test (5,000 req/sec) proving management API responsiveness under attack. |
| FR-15 | Multi-instance configuration export/sync for clustered horizontal scale-out behind load balancers. | P2 | Export/import bundle validation and multi-node traffic consistency tests. |

Priorities and acceptance criteria are proposed and require product/security review before they become release commitments.

## 5. Non-functional requirements

- **Security:** least privilege, constrained configuration executor, secret redaction, bounded parsing/storage, trusted-proxy configuration, safe upstream policy.
- **Reliability:** durable activation jobs and recovery journal; logging failures do not block public traffic; failed configuration never replaces working configuration.
- **Operability:** separate liveness/readiness, visible stale data and collector lag, backups and documented recovery.
- **Compatibility:** exact Nginx/module ABI and CRS versions tested in the final image; pluggable support for OWASP Coraza engine.
- **Privacy:** no request/response body retention by default; redact sensitive headers/parameters; bounded retention.
- **Portability:** local operation without mandatory paid service; local SQLite on a supported filesystem; no shared multi-replica volume.

Numeric throughput, latency, availability, and supported-host targets are **TBD** until measured on an implemented release.

## 6. Open decisions

- Final supervisor, base image, package strategy, supported architectures, and non-root volume ownership approach.
- Engine selection confirmation: Nginx + ModSecurity v3 vs OWASP Coraza.
- Exact CRS version (v4 recommended) and supported configuration surface.
- License approval and dependency compatibility.
- Threat model, supported TLS policy, and management exposure defaults.
- Performance targets and supported traffic/event volume.
- Release support policy and vulnerability response contact.

