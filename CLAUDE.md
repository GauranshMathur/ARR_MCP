# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run Commands
- Build: `go build -o arr-mcp ./cmd/arr-mcp`
- Test: `go test ./... -race`
- Run over stdio (desktop MCP clients): `./arr-mcp --transport stdio --config config.yaml`
- Run over HTTP (shared/in-cluster): `./arr-mcp --transport http --addr 0.0.0.0:8080`
- Check connectivity to every configured instance: `./arr-mcp --check`

There are no `--port` or `--host` flags; the HTTP transport takes a single
`--addr host:port`. See AGENTS.md for the repository conventions, which are
binding for any change here.

## Code Style Guidelines
- **Formatting**: Standard Go formatting (use `go fmt`)
- **Error Handling**: Use `fmt.Errorf("context: %w", err)` style for error wrapping
- **Naming**: 
  - Use CamelCase for public symbols, camelCase for private
  - ARR stack names use specific capitalization (SonarrClient, RadarrClient, etc.)
- **Imports**: Group standard library, then project imports with blank line separator
- **Documentation**: All exported types and functions must have doc comments
- **Error Messages**: Lowercase for error text, no trailing punctuation
- **Tool Pattern**: Register every MCP tool through the generic `register[In, Out]` helper in `pkg/server/tools.go` (there is no handler interface); inputs embed `InstanceArg`, and `toolMeta.access` declares the tier

## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues on `GauranshMathur/ARR_MCP`, driven by the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default vocabulary: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `CONTEXT.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.