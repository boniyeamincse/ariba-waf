# Local Development

**Status:** Partial. SQLite-backed account and desired website/policy APIs, health probes, the Coraza demo proxy, and an initial Next.js dashboard prototype exist; activation and integrated release runtime remain unimplemented.

## 1. Development topology

Run the dashboard, Go API, WAF proxy, and demo upstream separately during development. Production remains a single integrated image/container; the split topology is only a faster iteration loop.

| Component | Intended process | Example address/data |
| --- | --- | --- |
| Next.js dashboard | `cd frontend && npm run dev` | `http://localhost:3000` |
| Go management foundation | `go run ./cmd/ariba -dev-http -origin http://localhost:3000` | `http://127.0.0.1:9000/api/v1` |
| Coraza HTTP/HTTPS demo proxy | `go run ./cmd/waf-demo -mode detectionOnly` | `http://127.0.0.1:18080`, `https://127.0.0.1:18443` (loopback only) |
| Nginx + ModSecurity + CRS | Dedicated development container | `http://localhost:8080` |
| Demo upstream | Small local test service | `http://localhost:8081` |
| SQLite management database | Opened by Go API | `./dev-data/ariba.db` |

The Go management command, Coraza demo, and frontend prototype are runnable. The Nginx development container and standalone demo upstream are still planning examples. The Coraza demo uses its own fixed in-process upstream; it does not consume management configuration.

For the management API in Docker, see [the local container guide](../operations/management-container.md). Build with `docker build --network=host --pull -f Dockerfile.api -t ariba-shield-management-api:local .`, then run `docker compose up -d --no-build --wait management-api`. Build host networking is used only for module downloads when Docker's default DNS cannot resolve `proxy.golang.org`; omit it where build DNS works. The default endpoint is `http://127.0.0.1:19000` with a persistent SQLite volume; it is separate from the WAF demo and is not the final release container. An unsigned development preview is also published as `boniyeamin/ariba-shield-control-api:latest`.

## 2. Run the current Go services

To run the frontend prototype:

1. `cd frontend`
2. `npm install`
3. `npm run dev`
4. Open `http://localhost:3000`, then use `/setup` and `/dashboard`.

Use the Go version declared in `go.mod`. From the repository root, run `go test ./...`, generate a setup token with `go run ./cmd/ariba -enrollment-token`, then start the server with `go run ./cmd/ariba -dev-http -origin http://localhost:3000`. It binds only to `127.0.0.1:9000`. The database defaults to `./dev-data/ariba.db`; override it with `-db <private-directory>/ariba.db` consistently for token creation and the server. Token creation exits without starting a listener and is refused after setup. A newly generated token replaces the previous unused token and expires in 10 minutes.

- `GET http://127.0.0.1:9000/health/live` returns `200` while the process is responsive.
- `GET http://127.0.0.1:9000/health/ready` returns `503 NOT_READY` until WAF activation is connected, even when the account database is healthy.
- The management API implements accounts/sessions/roles, desired website/policy CRUD, Admin proxy CIDR settings, configuration status, and baseline read routes for events, metrics, and audit logs. Event ingestion is pending, so `/events` returns an empty list and `/metrics/overview` reports unavailable freshness. Activation is not implemented. Site listeners currently support HTTP only; certificates/trust bundles are pending. See [management API](api.md) for routes, setup, and a desired-site example.
- Development mode uses a separate insecure cookie name and requires a loopback browser origin. Without `-dev-http`, the API requires an HTTPS origin and issues Secure `__Host-` cookies for the future management TLS gateway. Do not expose the internal API beyond loopback.
- Run `go test ./...` to exercise the management foundation and local Coraza/CRS synthetic request cases.
- Run `go run ./cmd/waf-demo -mode detectionOnly` for loopback-only HTTP and HTTPS reverse-proxy demos. Use `-mode blocking` to verify the controlled local CRS test request is rejected. HTTPS uses an ephemeral self-signed certificate, so a local test client must explicitly trust it or disable verification for that isolated test only. The demo has a fixed in-process upstream and no Nginx; it is not the release proxy or a production service.
- Build the demo image with `docker build --network=host -t ariba-shield-waf-demo:local .` on Linux, then start it with `docker compose up -d --no-build`. Build host networking is used only so the build stage can download pinned Go modules when Docker's default DNS is unavailable; it does not give the running container host networking. The Compose file publishes only host-loopback ports `18080` and `18443` and applies a non-root, read-only-rootfs runtime with a restricted temporary filesystem. The same prototype is published as `boniyeamin/ariba-shield-waf-engine:latest`; it is not the integrated release image.
- To stop and remove the demo container/network, run `docker compose down` from the repository root. This prototype container has no persistent product data and is not the published single-image WAF release.

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
