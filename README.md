<p align="center">
  <img src="docs/assets/logo.svg" alt="ARR-MCP" width="360">
</p>

<p align="center">
  An <a href="https://modelcontextprotocol.io">MCP</a> server for the *arr media stack.<br>
  Let Claude, Cursor, VS Code or any other MCP client run your media server.
</p>

<p align="center">
  <a href="https://github.com/GauranshMathur/ARR_MCP/releases/latest"><img src="https://img.shields.io/github/v/release/GauranshMathur/ARR_MCP" alt="Latest release"></a>
  <a href="https://github.com/GauranshMathur/ARR_MCP/actions/workflows/ci.yml"><img src="https://github.com/GauranshMathur/ARR_MCP/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/GauranshMathur/ARR_MCP/pkgs/container/arr-mcp"><img src="https://img.shields.io/badge/ghcr.io-arr--mcp-blue?logo=docker&logoColor=white" alt="Container image"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/GauranshMathur/ARR_MCP" alt="License"></a>
</p>

<p align="center">
  <a href="#install-with-your-ai">Install with your AI</a> ·
  <a href="#60-second-quickstart">Quickstart</a> ·
  <a href="docs/clients.md">Client setup</a> ·
  <a href="docs/tools/README.md">Tools</a> ·
  <a href="#documentation">Docs</a>
</p>

---

- **Real MCP** — JSON-RPC 2.0 over stdio and Streamable HTTP, built on the official Go SDK
- **335 tools** — what you would otherwise do by clicking through each web UI
- **Multi-instance** — run two Sonarrs (4K and 1080p) and address them by name
- **Permission controls** — read-only, confirm-before-write, or full access
- **Single static binary**, distroless container, multi-arch image

## Supported services

| Service | Tools | | Service | Tools |
|---|---|---|---|---|
| [Sonarr](docs/tools/sonarr.md) | 77 | | [Maintainerr](docs/tools/maintainerr.md) | 25 |
| [Radarr](docs/tools/radarr.md) | 73 | | [Seerr](docs/tools/seerr.md) | 17 |
| [Bazarr](docs/tools/bazarr.md) | 33 | | [Jellyfin](docs/tools/jellyfin.md) | 23 |
| [Prowlarr](docs/tools/prowlarr.md) | 23 | | [Jellystat](docs/tools/jellystat.md) | 10 |
| [qBittorrent](docs/tools/qbittorrent.md) | 22 | | [Synology DSM](docs/tools/synology.md) | 14 |
| [NZBGet](docs/tools/nzbget.md) | 18 | | | |

Expose one service or all of them; anything you don't configure registers no tools.
What is planned and what is deliberately left out is in [docs/scope.md](docs/scope.md).

## Install with your AI

The fastest way in. Paste this into Claude, ChatGPT, Cursor, or whichever assistant you use:

```text
Read https://raw.githubusercontent.com/GauranshMathur/ARR_MCP/main/docs/ai-install-prompt.md
and follow it to set up ARR-MCP for me. It is an instruction document — act on it, don't
summarise it back to me.
```

That last sentence matters: without it, assistants tend to fetch the page and report on it
rather than doing anything.

It works out which MCP client you are using — often the one you are already talking to —
then asks only what it needs and either performs the setup itself or gives you exact steps
for your operating system, ending with a command to check it worked.

If your assistant can't browse the web, paste the contents of
[docs/ai-install-prompt.md](docs/ai-install-prompt.md) instead — it is self-contained and
needs no network access.

Would rather do it by hand? Carry on below.

## 60-second quickstart

The shortest path from nothing to a working server. No config file, no clone — just
environment variables and one container.

