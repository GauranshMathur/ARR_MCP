# ARR-MCP

An [MCP](https://modelcontextprotocol.io) server for the \*arr media stack. Connect Claude, Cursor, or any MCP client to Sonarr, Radarr and Prowlarr — including **multiple instances of each**.

- **Real MCP** — JSON-RPC 2.0 over stdio and Streamable HTTP, built on the official Go SDK
- **Multi-instance** — run two Sonarrs (4K and 1080p) and address them by name
- **Permission controls** — read-only, confirm-before-write, or full access
- **36 tools** across Sonarr, Radarr and Prowlarr
- **Single static binary**, distroless container, multi-arch image

## Quick start

### Docker Compose

```bash
git clone https://github.com/GauranshMathur/ARR_MCP.git
cd ARR_MCP
cp .env.example .env      # fill in your URLs and API keys
docker compose up -d
```

Then point your MCP client at `http://localhost:8080/mcp`.

### stdio (Claude Desktop, Claude Code, Cursor)

```bash
claude mcp add arr -- docker run -i --rm --env-file /path/to/.env \
  ghcr.io/gauranshmathur/arr-mcp --transport stdio
```

Or build from source:

```bash
go install github.com/GauranshMathur/ARR_MCP/cmd/server@latest
claude mcp add arr -- server --transport stdio --config /path/to/config.yaml
```

## Configuration

Two ways, depending on whether you need multiple instances.

### Environment variables (one instance per service)

No config file needed:

```bash
SONARR_URL=http://192.168.10.12:8989
SONARR_API_KEY=...
RADARR_URL=http://192.168.10.14:7878
RADARR_API_KEY=...
PROWLARR_URL=http://192.168.10.18:9696
PROWLARR_API_KEY=...
```

### Config file (multiple instances)

Copy `config.example.yaml` to `config.yaml`. Secrets stay in the environment and are referenced as `${VAR}`, so the file is safe to commit or mount from a ConfigMap:

```yaml
services:
  sonarr:
    - name: main
      url: http://192.168.10.12:8989
      apiKey: ${SONARR_MAIN_API_KEY}
      default: true
    - name: anime
      url: http://192.168.10.13:8989
      apiKey: ${SONARR_ANIME_API_KEY}
```

Every tool then takes an optional `instance` argument. Omit it to use the one marked `default`. The configured names are advertised as a schema `enum`, so the model picks from a closed set rather than guessing:

```
sonarr_search_series{query: "Severance", instance: "anime"}
```

> An unset `${VAR}` is a startup error, never a silent empty value — an empty API key would otherwise surface much later as a confusing 401.

## Permissions

Mutating tools are gated by policy:

```yaml
permissions:
  mode: confirm        # readonly | confirm | full
  confirmScope: write  # write | destructive
  fallback: deny       # deny | allow
```

| Mode | Behaviour |
|---|---|
| `readonly` | Only read tools are registered. Mutating tools are invisible to the client. |
| `confirm` | Mutating tools ask the user first, via MCP elicitation. **Default.** |
| `full` | Everything runs immediately. |

`confirmScope` selects what gets confirmed: `write` covers both writes and deletes, `destructive` covers deletes only.

`fallback` decides what happens when the client **cannot** prompt — elicitation requires client support, and not every client has it. The default `deny` fails closed. Setting `allow` means a client without elicitation gets unprompted write access, so change it deliberately.

Any instance can override the global policy:

```yaml
  - name: anime
    url: http://192.168.10.13:8989
    apiKey: ${SONARR_ANIME_API_KEY}
    permissions:
      mode: readonly
```

All tools also carry MCP `readOnlyHint` / `destructiveHint` annotations, so clients can render their own warnings independently of this gating.

## Tools

### Sonarr (15)

| Tool | Access |
|---|---|
| `sonarr_list_series`, `sonarr_search_series`, `sonarr_list_episodes` | read |
| `sonarr_list_quality_profiles`, `sonarr_list_root_folders` | read |
| `sonarr_calendar`, `sonarr_queue`, `sonarr_history` | read |
| `sonarr_health`, `sonarr_disk_space`, `sonarr_system_status` | read |
| `sonarr_add_series`, `sonarr_run_command` | write |
| `sonarr_delete_series`, `sonarr_delete_queue_item` | destructive |

### Radarr (14)

Same shape with movies: `radarr_list_movies`, `radarr_search_movies`, `radarr_calendar`, `radarr_queue`, `radarr_history`, `radarr_health`, `radarr_disk_space`, `radarr_list_quality_profiles`, `radarr_list_root_folders`, `radarr_system_status` (read); `radarr_add_movie`, `radarr_run_command` (write); `radarr_delete_movie`, `radarr_delete_queue_item` (destructive).

### Prowlarr (7)

`prowlarr_search`, `prowlarr_list_indexers`, `prowlarr_indexer_stats`, `prowlarr_health`, `prowlarr_history`, `prowlarr_system_status` (read); `prowlarr_run_command` (write).

Services with no configured instances register no tools at all, so the advertised list always reflects what is actually reachable.

## CLI

```
--config PATH        path to config.yaml (or set ARR_MCP_CONFIG)
--transport stdio    stdio or http
--addr HOST:PORT     listen address for http
--log-level LEVEL    debug, info, warn, error
--check              test connectivity to every configured instance and exit
--version            print version
```

`--check` is the fastest way to validate credentials:

```
$ arr-mcp --config config.yaml --check
OK    sonarr/main (http://192.168.10.12:8989)
OK    sonarr/anime (http://192.168.10.13:8989)
FAIL  radarr/main (http://192.168.10.14:7878): radarr returned 401: Unauthorized
```

> Logging always goes to **stderr**. Under the stdio transport, stdout carries the JSON-RPC stream and must not be written to by anything else.

## Development

```bash
go test ./... -race -cover
go build -o arr-mcp ./cmd/server
```

Inspect the tool surface interactively:

```bash
npx @modelcontextprotocol/inspector ./arr-mcp --transport stdio --config config.yaml
```

Adding a service means describing its API rather than writing a new client — `ServiceSpec` carries the base path, health path and auth scheme, so services on different API versions and auth headers share one transport:

```go
var BazarrSpec = ServiceSpec{
    Name: "bazarr", BasePath: "/api", StatusPath: "/system/status",
    Auth: AuthHeaderKey, AuthHeader: "X-API-KEY",
}
```

Releases are cut by [release-please](https://github.com/googleapis/release-please): conventional commits on `main` accumulate into a version-bump PR, and merging it tags the release and publishes multi-arch images to GHCR. Images are Trivy-scanned **before** push, so a vulnerable tag is never publicly pullable.

## License

See [LICENSE](LICENSE).
