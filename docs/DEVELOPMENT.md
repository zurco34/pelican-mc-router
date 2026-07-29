# Development

## Tech Stack

Backend
- Go

Dashboard
- Server-rendered Go templates

Router
- Chi

Logging
- Zerolog

Configuration
- Viper

Database
- SQLite
## Vulnerability scanning

Backend CI runs the pinned Go `govulncheck` scanner against all packages.
Address actionable findings before merging. If a finding cannot yet be fixed,
document the affected call path, mitigation, owner, and removal target in the
pull request; do not suppress it by weakening the CI gate.
