# Contributing

Thank you for your interest in Ariba Shield WAF. The project is currently in the design/documentation phase. The development workflow below is proposed and will be updated when source code, CI, and release processes exist.

## Before contributing

1. Read the [project status](README.md), [requirements](docs/planning/requirements.md), and [architecture](docs/architecture/architecture.md).
2. For security-sensitive changes, read [SECURITY.md](SECURITY.md) and [the product security model](docs/security/security.md).
3. Open an issue or discussion for substantial changes so scope and acceptance criteria can be agreed before implementation.
4. Do not submit real secrets, production keys, private customer data, or unredacted WAF traffic.

## Proposed change workflow

- Keep changes focused and explain user impact, design choices, and risks.
- Update affected documentation alongside implementation. Mark proposals, assumptions, and verified behavior distinctly.
- Add or update tests for behavior, authorization, error handling, failure recovery, and security boundaries.
- Do not claim checks passed unless they were actually run; include the command and result in the pull request.
- Avoid unrelated formatting or generated-file churn.

## Pull request checklist

- [ ] The change matches an accepted requirement or links to a decision.
- [ ] API, architecture, and user-facing documentation are synchronized.
- [ ] Tests cover normal, invalid, unauthorized, and failure cases as relevant.
- [ ] No secrets, private data, or unsafe defaults are introduced.
- [ ] Build/test/lint results and any unverified areas are stated clearly.
- [ ] Third-party code, dependencies, and license notices are reviewed.

## Current limitations

There is no established code layout, build command, CI pipeline, issue template, or release branch policy yet. Do not invent or rely on repository checks until they are added and documented. See [the roadmap](docs/planning/roadmap.md) for proposed next steps.
