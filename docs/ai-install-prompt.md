# Install ARR-MCP with your AI assistant

There are two ways to use this.

**If your assistant can browse the web**, this one line is enough:

```text
Set up the ARR-MCP server for me, following the guide at
https://raw.githubusercontent.com/GauranshMathur/ARR_MCP/main/docs/ai-install-prompt.md
```

**Otherwise**, copy the block below and paste it in directly — it is self-contained and
needs no network access. Use the copy button in the corner of the block.

Either way, the assistant interviews you about your setup and hands back working
configuration instead of guessing. Answer its questions and follow the steps it gives you.

> Your API keys stay between you and your \*arr apps. The prompt tells the assistant not to
> ask for them unless a step genuinely needs one, and to keep them out of files you might
> commit.

~~~~
You are helping me install and configure ARR-MCP, an MCP server for the *arr media stack.
Follow the facts in this message exactly. Do not invent flags, config keys, or file paths —
if something here does not cover my situation, say so and ask.

# What ARR-MCP is

An MCP server exposing Sonarr, Radarr, Prowlarr and Bazarr to any MCP client, with support
for MULTIPLE INSTANCES of each service (e.g. a 1080p Sonarr and a 4K Sonarr).

- Repository: https://github.com/GauranshMathur/ARR_MCP
- Container image: ghcr.io/gauranshmathur/arr-mcp
- 105 tools total: Sonarr 43, Radarr 41, Bazarr 14, Prowlarr 7
- Transports: stdio (desktop/editor clients) and Streamable HTTP (shared/self-hosted)

# How I want you to help

Interview me FIRST, then produce configuration. Ask, in this order, and wait for answers:

1. Which of Sonarr, Radarr, Prowlarr, Bazarr do I run, and what URL is each reachable at?
2. Do I run more than one instance of any of them? If yes, what should each be called?
3. Which MCP client am I connecting? (Claude Code, Claude Desktop, Cursor, VS Code,
   Windsurf, Zed, Cline, Roo Code, Continue.dev, Goose, LibreChat, or something else)
4. Docker, or a local binary?
5. Should the server be able to make changes (add/delete media), or read-only?

Then give me, in one reply:
- the exact `.env` and/or `config.yaml` content for my answers
- the exact client configuration block, at the exact file path for my OS
- the verification command to run, and what correct output looks like

Rules:
- Do NOT ask for my API keys, and do not put them in your replies. Use placeholders like
  ${SONARR_API_KEY} and tell me where to paste the real value myself.
- Never suggest committing a `.env` or a `config.yaml` containing real keys to git.
- If I say something contradicts the facts below, trust the facts and tell me.

# Requirements

- Docker route: Docker only.
- Binary route: Go 1.25 or newer.
- Network access from wherever ARR-MCP runs to each *arr service.
- An API key for each service I want to use.

# Installation

Docker (no install step; the client will run the container):
  ghcr.io/gauranshmathur/arr-mcp

Local binary:
  go install github.com/GauranshMathur/ARR_MCP/cmd/arr-mcp@latest
  # installs to $(go env GOPATH)/bin/arr-mcp

# Command line interface

  --config PATH      path to config.yaml; omit to configure from environment variables
  --transport VALUE  stdio or http (default stdio)
  --addr HOST:PORT   listen address for the http transport (default 0.0.0.0:8080)
  --log-level VALUE  debug, info, warn, error (default info)
  --check            test connectivity to every configured instance, then exit
  --version          print the version and exit

There are NO --port or --host flags. The HTTP transport takes a single --addr.
Over HTTP the MCP endpoint is /mcp and there is a plain /health probe.

# Configuration: two ways

## Way A — environment variables only (ONE instance per service, no config file)

  SONARR_URL=http://192.168.1.10:8989
  SONARR_API_KEY=...
  RADARR_URL=http://192.168.1.11:7878
  RADARR_API_KEY=...
  PROWLARR_URL=http://192.168.1.12:9696
  PROWLARR_API_KEY=...
  BAZARR_URL=http://192.168.1.13:6767
  BAZARR_API_KEY=...

Set only the services I actually run. BOTH the _URL and _API_KEY must be set for a service
or it is skipped silently and none of its tools appear.

## Way B — config.yaml (REQUIRED for multiple instances of a service)

  server:
    transport: stdio          # stdio | http
    addr: 0.0.0.0:8080
    logLevel: info

  permissions:
    mode: confirm             # readonly | confirm | full
    confirmScope: write       # write | destructive
    fallback: deny            # deny | allow

  services:
    sonarr:
      - name: hd
        url: http://192.168.1.10:8989
        apiKey: ${SONARR_HD_API_KEY}
        default: true
      - name: 4k
        url: http://192.168.1.14:8989
        apiKey: ${SONARR_4K_API_KEY}
    radarr:
      - name: main
        url: http://192.168.1.11:7878
        apiKey: ${RADARR_API_KEY}
        default: true

Notes that matter:
- ${VAR} is read from the environment, so config.yaml holds no secrets and can be committed
  or mounted from a ConfigMap. An UNSET variable is a startup error, not an empty value.
- Exactly one instance per service may set `default: true`. Tools take an optional
  `instance` argument; omitted, it uses the default.
- Valid service names are exactly: sonarr, radarr, prowlarr, bazarr. Anything else is a
  startup error.
- Any instance may override permissions with its own `permissions:` block.

Default ports, if I do not know my URLs: Sonarr 8989, Radarr 7878, Prowlarr 9696,
Bazarr 6767.

Where to find each API key:
- Sonarr / Radarr / Prowlarr: Settings -> General -> Security -> API Key
- Bazarr: Settings -> General -> Security -> API Key

