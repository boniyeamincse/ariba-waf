# Deployment Plan

**Status:** Target deployment design only. There is no supported deployment release.

A separate local management API image and persistent Compose service are available for development; see [the management container guide](management-container.md). It does not include the integrated WAF/dashboard described below.

Unsigned development preview images are published to Docker Hub for convenience and are not a supported release:

- `boniyeamin/ariba-shield-waf-engine:latest` — isolated Coraza/CRS demo proxy.
- `boniyeamin/ariba-shield-control-api:latest` — local management API foundation.

Do not treat these tags as a production artifact, a compatibility matrix, or a security-backed release. The real deployment contract below remains unimplemented.

## 1. Deployment contract

Target: one versioned Docker image, one running container, and one persistent volume mounted at `/data`. No separate database, cache, frontend, or backend container is required. Protected applications stay outside the container. High availability and shared multi-instance state are not supported by this design.

The sample image name/version and port mapping in the legacy architecture document are placeholders, not runnable release instructions. Do not substitute guessed registry tags.

## 2. Planned ports

| Host mapping | Container listener | Purpose | Proposed exposure |
| --- | --- | --- | --- |
| Host 80 → 8080 | HTTP | Protected-site traffic | Publish only when used |
| Host 443 → 8443 | HTTPS | Protected-site traffic | Publish when TLS sites are configured |
| Host loopback 9443 → 9443 | Management HTTPS | Dashboard and management API | Loopback by default |
| None | 127.0.0.1:9000 | Internal Go API | Never publish directly |

Verify exact bindings in the released container before use. Remote administration should use a deliberately configured management interface, firewall rules, and authenticated transport such as a managed VPN/SSH tunnel.

## 3. Installation prerequisites and checklist

Before a release runbook is published, document supported Docker Engine/Compose versions, host architectures, filesystem requirements, volume ownership, TLS bootstrap, backup, firewall, upgrade, and rollback procedures.

Planned operator flow:

1. Verify the image signature/digest and release notes.
2. Create and protect a persistent `/data` volume with correct non-root ownership.
3. Configure public site ports and loopback-only management binding.
4. Start the container and verify readiness/health.
5. Generate a short-lived bootstrap token through the local CLI; create the first Admin and disable enrollment.
6. Add a site in DetectionOnly, inspect events, test the upstream, then deliberately enable Blocking after tuning.
7. Create an encrypted backup and verify recovery procedure before production use.

## 4. Runtime requirements

Use a dedicated non-root runtime identity where compatible; the Go API should not need root. Supervisor forwards signals, reaps children, and stops on unrecoverable critical process failure. Docker health status is not itself a restart policy; specify process failure handling and container restart behavior.

The image must contain pinned Nginx, connector, libModSecurity, CRS, Go binary, static assets, and supervisor; it must not need a Node.js server at runtime. Validate exact module ABI and startup health in the release artifact.

## 5. Upgrade and rollback

Upgrade procedure is pending implementation. It must preserve volume data, back up before incompatible schema changes, define supported schema transitions, validate active configuration against new engine/rules, and specify image rollback compatibility. Never assume reverting an image can safely revert a migrated database. Test upgrade and rollback on a copy of representative state.

## 6. Release checklist

- [ ] Image digest and signature verified.
- [ ] SBOM, third-party notices, changelog, and compatibility matrix published.
- [ ] Fresh install, persistence, routing, enforcement, and final-image tests passed.
- [ ] Management access remains restricted.
- [ ] Bootstrap token lifecycle and no-default-credential behavior verified.
- [ ] Backup and restore tested on a fresh instance.
- [ ] Upgrade/recovery limitations and support window documented.
