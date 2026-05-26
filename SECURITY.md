# Security Policy

Beekeeper is a real-time safety harness for autonomous coding agents. Because it
sits on the security-critical path, we treat its own supply chain and disclosure
process as part of the product. This policy describes how to report a
vulnerability and what to expect in return.

## Supported Versions

Security fixes are provided for the following releases:

| Version  | Supported          |
| -------- | ------------------ |
| v0.1.0+  | :white_check_mark: |
| < v0.1.0 | :x:                |

Pre-release (`< v0.1.0`) builds are not supported; please upgrade to the latest
v0.1.0+ release before reporting.

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.** Public issues
disclose the problem before a fix is available and put users at risk.

Instead, report privately using **GitHub private security advisories**:

1. Go to the repository's **Security** tab.
2. Select **Report a vulnerability** to open a private advisory.
3. Include a description, affected version(s), reproduction steps or a
   proof-of-concept, and the impact you observed.

GitHub private security advisories are the only supported intake channel. They
keep the report confidential between you and the maintainers until a coordinated
fix is published.

## Our Commitment

| Stage                     | Target                                                    |
| ------------------------- | --------------------------------------------------------- |
| Acknowledgment            | Within **48 hours** of receiving your report              |
| Triage & severity         | As soon as practical after acknowledgment                 |
| Coordinated disclosure    | **90 days** by default, from acknowledgment to public fix |

We follow a coordinated disclosure model. We aim to release a fix and a public
advisory within the 90-day default window, and we will keep you updated on
progress. If a fix needs longer, we will agree on a revised timeline with you. We
are happy to credit reporters in the advisory unless you prefer to remain
anonymous.

## Supply-Chain Integrity

Every released binary is:

- **Reproducibly built** — deterministic Go build flags
  (`-trimpath -buildvcs=false -mod=readonly`); verify locally with
  `make verify-release VERSION=X.Y.Z`.
- **Keylessly signed** — Sigstore/cosign v3 keyless signing via GitHub Actions
  OIDC. No long-lived signing keys exist to be stolen.
- **Dependency-pinned** — `go.mod` + `go.sum` with CI `go mod verify`; updates
  are proposed by Renovate and require human review.

## Maintainer / Contact

Maintained by **Mzansi Agentive Pty Ltd** — Mfanafuthi Mhlanga.

For security matters, always use the GitHub private security advisory process
described above rather than direct email, so reports are tracked and
confidential.
