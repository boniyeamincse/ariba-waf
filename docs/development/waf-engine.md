# WAF Engine Development

**Status:** Proposed behavior; Nginx/ModSecurity/CRS integration is not implemented or compatibility-tested yet.

## 1. Components and responsibilities

- Nginx terminates configured listeners and routes to approved upstreams.
- **Engine Options:**
  - *Option A (C/C++):* ModSecurity v3 with a pinned Nginx connector. Requires strict ABI verification.
  - *Option B (Go/Memory-Safe):* OWASP Coraza (via Cgo/Wasm connector or native Go reverse proxy). Native Go metrics, zero C++ segfault risks, and direct CRS v4 integration.
- A pinned OWASP CRS release (targeting CRS v4 for lower false positives and improved modularity) supplies baseline rules.
- Go renders configuration from typed models and requests constrained validation/activation.

The exact Nginx build, connector, libModSecurity/Coraza, CRS version, base image, and supported architectures must be pinned and tested together in the final image. Never assume ABI compatibility based on a development host.

## 2. Pipeline stages (Order of execution)

To prevent resource exhaustion, requests must pass through stages in order:
1. **Transport & TLS Termination:** Cipher suites, HTTP/2 or HTTP/3, and trusted proxy header resolution.
2. **Layer 7 Rate Limiting & Burst Protection:** Token bucket per client IP/site. Block or 429 requests *before* expensive regex parsing.
3. **Request Normalization & Body Bounding:** Reject oversized payloads before buffer allocation.
4. **WAF Rule Engine (CRS v4):** Anomaly scoring across request line, headers, cookies, and parsed body.
5. **Upstream Forwarding:** Reverse proxy to validated upstream with strict timeouts and TLS verification.

## 3. Modes

- `off`: no rule inspection; proxy/TLS and transport limits remain active.
- `detectionOnly`: record rule matches without blocking; Nginx/body/rate limits may still reject traffic.
- `blocking`: enforce configured CRS anomaly thresholds and document separate transport/limit rejection behavior.

Start new sites in DetectionOnly. Require representative traffic review and explicit operator action before enabling Blocking. UI and API must not label a detection as a block.

## 4. Inspection behavior

Configure parsers for supported JSON, URL-encoded form, and multipart content types. Set bounded request body, file-upload, header, timeout, and resource limits. Define behavior for body-too-large, parser-error, unsupported content type, partial inspection, and unavailable engine. Show incomplete inspection and actual action in events; do not claim every request was fully inspected.


Avoid request/response body retention by default. Redact sensitive values before event persistence. Limit per-transaction audit output and total log/queue/disk usage.

## 5. Rules and exclusions

Pin and checksum CRS artifacts. Apply only structured supported tuning and site-scoped exclusions in the first release. Each exclusion needs a rule/target/path scope, reason, actor, and optional expiry; expiration takes effect only after a validated configuration activation. No arbitrary shell, raw Nginx directives, or unbounded custom rule upload in the initial scope.

## 6. Upstreams and network trust

Allow only administrator-approved destinations. Validate hostnames, resolved addresses, ports, schemes, and every re-resolution/connection to avoid DNS rebinding gaps. Deny cloud metadata services, management listeners, and unintended loopback targets while permitting explicitly approved private upstreams. Verify upstream TLS by default; do not add an insecure TLS bypass.

Trust client-IP headers only from configured proxy CIDRs and a defined chain. Ignore visitor-supplied forwarding headers otherwise.

## 7. Events and operational isolation

Use a bounded worker to read ModSecurity audit records, resume from file identity/offset, handle rotation/partial records, and deduplicate transactions. Logging/collector failure must not stall public proxy traffic. Expose backlog, dropped records, last-ingested time, and disk pressure. Keep access transaction counts separate from rule-match counts.

## 8. Tests required

Run a controlled benign request, CRS match in DetectionOnly, enforced match in Blocking, malformed body, oversized body, multipart upload, upstream TLS failure, forbidden destination, trusted/untrusted proxy header, log rotation, queue saturation, Nginx config failure, graceful reload, and restart during activation—all against the exact release image. Test requests must be safe and isolated to systems owned by the project.