**1. Collect one URL and one API key** for each service you want to expose. See
[Find your API key](docs/configuration.md#find-your-api-key) if you don't know where they live.

**2. Write them into a `.env` file:**

```bash
cat > .env <<'EOF'
SONARR_URL=http://192.168.10.12:8989
SONARR_API_KEY=your-sonarr-api-key
RADARR_URL=http://192.168.10.14:7878
RADARR_API_KEY=your-radarr-api-key
EOF
```

Any service you leave out is simply not exposed. One service is enough to start.

**3. Check that the credentials work** before wiring up a client, so a mistake shows
up as a clear error rather than as a client that mysteriously has no tools:

```bash
docker run --rm --env-file .env ghcr.io/gauranshmathur/arr-mcp --check
```

```
OK    sonarr/default (http://192.168.10.12:8989)
OK    radarr/default (http://192.168.10.14:7878)
```

**4. Add it to your client.** For Claude Code:

```bash
claude mcp add arr -- docker run -i --rm --env-file /absolute/path/to/.env \
  ghcr.io/gauranshmathur/arr-mcp --transport stdio
```

That is the whole setup. [docs/clients.md](docs/clients.md) has copy-pasteable blocks for
Claude Desktop, Cursor, VS Code, Windsurf, Zed, Cline, Roo Code, Continue.dev, LibreChat,
Goose and anything else that speaks MCP.

> Use an **absolute** path for `--env-file`. Your MCP client starts the server from its own
> working directory, which is rarely the one you were standing in when you created `.env`.

### Prefer a long-running server?

Run it once over HTTP and point every client at the same endpoint:

```bash
docker run -d --name arr-mcp --env-file .env -p 8080:8080 \
  ghcr.io/gauranshmathur/arr-mcp
curl -s localhost:8080/health   # {"status":"ok"}
```

MCP is served at `http://localhost:8080/mcp`. The container defaults to
`--transport http --addr 0.0.0.0:8080`, so no arguments are needed. If you cloned the
repository, `docker compose up -d` does the same thing.

## Permissions

Every credential you hand ARR-MCP is full access to that application, so mutating tools are
gated by a policy. The default, `confirm`, asks you before any write or delete; `readonly`
hides mutating tools from the client entirely; `full` runs everything immediately. Any
instance can override the global policy. If writes are refused without a prompt, your client
does not support MCP elicitation: see [docs/permissions.md](docs/permissions.md) for the
`fallback` and `confirmScope` settings.


## Connecting a client

Full copy-pasteable configuration for every client below lives in
**[docs/clients.md](docs/clients.md)**. The two forms everything reduces to:

**stdio** — the client launches the process and talks over stdin/stdout:

```
docker run -i --rm --env-file /absolute/path/to/.env ghcr.io/gauranshmathur/arr-mcp --transport stdio
```

The `-i` is required: without it Docker gives the container no stdin and the JSON-RPC
handshake never completes.

**HTTP** — the server runs somewhere and the client connects to it:

```
http://localhost:8080/mcp
```

`/health` on the same port answers `{"status":"ok"}` and is not part of the MCP protocol;
it exists for container and Kubernetes probes.

## Kubernetes

Manifests for a Deployment, Service, ConfigMap and Secret template are in
[`deploy/kubernetes/`](deploy/kubernetes/), with a `kustomization.yaml` so they can be
pointed at directly by Argo CD or `kubectl apply -k`.

## Documentation

| | |
|---|---|
| [Client setup](docs/clients.md) | Copy-pasteable config for Claude, Cursor, VS Code, Windsurf, Zed and others |
| [Configuration](docs/configuration.md) | Finding API keys, environment variables, `config.yaml`, multiple instances, CLI flags |
| [Permissions](docs/permissions.md) | Modes, confirmation scope, per-instance overrides |
| [Tools](docs/tools/README.md) | Every tool by service, and what responses contain |
| [Troubleshooting](docs/troubleshooting.md) | `401`, connection refused, missing tools, refused writes |
| [Scope](docs/scope.md) | What is covered, what is planned, and what is left out on purpose |
| [Development](docs/development.md) | Building, testing, adding a service, releases |

## Contributing

Issues and pull requests are welcome. Start with [docs/development.md](docs/development.md);
[AGENTS.md](AGENTS.md) holds the repository conventions.

## License

[MIT](LICENSE)
