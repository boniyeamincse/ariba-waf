# Frontend Development

**Status:** Proposed design and implementation boundary; dashboard source is not present yet.

## 1. Runtime model

Build the Next.js dashboard as a static export. The final image serves HTML, CSS, JavaScript, and assets through the management listener; Node.js is not a production runtime. Authentication, authorization, validation, persistence, and configuration activation belong to the Go API. Do not rely on Next.js server actions, server sessions, or runtime API routes.

During local development, use the Next.js dev server and proxy same-origin `/api/v1` calls to the loopback Go API. See [local development](local-development.md) and [the API contract](api.md).

## 2. Proposed navigation

- Overview: requests, confirmed blocks, detections, enabled sites, component status, data freshness.
- Websites: list/create/edit, domains, listeners, upstreams, TLS, policy, enabled state.
- Policies: engine mode, CRS options, inspection limits, rate limit, scoped exclusions.
- Events: time/site/action/rule filters, redacted transaction detail, bounded pagination.
- Certificates: upload/inspect/expiry/site assignment; never reveal private keys.
- Configuration: desired vs applied state, validate/apply job, last-known-good revision, rollback.
- Administration: users/roles/sessions, audit, retention, backup/restore.

Advanced items such as IP reputation and scheduled reports are future candidates, not MVP commitments.

## 3. Interaction and state requirements

Every data view must distinguish loading, empty, stale, unavailable, partial, permission-denied, and error states. A missing event collector must not render zero events. Apply controls show the exact revision/generation and prevent duplicate/conflicting operations; enqueueing a job is not success until the API verifies activation.

Show a clear DetectionOnly explanation and that transport/body limits can still reject requests. Confirm destructive actions, display validation errors beside fields, and retain user input after recoverable failures. Never display secret values in lists or diagnostics.

## 4. Accessibility and localization

Use semantic headings/controls, keyboard-operable navigation, visible focus, sufficient contrast, labels, and screen-reader status announcements. Prefer concise English with Bangla support where practical; translations must preserve security distinctions such as detected versus blocked and desired versus applied.

## 5. Data and security rules

- Use relative `/api/v1` URLs and server-side authorization responses.
- Do not store session tokens or credentials in local storage.
- Include CSRF token and approved Origin handling for unsafe requests.
- Escape all user-controlled text; do not render raw WAF messages as HTML.
- Use bounded event pagination and avoid sending request bodies or secrets to the browser.
- Mark any fixture/demo data as demo; production views consume real API telemetry.

## 6. Verification

Add component tests for forms, validation, accessibility and state variants; contract tests for API schemas; and browser tests for enrollment, role restrictions, site configuration, apply/rollback and event viewing. Verify the exported static build in the final image rather than treating the development server as production behavior.
