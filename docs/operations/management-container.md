# Local management API container

The management foundation has its own local image, `ariba-shield-management-api:local`, built from `Dockerfile.api`. It includes accounts/sessions, roles, desired websites/policies, proxy settings and SQLite. It is independent of the Coraza demo image and is not the integrated WAF release or a dashboard.

## Build and start

From the repository root:

```sh
docker build --network=host --pull -f Dockerfile.api -t ariba-shield-management-api:local .
docker compose up -d --no-build --wait management-api
docker compose ps management-api
```

Build host networking is used only so the build stage can reach `proxy.golang.org` when Docker's default DNS cannot resolve it; the running container does not use host networking. Omit `--network=host` on hosts where the Docker build network resolves external DNS normally.

An unsigned development preview is also published to Docker Hub as `boniyeamin/ariba-shield-control-api:latest`. It is the same foundation image with a different tag and is not a supported release.

The `management-api` service is in the `management` profile. Naming it explicitly starts it without starting the WAF demo. Open `http://127.0.0.1:19000/` for the static API status page, endpoint links and bootstrap instructions. The container does not serve the dashboard; the separate `frontend/` prototype runs through the Next.js dev server. Only host loopback is published. To use another local port, set `ARIBA_API_PORT` consistently when running Compose commands, for example `ARIBA_API_PORT=19001 docker compose up -d --no-build --wait management-api`. This updates both the published port and the allowed browser Origin.

The process listens on container port 9000 using the explicit `-allow-remote-listen` container opt-in. Host-run API commands still bind to loopback by default and reject non-loopback addresses without that flag. The Compose service uses development HTTP cookies and an exact loopback Origin; it is a local development service.

## Bootstrap and API access

Generate a setup token in the running container:

```sh
docker compose exec -T management-api /ariba -db /data/db/ariba.db -enrollment-token
```

Keep the printed token private. It expires in ten minutes and is refused after initial setup. The image has no default credentials. Use the [setup/login sequence and API examples](../development/api.md), replacing the host address with `http://127.0.0.1:19000` and the mutation Origin with that same exact value. Retain cookies and refresh the CSRF token after login or reauthentication. The token command must use the same `/data/db/ariba.db` database as the service.

## Persistence and runtime

Compose mounts the named `management-data` volume at `/data`. The image seeds a private `db/` directory owned by UID/GID 65532, allowing a fresh Docker-managed volume to initialize without running the API as root. SQLite databases use mode `0600` and the database directory uses `0700`. Existing bind mounts need matching ownership/permissions; the service does not change arbitrary host directory ownership.

The image runs as UID/GID `65532:65532` on a read-only root filesystem with capabilities dropped, `no-new-privileges`, a bounded temporary filesystem, PID/memory/CPU limits and bounded container log rotation. The service has its own Compose network. No Docker socket or privileged container is used.

To stop or replace the API while keeping its database:

```sh
docker compose stop management-api
docker compose up -d --no-build --force-recreate --wait management-api
```

Accounts, sessions, policies, sites and desired generation live in the volume and survive container replacement. Removing the volume deletes that state. Image replacement does not replace an existing volume's contents; see [Docker's volume lifecycle documentation](https://docs.docker.com/engine/storage/volumes/#a-volumes-lifecycle).

## Health and checks

`/ariba -healthcheck` probes `http://127.0.0.1:9000/health/live` without opening the database, following redirects or using an HTTP proxy. Docker runs this command directly; the distroless runtime does not need a shell or curl. Health checks assume internal port 9000; override the healthcheck too if using a custom internal listen port outside Compose.

- `/health/live` returns `200` while the API responds. Docker health status uses this liveness endpoint.
- `/health/ready` remains `503 NOT_READY` because WAF activation is not connected. A healthy local API container does not establish production WAF readiness.
- Desired configuration is saved but never activated by this container.

Run the container integration check after building the image:

```sh
python3 tests/integration/management_container.py
```

The check uses a unique Compose project, a free loopback port and a disposable named volume. It verifies runtime restrictions, health, setup/login, website/policy persistence, session persistence across container recreation, configuration generation, and logout. It removes only its own containers/network/volume afterward and leaves the normal development service intact. Docker Engine, Compose v2 and Python 3 are required.
