## Project knowledge

This repository contains a **Grafana plugin**. You must Read @./.config/AGENTS/instructions.md before doing changes.

## Beautify Alert

- Architecture and package responsibilities: docs/architecture.md. Template data and functions: docs/templates.md.
- Backend packages depend one way: plugin -> notify -> routing/templates/delivery/history -> alert/config.
- New email providers implement `delivery.Sender` and are returned by `delivery.New`.
- Run `go test ./pkg/...`, `npm run typecheck`, `npm run lint` and `npm run test:ci` before committing.
