# Product Roadmap

**Status:** Proposed sequencing; dates and staffing are not committed.  
**Planning basis:** Architecture and requirement drafts, not an implementation estimate.

For milestone deliverables, sequencing dependencies, and evidence-based exit gates, see [milestones](milestone.md).

## Principles

1. Prove the traffic/enforcement path before building a broad dashboard.
2. Default new sites to DetectionOnly and require deliberate tuning before Blocking.
3. Make activation transactional, observable, and recoverable before exposing broad configuration controls.
4. Add features only with a test plan and explicit data/privacy behavior.
5. Do not publish a release or security claim without final-image evidence.

## Phases

| Phase | Objective | Exit criteria |
| --- | --- | --- |
| 0. Foundation | Approve threat model, requirements, license approach, source layout, and test environment. | Decisions recorded; build and contribution conventions selected. |
| 1. WAF prototype | Package Nginx, inspection engine (ModSecurity v3 or OWASP Coraza), CRS v4, and a demo upstream. | Benign request succeeds; controlled detection logs in DetectionOnly; controlled test is blocked in Blocking; TLS and graceful reload work. |
| 2. Management core | Implement Go API, SQLite migrations, bootstrap/auth, typed site/policy models, and validation. | API tests cover persistence, roles, input bounds, CSRF, and upstream restrictions. |
| 3. Safe activation | Add immutable revisions, serialized jobs, candidate test, activation journal, readiness verification, rollback, and restart recovery. | Fault-injection tests prove failed apply preserves/restores last-known-good. |
| 4. Dashboard and events | Implement static Next.js dashboard, setup/login, site/policy workflows, redacted event search, accurate metrics, and health. | Browser workflows use real API responses; no production mock metrics; metrics show freshness. |
| 5. Operations | Automated ACME (Let's Encrypt / ZeroSSL) & manual certificates, retention, encrypted backup/restore, upgrade compatibility, non-root runtime, SBOM/notices. | Fresh install/restore, disk pressure, restart, and upgrade tests pass on release image. |
| 6. Release candidate | Security review, compatibility matrix, docs review, image signing where available, and release rehearsal. | All P0/P1 acceptance criteria met or explicitly deferred; published artifact digest and evidence. |

## Prioritization

- **P0:** traffic path & engine proof, initial setup, auth/roles, constrained upstreams, policy modes, safe apply/rollback, persistence, ring-buffered events, critical health.
- **P1:** accurate events/metrics rollups, ACME automated issuance & manual certificates, L7 rate limiting, backup/restore, retention, complete operations documentation.
- **Later candidates:** custom rule authoring, IP reputation feeds, scheduled PDF reports, multi-instance cluster sync, remote upgrades.


## Release gates

A phase does not exit because code compiles alone. Require tests matching its exit criteria, review of security impact, updated docs, and known limitations. Never use a planned item as a claim of existing capability.

## Unresolved planning items

No owner, delivery date, budget, public support window, target throughput, or supported host/CPU matrix has been approved. The project owner should resolve these before committing release dates.
