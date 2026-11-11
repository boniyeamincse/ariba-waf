# Project Structure

**Status:** Partially implemented. The tree below shows what exists today plus the planned directories needed for the integrated release. A directory appearing in the planned section is not evidence that its components exist.

## 1. Current repository layout

```text
ariba-waf/
├── cmd/
│   ├── ariba/                 # Go management API, enrollment CLI, and healthcheck
│   └── waf-demo/              # Loopback Coraza/CRS demo proxy
├── internal/
│   ├── api/                   # Root status page and health handlers
│   ├── management/            # SQLite store, auth/sessions, users, sites, policies, proxy settings
│   └── waf/                   # Coraza + CRS adapter and synthetic tests
├── frontend/                  # Next.js static-export dashboard prototype
│   ├── app/                   # Routes: setup, dashboard, websites, policies, events, users, account, settings
│   ├── components/            # Shared UI and auth/session shell
│   └── lib/                   # Same-origin /api/v1 client and formatting helpers
├── tests/
│   └── integration/           # Management container packaging check
├── docs/                      # Product, architecture, development, security, testing, operations
├── dev-data/                  # Ignored local runtime data; never commit
├── Dockerfile                 # Coraza/CRS demo image
├── Dockerfile.api             # Local management API image
├── compose.yaml               # Local demo and profile-gated management service
├── .dockerignore
├── .gitignore
├── go.mod
└── go.sum
```

The Next.js project keeps its own `package.json` and lockfile under `frontend/`; there is no root workspace manifest.

## 2. Planned additions

These paths are proposed for the integrated release and do not exist yet:

```text
├── internal/
│   ├── config/                # Domain models, validation, rendering, activation
│   ├── events/                # Audit log ingestion, redaction, indexing, queries
│   ├── jobs/                  # Durable bounded background jobs
│   ├── backup/                # Consistent archive, encryption, validation, restore
│   └── storage/               # Split-out migrations and repositories if the store grows
├── deploy/
│   ├── nginx/                 # Nginx templates and module wiring
│   ├── modsecurity/           # Engine configuration and CRS integration
│   ├── supervisor/            # Selected PID 1/supervisor configuration
│   └── docker/                # Final-image build assets
└── tests/
    ├── integration/           # Real proxy/API/database tests beyond the current container check
    ├── security/              # Access-control and boundary tests
    └── fixtures/              # Sanitized test traffic/configuration
```

## 3. Boundaries

- `cmd/` contains composition/entry points; reusable application logic belongs under `internal/`.
- Frontend calls only the versioned management API. It does not own security decisions or persistent state.
- Nginx and ModSecurity templates are rendered from typed, validated data; never concatenate arbitrary user input into directives or shell commands.
- `/data` is runtime state and must not be embedded in the image or committed to Git.
- Tests must exercise the final Nginx/module combination, not merely mocks of the proxy.
- The two current images are separate on purpose: `Dockerfile` is the isolated engine demo and `Dockerfile.api` is the management foundation. The integrated release image is still unimplemented.

## 4. Repository hygiene

Ignore local databases, WAL/SHM files, logs, generated certificates/private keys, enrollment tokens, local backups, build outputs, and `dev-data/`. Keep sanitized fixtures and configuration samples only. Treat generated production config as immutable runtime data rather than source templates.

## 5. Pending decisions

Confirm the frontend package manager (npm today), migration tooling, generated-file policy, whether the management store should split into `internal/storage`, and exact deployment asset ownership before creating the planned directories.
