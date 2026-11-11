# Security Policy

## Project status

Ariba Shield WAF has an early Go WAF prototype and management API foundation. There is no supported software release or production deployment. The security design is not a certification or guarantee of protection.

## Reporting a vulnerability

Please do not publish exploitable details, credentials, private keys, customer data, or proof-of-concept traffic in a public issue. Until a private security contact is published, use the repository host's private vulnerability-reporting feature if available. If it is unavailable, contact the project maintainers through a private channel and ask them to establish one; do not send secrets through public discussions.

Include only the information needed to triage: affected revision or component, impact, prerequisites, safe reproduction summary, and suggested mitigation. Coordinate disclosure with maintainers. Avoid testing against systems you do not own or have explicit permission to assess.

## Response expectations

A response SLA is not established yet. Maintainers should acknowledge reports, assess impact and affected versions, coordinate remediation, and publish an advisory after a fix is available. Do not promise timelines before maintainers adopt them.

## Supported versions

None yet. This section will list supported release branches after the first tested release.

## Handling secrets

Never commit credentials, bootstrap tokens, TLS private keys, or real request data. If a secret is exposed, revoke or rotate it; deleting the committed copy alone is not sufficient. See [the product security model](docs/security/security.md).
