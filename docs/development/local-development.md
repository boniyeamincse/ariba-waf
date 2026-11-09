# Local Development

**Status:** Partial. The Go health-probe service and tests exist; dashboard, WAF proxy, database, and product API are not implemented.

## 1. Development topology

Run the dashboard, Go API, WAF proxy, and demo upstream separately during development. Production remains a single integrated image/container; the split topology is only a faster iteration loop.

| Component | Intended process | Example address/data |
| --- | --- | --- |
| Next.js dashboard | `npm run dev` | `http://localhost:3000` |
| Go health-probe skeleton | `go run ./cmd/ariba` | `http://127.0.0.1:9000/health/live` |
| Coraza HTTP demo proxy | `go run ./cmd/waf-demo -mode detectionOnly` | `http://127.0.0.1:18080` (loopback only) |
| Nginx + ModSecurity + CRS | Dedicated development container | `http://localhost:8080` |
| Demo upstream | Small local test service | `http://localhost:8081` |
| SQLite | Opened by Go API | `./dev-data/ariba.db` |

The Go health command and Coraza handler package exist. There is not yet a runnable WAF proxy command, frontend, Nginx container, or SQLite integration; those processes, ports, and the database path are planning examples until their source/configuration is added.

## 2. Run the current Go skeleton

From the repository root, run `go test ./...`, then start the server with `go run ./cmd/ariba`. It binds only to `127.0.0.1:9000`.

- `GET http://127.0.0.1:9000/health/live` returns `200` while the process is responsive.
- `GET http://127.0.0.1:9000/health/ready` currently returns `503 NOT_READY`; storage and WAF dependencies are not wired in yet.
- The probe server is not a management API and has no authentication or configuration endpoints. Do not expose it beyond loopback.
- Run `go test ./...` to exercise the local Coraza/CRS synthetic request cases; the handler package is a testable adapter, not an externally reachable proxy.
- Run `go run ./cmd/waf-demo -mode detectionOnly` for a loopback-only HTTP reverse-proxy demo. Use `-mode blocking` to verify the controlled local CRS test request is rejected. The demo has a fixed in-process upstream, no TLS, and no Nginx; it is not the release proxy or a production service.

## 3. Planned setup sequence

1. Install the selected Go, Node.js/package-manager, Docker, and test tools after versions are pinned in the implementation.
2. Keep source for frontend, backend, and deploy assets in one repository. Ignore `dev-data/`, secrets, generated keys, databases, logs, and backup artifacts.
3. Start a demo upstream and the WAF development container on a dedicated network. Configure the upstream using an address reachable from the container; container `localhost` refers to that container, not the host.
4. Start the Go API bound to loopback with an isolated development database. Use test-only bootstrap data and non-production credentials.
5. Start Next.js and proxy `/api/v1` to the loopback Go API. Allow only the exact development origin and development-only cookie behavior.
6. Exercise WAF routing and controlled test traffic before wiring up dashboard workflows.
7. Build the integrated image to test activation, because a host-run Go process cannot reload Nginx in a separate container without an unsafe Docker socket/control bridge.

## 4. Iteration order

- WAF path: benign request, CRS detection, Blocking response, upstream failure, TLS, and graceful reload.
- API foundation: health, migrations, setup/auth, typed site and policy endpoints.
- Safe activation: validation, durable job, revision, reload/readiness, rollback, restart recovery.
- Dashboard: setup/login, website/policy editing, apply status, events and health using real API responses.
- Operations: certificates, retention, backup/restore, failure injection, final-image smoke tests.

## 5. Local safety

Bind management and development services to loopback unless a reviewed test requires otherwise. Never use production keys or real customer traffic. Do not expose debug handlers, SQLite, or an activation endpoint to the LAN. Do not use a privileged container, host networking, or Docker socket as a shortcut.

## 6. Cleanup and verification

Use disposable test data. Verify local DB, generated config, certificate/key and logs remain ignored by Git. Test the final image with a fresh volume and with an existing persisted volume. Once build tooling exists, record exact verified commands and versions here instead of leaving illustrative commands as operational instructions.
