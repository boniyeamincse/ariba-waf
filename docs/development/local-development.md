# Local Development

**Status:** Proposed workflow. Repository build files and runnable services are not implemented yet.

## 1. Development topology

Run the dashboard, Go API, WAF proxy, and demo upstream separately during development. Production remains a single integrated image/container; the split topology is only a faster iteration loop.

| Component | Intended process | Example address/data |
| --- | --- | --- |
| Next.js dashboard | `npm run dev` | `http://localhost:3000` |
| Go API | `go run ./cmd/ariba` | `http://127.0.0.1:9000` |
| Nginx + ModSecurity + CRS | Dedicated development container | `http://localhost:8080` |
| Demo upstream | Small local test service | `http://localhost:8081` |
| SQLite | Opened by Go API | `./dev-data/ariba.db` |

These commands and ports are illustrative until the project has the corresponding source, scripts, and container configuration.

## 2. Setup sequence

1. Install the selected Go, Node.js/package-manager, Docker, and test tools after versions are pinned in the implementation.
2. Keep source for frontend, backend, and deploy assets in one repository. Ignore `dev-data/`, secrets, generated keys, databases, logs, and backup artifacts.
3. Start a demo upstream and the WAF development container on a dedicated network. Configure the upstream using an address reachable from the container; container `localhost` refers to that container, not the host.
4. Start the Go API bound to loopback with an isolated development database. Use test-only bootstrap data and non-production credentials.
5. Start Next.js and proxy `/api/v1` to the loopback Go API. Allow only the exact development origin and development-only cookie behavior.
6. Exercise WAF routing and controlled test traffic before wiring up dashboard workflows.
7. Build the integrated image to test activation, because a host-run Go process cannot reload Nginx in a separate container without an unsafe Docker socket/control bridge.

## 3. Iteration order

- WAF path: benign request, CRS detection, Blocking response, upstream failure, TLS, and graceful reload.
- API foundation: health, migrations, setup/auth, typed site and policy endpoints.
- Safe activation: validation, durable job, revision, reload/readiness, rollback, restart recovery.
- Dashboard: setup/login, website/policy editing, apply status, events and health using real API responses.
- Operations: certificates, retention, backup/restore, failure injection, final-image smoke tests.

## 4. Local safety

Bind management and development services to loopback unless a reviewed test requires otherwise. Never use production keys or real customer traffic. Do not expose debug handlers, SQLite, or an activation endpoint to the LAN. Do not use a privileged container, host networking, or Docker socket as a shortcut.

## 5. Cleanup and verification

Use disposable test data. Verify local DB, generated config, certificate/key and logs remain ignored by Git. Test the final image with a fresh volume and with an existing persisted volume. Once build tooling exists, record exact verified commands and versions here instead of leaving illustrative commands as operational instructions.
