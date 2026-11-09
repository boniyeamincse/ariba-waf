# Proposed Project Structure

**Status:** Recommendation for implementation; directories below are not evidence that these components exist.

```text
ariba-shield-waf/
├── cmd/
│   └── ariba/                 # Go API entry point and local admin CLI
├── internal/
│   ├── api/                   # HTTP routing, middleware, request/response types
│   ├── auth/                  # Enrollment, passwords, sessions, CSRF, RBAC
│   ├── config/                # Domain models, validation, rendering, activation
│   ├── events/                # Audit log ingestion, redaction, indexing, queries
│   ├── jobs/                  # Durable bounded background jobs
│   ├── storage/               # SQLite migrations and repositories
│   └── backup/                # Consistent archive, encryption, validation, restore
├── frontend/                  # Next.js static-export dashboard
├── deploy/
│   ├── nginx/                 # Nginx templates and module wiring
│   ├── modsecurity/           # Engine configuration and CRS integration
│   ├── supervisor/            # Selected PID 1/supervisor configuration
│   └── docker/                # Final-image build assets
├── tests/
│   ├── integration/           # Real proxy/API/database tests
│   ├── security/              # Access-control and boundary tests
│   └── fixtures/              # Sanitized test traffic/configuration
├── docs/                      # Product, development, security, and operations docs
├── dev-data/                  # Ignored local runtime data; never commit
├── Dockerfile
├── .dockerignore
├── .gitignore
├── go.mod
└── package.json               # Frontend workspace/build metadata
```

## Boundaries

- `cmd/` contains composition/entry points; reusable application logic belongs under `internal/`.
- Frontend calls only the versioned management API. It does not own security decisions or persistent state.
- Nginx and ModSecurity templates are rendered from typed, validated data; never concatenate arbitrary user input into directives or shell commands.
- `/data` is runtime state and must not be embedded in the image or committed to Git.
- Tests must exercise the final Nginx/module combination, not merely mocks of the proxy.

## Repository hygiene

Ignore local databases, WAL/SHM files, logs, generated certificates/private keys, enrollment tokens, local backups, build outputs, and `dev-data/`. Keep sanitized fixtures and configuration samples only. Treat generated production config as immutable runtime data rather than source templates.

## Pending decisions

Confirm whether this is a Go multi-module workspace or a single Go module, the frontend package manager, migration tool, generated-file policy, and exact deployment asset ownership before creating implementation directories.
