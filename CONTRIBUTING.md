# Contributing

Thanks for helping. Bug reports, template contributions and code are all welcome.

## Report a bug or ask for a feature

Open an [issue](https://github.com/MiteshDhangar/beautify-alert/issues) with your Grafana version, the plugin version and what you expected. For template problems, include the template and the error from the preview.

## Share a template

Share a template by adding it to [`examples/templates/`](examples/templates/) with a comment at the top saying what it is for and which labels it expects. Templates that suit most users can become starters: add the file to `pkg/templates/starters/`, register it in `pkg/templates/starters.go` with a title, description and subject, and run `go test ./pkg/templates/` (every starter must render the sample alerts). Use tables for layout, keep CSS in one `<style>` block, and handle `.Problem` for Grafana's error alerts.

## Change the code

1. Read [docs/architecture.md](docs/architecture.md).
2. Start the dev environment: `npm install`, `npm run dev`, `mage -v build:linux`, `npm run server`.
3. Make the change with tests. Backend: `go test ./pkg/...`. Frontend: `npm run typecheck`, `npm run lint`, `npm run test:ci`. Browser: `npm run e2e`.
4. Open a pull request describing what changed and how you tested it.

Do not edit the `.config/` directory; it is managed by Grafana's `create-plugin` tool.

## Code of conduct

Be kind and assume good intent.