# Permissions — read this before choosing

  mode: readonly   only read tools are registered; write tools are invisible to the client
  mode: confirm    write tools ask me to approve before running   (DEFAULT)
  mode: full       everything runs immediately

  confirmScope: write        confirm both writes and deletes  (DEFAULT)
  confirmScope: destructive  confirm deletes only

  fallback: deny   refuse the action if the client cannot ask me   (DEFAULT)
  fallback: allow  run it anyway

IMPORTANT, this causes the most confusion: `confirm` mode asks via the MCP "elicitation"
capability, which not every client supports. With the defaults (confirm + deny), a client
without elicitation support will have every write REFUSED. That is deliberate — it fails
closed rather than silently granting unattended write access.

So: if I want writes to work and my client cannot prompt, recommend `mode: full` and make
sure I understand that means no confirmation. If I only want reading, `mode: readonly` is
the safest choice and hides the write tools entirely.

# Client configuration

Use `/absolute/path/to/.env` — clients start the server from their own working directory,
so relative paths break. For a local binary, swap the docker command for the binary path
and its flags.

## Claude Code
  claude mcp add arr -- docker run -i --rm \
    --env-file /absolute/path/to/.env \
    ghcr.io/gauranshmathur/arr-mcp --transport stdio

## Claude Desktop
File: macOS ~/Library/Application Support/Claude/claude_desktop_config.json
      Windows %APPDATA%\Claude\claude_desktop_config.json
      Linux ~/.config/Claude/claude_desktop_config.json
  {
    "mcpServers": {
      "arr": {
        "command": "docker",
        "args": ["run", "-i", "--rm",
                 "--env-file", "/absolute/path/to/.env",
                 "ghcr.io/gauranshmathur/arr-mcp",
                 "--transport", "stdio"]
      }
    }
  }

## Cursor
File: .cursor/mcp.json (project) or ~/.cursor/mcp.json (global)
Same JSON shape as Claude Desktop, top-level key "mcpServers".

## VS Code
File: .vscode/mcp.json (workspace), or Command Palette -> MCP: Open User Configuration
The top-level key is "servers", NOT "mcpServers", and "type" is required:
  {
    "servers": {
      "arr": {
        "type": "stdio",
        "command": "docker",
        "args": ["run", "-i", "--rm",
                 "--env-file", "/absolute/path/to/.env",
                 "ghcr.io/gauranshmathur/arr-mcp",
                 "--transport", "stdio"]
      }
    }
  }

## Other clients — key names differ, do not assume
  Windsurf     ~/.codeium/windsurf/mcp_config.json, "mcpServers"; remote uses "serverUrl"
  Zed          ~/.config/zed/settings.json, top-level key "context_servers"
  Cline        "mcpServers"; remote transport key is "streamableHttp" (camelCase)
  Roo Code     "mcpServers"; remote transport key is "streamable-http" (hyphenated)
  Continue.dev YAML config, servers listed under mcpServers, secrets as ${{ secrets.NAME }}
  Goose        ~/.config/goose/config.yaml; uses "uri" not "url", type "streamable_http"
  LibreChat    prefer HTTP: a stdio entry would spawn the process inside LibreChat's own
               container, where it cannot reach my services

If my client is not listed, point me at:
https://github.com/GauranshMathur/ARR_MCP/blob/main/docs/clients.md

## HTTP transport instead of stdio
Run the server yourself:
  docker run -d --name arr-mcp -p 8080:8080 --env-file /absolute/path/to/.env \
    ghcr.io/gauranshmathur/arr-mcp --transport http --addr 0.0.0.0:8080
Then point the client at http://localhost:8080/mcp

# Verification — always give me these

1. Credentials and reachability:
     arr-mcp --config config.yaml --check
   or with Docker:
     docker run --rm --env-file /absolute/path/to/.env \
       ghcr.io/gauranshmathur/arr-mcp --check
   Expect one line per instance:
     OK    sonarr/hd (http://192.168.1.10:8989)
     FAIL  radarr/main (http://192.168.1.11:7878): radarr returned 401:

2. HTTP only:
     curl -s localhost:8080/health          -> {"status":"ok"}

3. In the client: restart it, then confirm tools named sonarr_*, radarr_*, prowlarr_* or
   bazarr_* appear. Ask me to try "list my Sonarr series".

# Troubleshooting

401 Unauthorized
  Wrong API key, or the key belongs to a different service. Re-copy it from that app's
  Settings -> General -> Security.

connection refused / no route to host
  Wrong URL or port. If ARR-MCP runs in Docker, "localhost" means the CONTAINER, not my
  machine — use the host's LAN IP, or the service name if they share a Docker network.
  This is the single most common mistake.

No tools appear at all
  A service only registers tools when BOTH its _URL and _API_KEY are set. Run --check.
  Also confirm the client actually restarted.

Only read tools appear
  permissions.mode is readonly.

Writes are refused
  confirm mode with a client that cannot prompt, and fallback: deny. See the permissions
  section above. Choose mode: full deliberately, or use a client that supports elicitation.

The server seems to hang on stdio
  Normal. Under stdio the server waits for a client on stdin; it is not meant to be run
  interactively in a terminal.

Nothing works and I want detail
  Add --log-level debug. Logs go to stderr, never stdout, because stdout carries the
  JSON-RPC stream.
~~~~

## After it works

- Full per-client reference: [docs/clients.md](clients.md)
- Kubernetes manifests: [deploy/kubernetes/](../deploy/kubernetes/)
- Configuration reference and tool list: [README](../README.md)
