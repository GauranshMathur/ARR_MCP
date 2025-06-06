# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run Commands
- Build: `go build -o arr-mcp ./cmd/server`
- Run: `./arr-mcp`
- Run with flags: `./arr-mcp --port 9000 --host 0.0.0.0`

## Code Style Guidelines
- **Formatting**: Standard Go formatting (use `go fmt`)
- **Error Handling**: Use `fmt.Errorf("context: %w", err)` style for error wrapping
- **Naming**: 
  - Use CamelCase for public symbols, camelCase for private
  - ARR stack names use specific capitalization (SonarrClient, RadarrClient, etc.)
- **Imports**: Group standard library, then project imports with blank line separator
- **Documentation**: All exported types and functions must have doc comments
- **Error Messages**: Lowercase for error text, no trailing punctuation
- **Handler Pattern**: Implement `api.Handler` interface for all MCP tools