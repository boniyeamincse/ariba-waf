# Ariba Shield WAF — Milestones

**Status:** Proposed execution plan; dates, owners, estimates, and funding are not approved.  
**Planning basis:** Requirements and architecture drafts only. No milestone is complete until evidence is recorded.

## Milestone summary

| ID | Milestone | Main outcome | Exit gate |
| --- | --- | --- | --- |
| M0 | Foundation and decisions | Approved scope, threat model, license decision, repository conventions, and test environment. | High-impact decisions have owners and written outcomes. |
| M1 | WAF traffic prototype | One integrated image routes a demo site through Nginx, inspection engine (ModSecurity v3 or OWASP Coraza), and CRS v4. | Benign traffic passes; a controlled rule match is visible in DetectionOnly and blocked in Blocking; TLS/reload behavior is tested. |
| M2 | Management API foundation | Go API, SQLite migrations, bootstrap, sessions, roles, and typed site/policy management. | API tests cover persistence, validation, permissions, CSRF, and upstream restrictions. |
| M3 | Safe configuration activation | Immutable revisions, validation, serialized jobs, activation journal, readiness, rollback, and restart recovery. | Fault-injection tests prove failed apply does not leave an unverified candidate active. |
| M4 | Dashboard and event visibility | Static dashboard with real API data, events, metrics, and health/freshness states. | Browser workflows work end-to-end; no mock metrics appear in production. |
| M5 | Operational readiness | Automated ACME & manual certificates, retention, encrypted backup/restore, non-root runtime, upgrade process, notices, and SBOM. | Fresh install/restore, disk-pressure, restart, and upgrade tests pass on the final image. |
| M6 | Release candidate | Security review, compatibility review, release notes, artifact integrity/signing where supported, and support process. | P0/P1 acceptance criteria are met or explicitly deferred; exact image digest and evidence are recorded. |

## Milestone details

### M0 — Foundation and decisions

**Deliverables**

- Review and approve the [product requirements](requirements.md) and [roadmap](roadmap.md).
- Decide initial threat model, supported host architectures, base image, supervisor, dependency pinning, and test environment.
- Approve or replace the proposed license and define contribution/security-reporting ownership.
- Establish repository layout, build conventions, CI checks, and sanitized fixtures.

**Exit evidence**

- Decisions and unresolved risks have owners.
- No release date or performance target is presented as committed without an estimate.
- Security reporting and license status are clear to contributors.

### M1 — WAF traffic prototype

**Deliverables**

- Evaluate and benchmark the inspection engine: Nginx + compatible ModSecurity connector vs OWASP Coraza (Cgo/Wasm or Go proxy) with pinned CRS v4.
- Route a controlled demo upstream through HTTP/HTTPS listeners.
- Produce safe synthetic requests and demonstrate DetectionOnly and Blocking semantics.
- Verify upstream TLS, graceful reload, startup, and container signal handling.

**Exit evidence**

- Final-image integration test results and image digest are recorded.
- Normal requests pass; controlled detection is logged without blocking in DetectionOnly; Blocking rejects the controlled match.
- Module/rule load failures and upstream failures are visible and do not silently bypass inspection.
- Memory leak and stability benchmarks under 5,000 req/sec recorded.


### M2 — Management API foundation

**Deliverables**

- Versioned Go API, SQLite schema migrations, setup enrollment, authentication, sessions, roles, and audit events.
- Typed website, upstream, policy, certificate metadata, and exclusion models.
- Bounded input validation, destination allowlists, CSRF/Origin controls, and API contract tests.

**Exit evidence**

- No default credentials; bootstrap token is single-use and enrollment closes after setup.
- Role, concurrency, error, and persistence tests pass.
- API writes are clearly represented as desired state, not applied state.

### M3 — Safe configuration activation

**Deliverables**

- Desired-generation tracking and immutable revision snapshots.
- Render/test pipeline, durable jobs, serialized activation, audit journal, readiness verification, rollback, and restart reconciliation.
- Safe local development behavior when the Go API cannot control a separate Nginx container.

**Exit evidence**

- Invalid candidate, reload failure, restart at each activation stage, and rollback tests preserve or restore the last-known-good state.
- No shell interpolation, general command execution, or Docker socket requirement.
- API/UI only report applied after verification.

### M4 — Dashboard and event visibility

**Deliverables**

- Next.js static export and initial setup/login, website/policy workflows, configuration status, events, and overview.
- Bounded event collector with rotation handling, deduplication, redaction, retention, and freshness indicators.
- Accessibility, keyboard, error, empty, loading, permission, and partial-data states.

**Exit evidence**

- Browser end-to-end tests exercise real API responses.
- Counters distinguish unique requests, detections, rule matches, confirmed blocks, and separate rejections.
- Collector lag/unavailability is not rendered as zero.

### M5 — Operational readiness

**Deliverables**

- Automated ACME (Let's Encrypt / ZeroSSL) HTTP-01 challenge handler and auto-renewal background job.
- Site certificate manual upload/validation, CA trust bundles, retention, protected backup and restore, non-root volume permissions, and documented upgrade compatibility.
- Health/readiness behavior, disk-pressure handling, logs/notices, SBOM, and operator runbooks.

**Exit evidence**

- ACME staging/production issuance and automated renewal succeed without traffic interruption.
- Restore to a clean volume succeeds; failed restore recovery is tested.
- Container replacement preserves expected state; upgrade/rollback behavior is documented and tested.
- Final image passes module ABI, routing, enforcement, and lifecycle suites.


### M6 — Release candidate

**Deliverables**

- Security and dependency review, compatibility matrix, versioned release notes, vulnerability reporting process, and reproducible release evidence.
- Versioned immutable image artifact; signature/provenance where the chosen registry/build pipeline supports it.

**Exit evidence**

- Release owner approves known limitations and deferred requirements.
- Exact source revision, image digest, SBOM, notices, test results, and rollback instructions are published.
- No unresolved critical/high-risk issue is represented as accepted without explicit review and documented mitigation.

## Dependencies and sequencing

M0 precedes implementation. M1 validates the fundamental product choice. M2 can begin in parallel after the API/data model is reviewed, but M3 depends on the real engine behavior from M1 and the typed state from M2. M4 depends on stable API contracts and configuration job semantics. M5 depends on persistent data and final-image integration. M6 depends on all release gates.

Frontend visual exploration may happen earlier, but it does not replace traffic-path or API/security validation.

## Tracking template

For each milestone, record:

- Owner and reviewers: **TBD**
- Start/target dates: **TBD; not committed**
- Status: **Not started** until work begins
- Evidence links: tests, source revision, image digest, review record
- Risks/dependencies and approved exceptions

Do not mark a milestone complete based only on code presence, a mock, a design document, or a successful build.
