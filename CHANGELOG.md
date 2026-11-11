# Changelog

All notable changes to Ariba Shield WAF will be recorded here. The project has no released software versions yet; unreleased entries describe repository work, not a supported product release.

## Unreleased

### Added

- Initial product architecture and proposed management API specifications.
- Documentation structure for requirements, roadmap, architecture, development, security, testing, and operations.
- Loopback Go health probes and an isolated Coraza/CRS HTTP/HTTPS demo with a local container build.
- SQLite-backed management API foundation: one-time enrollment CLI, setup, Argon2id passwords, hashed server-side sessions, login/logout, reauthentication, password changes, session revocation, roles, and Admin user management.
- Origin/CSRF checks, bounded login attempts, ETag concurrency checks, last-enabled-Admin protection, account audit records, and management API tests.
- Desired website and protection-policy CRUD, Admin proxy CIDR settings, IDNA domain normalization/conflict checks, save-time upstream address validation, signed pagination, and transactional desired-generation/audit updates.
- SQLite schema version 2 migration preserving existing accounts and sessions. Configuration status reports pending desired changes without claiming live activation.
- Separate local management API image and Compose service with a non-root runtime, persistent SQLite volume, host-loopback port, liveness healthcheck and container-recreation integration check.
- Root API status page with health/setup links and bootstrap instructions, replacing the unhelpful root-path 404.
- Next.js dashboard prototype under `frontend/` for setup/login, account/session management, website/policy/user/proxy-setting editing, and read-only dashboard/event views.
- Unsigned development preview images published to Docker Hub as `boniyeamin/ariba-shield-waf-engine:latest` and `boniyeamin/ariba-shield-control-api:latest`; these are not supported releases.
- Documentation updates: current-vs-planned project structure, Docker Hub preview image disclaimers, and build host-networking notes for offline Docker DNS.

### Not implemented

- Integrated production WAF image, production dashboard, configuration activation, certificates/trust bundles, scoped exclusions, event ingestion/persistence, or installer.
- Production-ready setup, certificate automation, backup/restore, or upgrade process.

When software releases begin, follow Keep a Changelog conventions and use semantic version tags only after release artifacts and verification evidence are available.
