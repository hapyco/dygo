# Security Policy

dygo is in early framework development. Security reports are still welcome, but the project does not have a formal bug bounty program or guaranteed response SLA yet.

## Supported Versions

Security fixes target the active development branch and the latest released dygo version once releases are published.

| Version | Supported |
| --- | --- |
| Active development branch | Yes |
| Latest tagged release | Yes, when releases exist |
| Older tags and commits | No, unless a maintainer states otherwise |

## Reporting a Vulnerability

Do not open a public issue with exploit details, secret values, private data, or proof-of-concept code.

Use GitHub's private vulnerability reporting for this repository:

- [Report a vulnerability privately](https://github.com/hapyco/dygo/security/advisories/new)

If that form is unavailable, open a public issue asking for a private security contact channel. Keep the public issue limited to a short request for contact and do not include technical details.

Include enough private detail for maintainers to reproduce and assess the issue:

- affected dygo version, commit, or branch
- affected surface, such as CLI, installer, generated project runtime, Studio, auth/session handling, encrypted secrets, database access, Record APIs, or SDK hooks
- setup steps and environment details
- impact and expected attacker capability
- reproduction steps or a minimal proof of concept
- any known mitigations or workarounds

## Safe Research Guidelines

Please keep testing local or on systems you own or are authorized to test. Do not attempt to access, modify, destroy, or exfiltrate data that is not yours.

The following activity is not acceptable:

- social engineering, phishing, or physical attacks
- denial-of-service testing against public infrastructure
- automated high-volume scanning of services not owned by you
- disclosure of secrets, customer data, or private project data
- persistence, lateral movement, or post-exploitation beyond what is needed to prove impact

## Coordinated Disclosure

After a private report is received, maintainers will review the issue, ask follow-up questions if needed, and decide whether a fix, advisory, documentation update, or configuration warning is appropriate.

Please keep vulnerability details private until maintainers have had a reasonable opportunity to investigate and publish a fix or mitigation.

## Public Issues

Public GitHub issues are appropriate for general hardening ideas, confusing security documentation, setup pain points, or non-sensitive bug reports. Use the private process above for anything that could help someone exploit dygo users or generated projects.
