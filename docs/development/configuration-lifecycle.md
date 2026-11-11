# Configuration Lifecycle

**Status:** Desired website/policy state, proxy settings, transactional generation tracking, and read-only configuration status are implemented. The activation state machine below remains proposed; no activation implementation exists yet.

## 1. State model

Keep these concepts distinct:

- **Desired state:** editable sites, policies, certificates, exclusions, and settings.
- **Desired generation:** monotonic version incremented atomically by relevant edits.
- **Revision:** immutable snapshot of one desired generation with checksum and metadata.
- **Validated revision:** snapshot rendered and accepted by the exact installed Nginx/module/rule environment.
- **Applied revision:** configuration confirmed loaded and ready in the running proxy.
- **Activation job:** durable apply/rollback workflow with current stage and recovery status.

A successful database write or Nginx reload signal does not prove a revision is applied.

## 2. Apply sequence

1. Capture the expected desired generation and reject stale edits.
2. Serialize apply, rollback, and restore jobs; reject concurrent activation.
3. Render a complete immutable candidate from typed inputs and controlled templates.
4. Validate references, destination policy, certificates, policy semantics, Nginx syntax, connector loading, and CRS loading.
5. Persist validation output and activation intent before changing active state.
6. Atomically switch the active revision pointer and request graceful reload using a fixed executable and arguments.
7. Verify reload completion and perform bounded readiness checks against the intended revision/site.
8. Commit applied revision only after checks pass; record actor, times, checksums, and outcome.
9. On failure, restore the previous active pointer/config, reload it, and verify recovery. Report failure even if recovery succeeds.

Never interpolate browser input into a shell command or expose a general command-execution API.

## 3. Rollback and recovery

Rollback activates a retained known-good revision but does not rewrite current desired state. The UI must show pending differences. Revalidate the target against the currently installed engine because versions/rules may have changed.

Persist enough journal state to recover from process or container restart at each activation stage. At startup, reconcile interrupted jobs before accepting another activation. If recovery cannot verify the running config, mark readiness unavailable and provide an Admin-visible incident; do not report success.

## 4. Local development

When the Go API runs on the host and Nginx runs in another container, the API cannot safely reload that Nginx. Expose activation as unavailable in split mode and allow manual config testing only for local experiments. Do not simulate a successful apply. Exercise automated activation only in the integrated one-container runtime without a Docker socket mount.

## 5. User/API visibility

Every operation returns a durable job ID and status. Expose stage, sanitized validation diagnostics, target revision, prior revision, timestamps, result, and recovery status. Do not expose raw command output, filesystem paths, secrets, or stack traces. Distinguish queued, running, succeeded, failed, interrupted, and recovery failure.

## 6. Acceptance tests

Test valid apply, invalid render, module/rule load failure, reload timeout, readiness mismatch, rollback success/failure, crash at each journal boundary, simultaneous apply requests, stale desired generation, and restore/apply contention. Confirm last-known-good traffic continues whenever candidate validation fails.
