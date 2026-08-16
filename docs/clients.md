# Client setup

> Want an assistant to walk you through this instead? Paste
> [ai-install-prompt.md](ai-install-prompt.md) into Claude, ChatGPT or similar.

Configuration for connecting ARR-MCP to specific MCP clients. If yours is not listed,
read [Any MCP client](#any-mcp-client) — it describes the two forms every entry here is a
dialect of.

Start from the [60-second quickstart](../README.md#60-second-quickstart) to get a working
`.env` first. Every example below assumes you have one.

**Clients:** [Claude Code](#claude-code) · [Claude Desktop](#claude-desktop) ·
[Cursor](#cursor) · [VS Code](#vs-code) · [Windsurf](#windsurf) · [Zed](#zed) ·
[Cline](#cline) · [Roo Code](#roo-code) · [Continue.dev](#continuedev) ·
[LibreChat](#librechat) · [Goose](#goose) · [Any MCP client](#any-mcp-client)

## Before you start

Two decisions apply to every client.

**Docker or a local binary?** The Docker form needs nothing installed and is the one to
copy if you are unsure. The binary form starts faster and is easier to point at a config
file, at the cost of building or installing it first:

```bash
git clone https://github.com/GauranshMathur/ARR_MCP.git
cd ARR_MCP
go build -o arr-mcp ./cmd/arr-mcp
```

`go install github.com/GauranshMathur/ARR_MCP/cmd/arr-mcp@latest` also works, and installs
it as `$(go env GOPATH)/bin/arr-mcp`.

**stdio or HTTP?** stdio means the client starts and owns the process; it is the default
for desktop and editor clients, and needs no port. HTTP means ARR-MCP runs somewhere as a
service and clients connect to `http://<host>:8080/mcp`; use it when several clients or
several machines share one server, or when the client only supports remote servers.

Wherever a stdio example passes `--env-file`, the path must be **absolute**. Clients spawn
the server from their own working directory, not yours.

## Claude Code

The CLI writes the configuration for you. `--` separates Claude Code's own flags from the
command that runs the server; everything after it is passed through untouched.

**Docker, stdio:**

```bash
claude mcp add arr -- docker run -i --rm \
  --env-file /absolute/path/to/.env \
  ghcr.io/gauranshmathur/arr-mcp --transport stdio
```

**Local binary, stdio, with a config file:**

```bash
claude mcp add arr -- /absolute/path/to/arr-mcp \
  --transport stdio --config /absolute/path/to/config.yaml
```

**Local binary, stdio, passing credentials as flags** instead of a `.env` file. Note the
`--transport stdio` between `--env` and the server name: if the name comes directly after
`--env`, the CLI reads it as another `KEY=value` pair and rejects it.

```bash
claude mcp add \
  --env SONARR_URL=http://192.168.10.12:8989 \
  --env SONARR_API_KEY=your-key \
  --transport stdio arr \
  -- /absolute/path/to/arr-mcp
```

**HTTP:**

```bash
claude mcp add --transport http arr http://localhost:8080/mcp
```

Add `--scope user` to make the server available in every project instead of just the
current one, or `--scope project` to write it to a checked-in `.mcp.json`. Verify with
`claude mcp list`, which prints a health status per server, and use `/mcp` inside a session
to inspect the tools.

Claude Code supports MCP elicitation, so the default `confirm` permission mode prompts
correctly here.

## Claude Desktop

Edit `claude_desktop_config.json`. Settings → Developer → **Edit Config** opens the right
file; otherwise:

| OS | Path |
|---|---|
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Windows | `%APPDATA%\Claude\claude_desktop_config.json` |
| Linux (beta) | `~/.config/Claude/claude_desktop_config.json` |

**Docker:**

```json
{
  "mcpServers": {
    "arr": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "--env-file", "/absolute/path/to/.env",
        "ghcr.io/gauranshmathur/arr-mcp",
        "--transport", "stdio"
      ]
    }
  }
}
```

**Local binary**, with credentials inline rather than in a `.env` file:

```json
{
  "mcpServers": {
    "arr": {
      "command": "/absolute/path/to/arr-mcp",
      "args": ["--transport", "stdio"],
      "env": {
        "SONARR_URL": "http://192.168.10.12:8989",
        "SONARR_API_KEY": "your-sonarr-api-key",
        "RADARR_URL": "http://192.168.10.14:7878",
        "RADARR_API_KEY": "your-radarr-api-key"
      }
    }
  }
}
```

Restart Claude Desktop completely after editing — it reads the file only at launch. Paths
must be absolute; the app does not inherit your shell's `PATH` or environment, so a bare
`arr-mcp` or a `docker` that lives outside `/usr/local/bin` may not resolve. Server logs
land in `~/Library/Logs/Claude/mcp-server-arr.log` (macOS) or `%APPDATA%\Claude\logs\`
(Windows).

Claude Desktop connects to stdio servers only. To use an HTTP deployment, add it as a
custom connector from Settings → Connectors instead.

## Cursor

Write `.cursor/mcp.json` in the project, or `~/.cursor/mcp.json` to make it global.

**Docker, stdio:**

```json
{
  "mcpServers": {
    "arr": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "--env-file", "/absolute/path/to/.env",
        "ghcr.io/gauranshmathur/arr-mcp",
        "--transport", "stdio"
      ]
    }
  }
}
```

**Local binary, stdio:**

```json
{
  "mcpServers": {
    "arr": {
      "command": "/absolute/path/to/arr-mcp",
      "args": ["--transport", "stdio", "--config", "/absolute/path/to/config.yaml"],
      "env": {
        "SONARR_MAIN_API_KEY": "your-sonarr-api-key"
      }
    }
  }
}
```

**HTTP** — a remote entry is one with a `url` and no `command`:

```json
{
  "mcpServers": {
    "arr": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

Check the result under Settings → Cursor Settings → MCP, which lists each server with its
tools. MCP tools are used in Agent mode.

## VS Code

VS Code has native MCP support used by Copilot's agent mode. Its schema differs from most
other clients in two ways worth noting before you copy anything: the top-level key is
**`servers`**, not `mcpServers`, and the transport is named explicitly with `type`.

Put this in `.vscode/mcp.json` for one workspace, or run **MCP: Open User Configuration**
from the Command Palette to edit the profile-wide file.

**Docker, stdio:**

```json
{
  "servers": {
    "arr": {
      "type": "stdio",
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "--env-file", "/absolute/path/to/.env",
        "ghcr.io/gauranshmathur/arr-mcp",
        "--transport", "stdio"
      ]
    }
  }
}
```

**Local binary, stdio, prompting for the API key** instead of committing it. VS Code asks
once on first use and stores the answer in secret storage:

```json
{
  "inputs": [
    {
      "type": "promptString",
      "id": "sonarr-api-key",
      "description": "Sonarr API key",
      "password": true
    }
  ],
  "servers": {
    "arr": {
      "type": "stdio",
      "command": "/absolute/path/to/arr-mcp",
      "args": ["--transport", "stdio"],
      "env": {
        "SONARR_URL": "http://192.168.10.12:8989",
        "SONARR_API_KEY": "${input:sonarr-api-key}"
      }
    }
  }
}
```

An `envFile` field is also supported if you would rather point at the `.env` you already
have: `"envFile": "/absolute/path/to/.env"`.

**HTTP:**

```json
{
  "servers": {
    "arr": {
      "type": "http",
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

There is a CLI form too:

```bash
code --add-mcp '{"name":"arr","type":"http","url":"http://localhost:8080/mcp"}'
```

Open the Chat view, switch to **Agent** mode and use the tools picker to confirm the ARR
tools are listed. `.vscode/mcp.json` is checked in and shared with the repository, so keep
secrets in `inputs` or `envFile` rather than literals.

## Windsurf

Edit `~/.codeium/windsurf/mcp_config.json`, or open Windsurf Settings → Cascade → **Manage
MCP servers** → View raw config.

**Docker, stdio:**

```json
{
  "mcpServers": {
    "arr": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "--env-file", "/absolute/path/to/.env",
        "ghcr.io/gauranshmathur/arr-mcp",
        "--transport", "stdio"
      ]
    }
  }
}
```

**Local binary, stdio:**

```json
{
  "mcpServers": {
    "arr": {
      "command": "/absolute/path/to/arr-mcp",
      "args": ["--transport", "stdio"],
      "env": {
        "SONARR_URL": "http://192.168.10.12:8989",
        "SONARR_API_KEY": "your-sonarr-api-key"
      }
    }
  }
}
```

**HTTP** — Windsurf names this field `serverUrl`, not `url`:

```json
{
  "mcpServers": {
    "arr": {
      "serverUrl": "http://localhost:8080/mcp"
    }
  }
}
```

Press **Refresh** in the MCP panel after editing.

## Zed

Zed calls MCP servers *context servers*. Add one from the Agent Panel (Settings → AI → MCP
Servers → **Add Server**), or edit `settings.json` directly — `zed: open settings` from the
command palette, which is `~/.config/zed/settings.json` on macOS and Linux.

**Docker, stdio:**

```json
{
  "context_servers": {
    "arr": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "--env-file", "/absolute/path/to/.env",
        "ghcr.io/gauranshmathur/arr-mcp",
        "--transport", "stdio"
      ],
      "env": {}
    }
  }
}
```

**Local binary, stdio:**

```json
{
  "context_servers": {
    "arr": {
      "command": "/absolute/path/to/arr-mcp",
      "args": ["--transport", "stdio"],
      "env": {
        "SONARR_URL": "http://192.168.10.12:8989",
        "SONARR_API_KEY": "your-sonarr-api-key"
      }
    }
  }
}
```

**HTTP:**

```json
{
  "context_servers": {
    "arr": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

## Cline

Open the Cline panel → **MCP Servers** icon → Installed → **Configure MCP Servers**, which
opens `cline_mcp_settings.json`. Letting the panel open it is easier than finding it:

| OS | Path |
|---|---|
| macOS | `~/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json` |
| Windows | `%APPDATA%\Code\User\globalStorage\saoudrizwan.claude-dev\settings\cline_mcp_settings.json` |
| Linux | `~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json` |

The Cline CLI uses `~/.cline/mcp.json` with the same schema.

**Docker, stdio:**

```json
{
  "mcpServers": {
    "arr": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "--env-file", "/absolute/path/to/.env",
        "ghcr.io/gauranshmathur/arr-mcp",
        "--transport", "stdio"
      ],
      "disabled": false,
      "autoApprove": []
    }
  }
}
```

**HTTP:**

```json
{
  "mcpServers": {
    "arr": {
      "type": "streamableHttp",
      "url": "http://localhost:8080/mcp",
      "disabled": false,
      "autoApprove": []
    }
  }
}
```

`autoApprove` lists tools Cline may call without asking. It is Cline's own approval layer
and is independent of ARR-MCP's permission mode — a tool listed there still has to pass
the server's [permission gate](../README.md#permissions). Leaving it as `[]` and letting
ARR-MCP do the gating keeps one policy instead of two, and read tools are never gated by
either.

## Roo Code

Global settings live in `mcp_settings.json` (Roo Code panel → MCP → **Edit Global MCP**);
per-project settings go in `.roo/mcp.json`, which wins when a server name appears in both.

**Docker, stdio:**

```json
{
  "mcpServers": {
    "arr": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "--env-file", "/absolute/path/to/.env",
        "ghcr.io/gauranshmathur/arr-mcp",
        "--transport", "stdio"
      ],
      "alwaysAllow": [],
      "disabled": false
    }
  }
}
```

**HTTP** — note the hyphen: Roo Code spells this `streamable-http`, where Cline spells it
`streamableHttp`.

```json
{
  "mcpServers": {
    "arr": {
      "type": "streamable-http",
      "url": "http://localhost:8080/mcp",
      "alwaysAllow": [],
      "disabled": false
    }
  }
}
```

## Continue.dev

Continue takes YAML. Add an `mcpServers` block to your assistant's `config.yaml`, or drop
a file in `.continue/mcpServers/` in the project. MCP tools are available in **Agent**
mode only.

**Docker, stdio:**

```yaml
mcpServers:
  - name: arr
    command: docker
    args:
      - run
      - -i
      - --rm
      - --env-file
      - /absolute/path/to/.env
      - ghcr.io/gauranshmathur/arr-mcp
      - --transport
      - stdio
```

**Local binary, stdio, with secrets** pulled from Continue's secret store:

```yaml
mcpServers:
  - name: arr
    command: /absolute/path/to/arr-mcp
    args:
      - --transport
      - stdio
    env:
      SONARR_URL: http://192.168.10.12:8989
      SONARR_API_KEY: ${{ secrets.SONARR_API_KEY }}
```

**HTTP:**

```yaml
mcpServers:
  - name: arr
    type: streamable-http
    url: http://localhost:8080/mcp
```

## LibreChat

Add an `mcpServers` block to `librechat.yaml` and restart the API service.

LibreChat usually runs in a container, which makes HTTP the natural choice: a stdio entry
would have to spawn a process *inside the LibreChat container*, where neither the binary
nor a Docker socket exists. Run ARR-MCP as its own service on the same Docker network and
address it by service name.

**HTTP:**

```yaml
mcpServers:
  arr:
    type: streamable-http
    url: http://arr-mcp:8080/mcp
    timeout: 30000
```

Use `http://host.docker.internal:8080/mcp` if ARR-MCP runs on the host rather than on
LibreChat's network, and `http://localhost:8080/mcp` only if you are running LibreChat
outside a container.

**stdio**, for a bare-metal LibreChat installation:

```yaml
mcpServers:
  arr:
    command: /absolute/path/to/arr-mcp
    args:
      - --transport
      - stdio
    env:
      SONARR_URL: http://192.168.10.12:8989
      SONARR_API_KEY: ${SONARR_API_KEY}
```

`${VAR}` in `librechat.yaml` is resolved from LibreChat's own environment, so the key still
lives in its `.env` rather than in the YAML.

## Goose

Goose calls MCP servers *extensions*. The interactive route:

```bash
goose configure
# → Add Extension → Command-line Extension
#   Name:    arr
#   Command: docker run -i --rm --env-file /absolute/path/to/.env ghcr.io/gauranshmathur/arr-mcp --transport stdio
#   Timeout: 300
```

Or edit `~/.config/goose/config.yaml` directly
(`%APPDATA%\Block\goose\config\config.yaml` on Windows):

```yaml
extensions:
  arr:
    type: stdio
    name: arr
    enabled: true
    cmd: docker
    args:
      - run
      - -i
      - --rm
      - --env-file
      - /absolute/path/to/.env
      - ghcr.io/gauranshmathur/arr-mcp
      - --transport
      - stdio
    envs: {}
    env_keys: []
    timeout: 300
```

**Local binary** — the same block with `cmd: /absolute/path/to/arr-mcp`, args
`[--transport, stdio]`, and credentials under `envs`:

```yaml
    envs:
      SONARR_URL: http://192.168.10.12:8989
      SONARR_API_KEY: your-sonarr-api-key
```

**HTTP** — Goose names this type `streamable_http` with an underscore, and the field is
`uri`, not `url`:

```yaml
extensions:
  arr:
    type: streamable_http
    name: arr
    enabled: true
    uri: http://localhost:8080/mcp
    headers: {}
    envs: {}
    env_keys: []
    timeout: 300
```

## Any MCP client

Every entry above is a dialect of one of two things.

### stdio

The client spawns a process and speaks JSON-RPC 2.0 over its stdin and stdout. The command
is:

```
docker run -i --rm --env-file /absolute/path/to/.env ghcr.io/gauranshmathur/arr-mcp --transport stdio
```

or, with a local binary:

```
/absolute/path/to/arr-mcp --transport stdio --config /absolute/path/to/config.yaml
```

Three things that break stdio setups:

- **Missing `-i` on `docker run`.** Without it the container gets no stdin, and the
  initialize handshake never completes.
- **Relative paths.** The client's working directory is not yours.
- **Assuming your shell environment is inherited.** GUI clients start with a minimal
  environment; pass credentials through the client's `env` block or `--env-file`.

Nothing but the protocol may be written to stdout. ARR-MCP logs to stderr for exactly this
reason, and your client will normally capture that stream into a log file.

### HTTP

Run the server once and point clients at it:

```bash
docker run -d --name arr-mcp --env-file .env -p 8080:8080 ghcr.io/gauranshmathur/arr-mcp
```

| Endpoint | Purpose |
|---|---|
| `http://<host>:8080/mcp` | MCP over Streamable HTTP. This is the URL a client wants. |
| `http://<host>:8080/health` | `{"status":"ok"}`. Not part of MCP; for container and Kubernetes probes. |

Some clients label this transport `http`, others `streamable-http`, `streamableHttp` or
`streamable_http`. They are the same thing — the MCP specification's name is
Streamable HTTP. A client that offers only the deprecated SSE transport cannot connect;
use stdio there instead.

There is no authentication in front of `/mcp`. Anyone who can reach the port can drive
every configured \*arr instance, subject only to the
[permission mode](../README.md#permissions). Keep it on a trusted network, or put
authentication in front of it with a reverse proxy.

### Verifying a connection by hand

The MCP inspector lists the tool surface without involving any client:

```bash
npx @modelcontextprotocol/inspector docker run -i --rm \
  --env-file /absolute/path/to/.env ghcr.io/gauranshmathur/arr-mcp --transport stdio
```

For an HTTP deployment, `curl -s localhost:8080/health` confirms the server is up, and
pointing the inspector at `http://localhost:8080/mcp` confirms MCP itself is answering.
