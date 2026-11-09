# Troubleshooting Guide

**Status:** Diagnostic guide for the proposed design. Exact logs/commands depend on the future release and must be confirmed against the image.

## Safety first

Do not expose management ports publicly while troubleshooting. Do not run guessed privileged commands, mount the Docker socket, edit active configuration by hand in production, or paste credentials/private keys/log bodies into public tickets. Capture image digest, sanitized status, timestamps, and request/job IDs.

## Common symptoms

| Symptom | Likely area | Safe diagnostic direction |
| --- | --- | --- |
| Dashboard unavailable | Container, management listener, TLS, static assets | Check container state and health endpoint from the host; verify loopback port binding and TLS certificate. |
| Login/setup rejected | Enrollment state, token expiry, cookies, CSRF, Origin | Confirm setup status; use a fresh local token; check browser origin/cookie policy; never reuse an expired token. |
| Website returns 502/504 | Upstream reachability, DNS, port, upstream TLS, timeout | Verify configured destination is allowed and reachable from the container network; inspect sanitized proxy reason. |
| Unexpected block | Policy mode, CRS rule, threshold, applied revision | Inspect redacted event, matched rule, site and applied revision; tune a scoped exclusion with reason, then validate/apply. |
| Detection visible but request passed | DetectionOnly mode | This is expected: DetectionOnly records matches without WAF blocking. Verify whether another transport limit rejected it. |
| Dashboard shows zero/stale metrics | Collector lag/failure, unavailable source | Check `generatedAt`, ingestion cursor/lag, dropped-record count, and availability; zero is not proof of no traffic. |
| Apply job fails | Candidate config, module/CRS loading, certificate, reload/readiness | Read sanitized job stage and diagnostics; current applied revision should remain known-good. Do not manually edit active files. |
| Service not ready after restart | Interrupted activation or recovery | Treat as a recovery incident; inspect activation job/journal and Admin health details before retrying apply. |
| Events missing/duplicated | Log rotation, offsets, partial records, deduplication | Check collector cursor/lag and rotation status; preserve logs for controlled investigation without publishing sensitive records. |
| Disk usage high | Event retention, audit volume, backups, WAL | Check volume capacity and configured retention; preserve required backups/audit evidence and avoid unbounded manual deletion. |
| Certificate upload/HTTPS fails | PEM parse/key match/SAN/validity/listener TLS | Check sanitized validation result and certificate metadata; never share the private key for debugging. |
| Restore rejected | Wrong passphrase, archive integrity, schema/version, limits | Re-run validation with the correct local passphrase; verify artifact version and disk headroom; do not bypass archive checks. |

## Incident handling

For suspected compromise, restrict management access, preserve relevant image digest and audit evidence, rotate exposed credentials/keys, and use the private reporting channel in [SECURITY.md](../../SECURITY.md). Do not delete the data volume before deciding whether forensic preservation is needed.

## Information to include in a support report

- Product version/image digest and host architecture.
- Approximate time in UTC and affected site/job/request ID.
- Relevant sanitized health and job status.
- Recent configuration revision and whether it is desired, validated, or applied.
- Steps to reproduce using a synthetic site/fixture.
- Omit credentials, session cookies, private keys, raw bodies, and personal data.

Exact CLI/log collection commands should be added only after the release interface is implemented and tested.
