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
| 1. WAF prototype | Package Nginx, compatible ModSecurity connector, CRS, and a demo upstream. | Benign request succeeds; controlled detection logs in DetectionOnly; controlled test is blocked in Blocking; TLS and graceful reload work. |
| 2. Management core | Implement Go API, SQLite migrations, bootstrap/auth, typed site/policy models, and validation. | API tests cover persistence, roles, input bounds, CSRF, and upstream restrictions. |
| 3. Safe activation | Add immutable revisions, serialized jobs, candidate test, activation journal, readiness verification, rollback, and restart recovery. | Fault-injection tests prove failed apply preserves/restores last-known-good. |
| 4. Dashboard and events | Implement static Next.js dashboard, setup/login, site/policy workflows, redacted event search, accurate metrics, and health. | Browser workflows use real API responses; no production mock metrics; metrics show freshness. |
| 5. Operations | Certificates, retention, encrypted backup/restore, upgrade compatibility, non-root runtime, SBOM/notices. | Fresh install/restore, disk pressure, restart, and upgrade tests pass on release image. |
| 6. Release candidate | Security review, compatibility matrix, docs review, image signing where available, and release rehearsal. | All P0/P1 acceptance criteria met or explicitly deferred; published artifact digest and evidence. |

## Prioritization

- **P0:** traffic path, initial setup, auth/roles, constrained upstreams, policy modes, safe apply/rollback, persistence, critical health.
- **P1:** accurate events/metrics, certificates, backup/restore, retention, complete operations documentation.
- **Later candidates:** ACME automation, custom rule authoring, IP reputation, scheduled reports, multi-instance coordination, remote upgrades.

## Release gates

A phase does not exit because code compiles alone. Require tests matching its exit criteria, review of security impact, updated docs, and known limitations. Never use a planned item as a claim of existing capability.

## Unresolved planning items

No owner, delivery date, budget, public support window, target throughput, or supported host/CPU matrix has been approved. The project owner should resolve these before committing release dates.
