# Ariba Shield WAF

> **Status:** Proposed open-source project. The current repository contains architecture and API design documents only; no release image or working product implementation is available yet.

Ariba Shield WAF is planned as a browser-managed, application-layer Web Application Firewall. Its release target is one Docker image, one running container, and one persistent data volume. Nginx, ModSecurity v3, OWASP CRS, a Go management API, a static Next.js dashboard, and SQLite are proposed components. Protected applications remain external upstreams.

## Project status

- **Current phase:** design and documentation.
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
| Request inspection | ModSecurity v3 with a compatible Nginx connector |
| Baseline rules | Pinned OWASP Core Rule Set (CRS) |
| Management API | Go |
| Dashboard | Next.js static export; no Node.js runtime in the release image |
| Persistence | SQLite and files on a mounted `/data` volume |
| Process supervision | Small PID 1 supervisor, to be selected and tested |

## Contributing and security

Contributions and vulnerability reports are welcome after the repository's contribution and security processes are established. Read [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md). Do not include real credentials, private keys, or sensitive customer traffic in issues or pull requests.

## License

License selection is pending. Apache-2.0 is a proposal only; see [LICENSE](LICENSE). No license grant should be inferred until the project owner approves and replaces that notice with the complete chosen license.
