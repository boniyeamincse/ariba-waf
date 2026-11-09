# Ariba Shield WAF

> **Status:** Early development. Go health probes and a Coraza/CRS v4 handler prototype with tests exist; the integrated proxy, management features, and release image are not implemented or available yet.

Ariba Shield WAF is planned as a browser-managed, application-layer Web Application Firewall. Its release target is one Docker image, one running container, and one persistent data volume. Nginx, OWASP Coraza (or ModSecurity v3), OWASP CRS v4, a Go management API, a static Next.js dashboard, and SQLite (WAL mode) are proposed components. Protected applications remain external upstreams.


## Project status

- **Current phase:** M1 WAF prototype; the Coraza adapter is exploratory and the production engine/integration decision remains open.
- **Release image:** not published.
- **Installation commands:** target examples only; registry and version are not selected.
- **Security and performance claims:** not yet validated by an implementation.

Do not deploy this project as a production security boundary until implementation, testing, threat review, and release verification are complete.

## Intended workflow

`Pull image → run container → open management UI → complete bootstrap → add site → test in DetectionOnly → tune → enable Blocking`

The first release aims to support site and policy management, TLS certificates, event visibility, safe configuration activation and rollback, and backup/restore. See [requirements](docs/planning/requirements.md) and [roadmap](docs/planning/roadmap.md) for proposed scope.

## Documentation

Start at the [documentation index](docs/README.md). Key documents:

- [Architecture overview](docs/architecture/architecture.md)
- [Local development](docs/development/local-development.md)
- [Management API](docs/development/api.md)
- [Security model](docs/security/security.md)
- [Testing strategy](docs/testing/testing.md)
- [Deployment plan](docs/operations/deployment.md)

The earlier detailed drafts remain available at [architecture specification](docs/ariba-shield-waf-architecture.md) and [API specification](docs/api.md). Their contents are proposals, not evidence of implemented behavior.

## Planned stack

| Concern | Proposed choice |
| --- | --- |
| Public proxy | Nginx |
| Request inspection | OWASP Coraza (Go, memory-safe; recommended) or ModSecurity v3 with a pinned Nginx connector |
| Baseline rules | Pinned OWASP Core Rule Set (CRS v4) |
| Management API | Go |
| Dashboard | Next.js static export; no Node.js runtime in the release image |
| Persistence | SQLite (WAL mode) and files on a mounted `/data` volume |
| Process supervision | Small PID 1 supervisor (or Go supervisor entrypoint) |


## Contributing and security

Contributions and vulnerability reports are welcome after the repository's contribution and security processes are established. Read [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md). Do not include real credentials, private keys, or sensitive customer traffic in issues or pull requests.

## License

License selection is pending. Apache-2.0 is a proposal only; see [LICENSE](LICENSE). No license grant should be inferred until the project owner approves and replaces that notice with the complete chosen license.
