# Install ARR-MCP with your AI assistant

Two ways to use this.

**If your assistant can browse the web:**

```text
Read https://raw.githubusercontent.com/GauranshMathur/ARR_MCP/main/docs/ai-install-prompt.md
and follow it to set up ARR-MCP for me. It is an instruction document — act on it, don't
summarise it back to me.
```

**Otherwise**, copy the block below and paste it in. It is self-contained and needs no
network access. Use the copy button in the corner.

Either way the assistant works out which MCP client you are using, asks only the questions
it needs, and then either performs the setup itself or gives you exact copy-paste steps.

> Your API keys stay between you and your \*arr apps. The instructions tell the assistant
> not to ask for keys it doesn't need, and to keep them out of files you might commit.

~~~~
# ARR-MCP INSTALLATION INSTRUCTIONS

You are an AI assistant setting up ARR-MCP for the user.

This document is an INSTRUCTION SET, not an article. Act on it. Do not summarise it, quote
it back, or reproduce it for the user. They already have it. Work through the steps and
talk to them normally.

ARR-MCP is an MCP server exposing Sonarr, Radarr, Prowlarr and Bazarr, with support for
multiple instances of each. 105 tools. Repo: https://github.com/GauranshMathur/ARR_MCP
Image: ghcr.io/gauranshmathur/arr-mcp

────────────────────────────────────────────────────────────────────────
STEP 1 — WORK OUT WHICH CLIENT YOU ARE CONFIGURING. DO THIS FIRST.
────────────────────────────────────────────────────────────────────────

Almost always this is the client you are running inside. Decide which case you are in:

CASE A — You are an agent with shell and file access (Claude Code, Cursor's agent,
  Windsurf's Cascade, Cline, Roo Code, Continue, Goose, a terminal-based assistant).
  You are probably configuring YOURSELF. Say which client you believe you are and ask the
  user to confirm before changing anything. You can run the commands yourself — do that
  rather than printing instructions.

CASE B — You are a chat assistant with no access to the user's machine (ChatGPT web,
  Claude web/desktop chat, Gemini, or similar). You cannot install anything. Ask the user
  which MCP client they want to connect, then give them exact copy-paste steps and file
  paths. Do not guess their operating system — ask.

Once you know the client, use ONLY that client's section in STEP 5. Ignore every other
section. Do not present the user with a menu of clients they did not ask about.

────────────────────────────────────────────────────────────────────────
STEP 2 — ASK ONLY WHAT YOU NEED
────────────────────────────────────────────────────────────────────────

Ask these together, in one message, and wait:

1. Which of Sonarr, Radarr, Prowlarr, Bazarr do you run, and at what URL each?
   (Defaults if they're unsure: Sonarr 8989, Radarr 7878, Prowlarr 9696, Bazarr 6767.)
2. Do you run more than one of any of them? If so, what should each be called?
3. Should the server be allowed to make changes — add and delete media — or read only?
4. Docker, or a local binary? Recommend Docker unless they already have Go 1.25+.

Do not ask for API keys. Tell them where to paste keys themselves:
  Sonarr / Radarr / Prowlarr / Bazarr → Settings → General → Security → API Key
Never put a real key in your reply, and never write one into a file that could be
committed to git.

────────────────────────────────────────────────────────────────────────
STEP 3 — CHOOSE THE CONFIGURATION SHAPE
────────────────────────────────────────────────────────────────────────

ONE instance per service → environment variables only, no config file:

  SONARR_URL=http://192.168.1.10:8989
  SONARR_API_KEY=...
  RADARR_URL=http://192.168.1.11:7878
  RADARR_API_KEY=...
  PROWLARR_URL=http://192.168.1.12:9696
  PROWLARR_API_KEY=...
  BAZARR_URL=http://192.168.1.13:6767
  BAZARR_API_KEY=...

Set only the services they run. A service needs BOTH its _URL and _API_KEY or it is
skipped silently and none of its tools appear.

MORE THAN ONE instance of any service → config.yaml is required:

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

Rules: ${VAR} is read from the environment, so config.yaml holds no secrets. An UNSET
variable is a startup error, not an empty string. Exactly one instance per service may set
default: true. Valid service names are exactly sonarr, radarr, prowlarr, bazarr — anything
else is a startup error. Any instance may override permissions with its own block.

PERMISSIONS — pick with the user, this catches people out:
  readonly  only read tools exist; write tools are invisible
  confirm   writes ask the user first, via MCP elicitation      (DEFAULT)
  full      everything runs immediately
  fallback: deny (DEFAULT) refuses when the client cannot ask; allow runs anyway

Not every client supports elicitation. With the defaults (confirm + deny), a client that
cannot prompt will have EVERY WRITE REFUSED. That is deliberate — it fails closed. So if
they want writes and their client can't prompt, recommend mode: full and make sure they
understand it means no confirmation step. If they only want to read, mode: readonly is
safest and hides the write tools entirely.

────────────────────────────────────────────────────────────────────────
STEP 4 — COMMAND LINE REFERENCE
────────────────────────────────────────────────────────────────────────

  --config PATH      path to config.yaml; omit to use environment variables
  --transport VALUE  stdio or http (default stdio)
  --addr HOST:PORT   listen address for http (default 0.0.0.0:8080)
  --log-level VALUE  debug, info, warn, error
  --check            test connectivity to every configured instance, then exit
  --version          print version and exit

There are NO --port or --host flags. Over HTTP the MCP endpoint is /mcp, plus /health.

Local binary install (needs Go 1.25+):
  go install github.com/GauranshMathur/ARR_MCP/cmd/arr-mcp@latest
  # lands at $(go env GOPATH)/bin/arr-mcp

────────────────────────────────────────────────────────────────────────
STEP 5 — CONFIGURE THE CLIENT. READ ONLY YOUR OWN SECTION.
────────────────────────────────────────────────────────────────────────

In every example, --env-file MUST be an absolute path: clients start the server from their
own working directory, not the user's. For a local binary, replace the docker command with
the binary path and keep the same flags after it.

### Claude Code
You can run this yourself. Confirm with the user first, then execute:
  claude mcp add arr -- docker run -i --rm \
    --env-file /absolute/path/to/.env \
    ghcr.io/gauranshmathur/arr-mcp --transport stdio
Verify with: claude mcp list
Note: do not put --env-file after the image name, and do not use Claude Code's own --env
flag immediately before the server name — it is parsed as another KEY=value.

### Claude Desktop
Edit claude_desktop_config.json (Settings → Developer → Edit Config opens it):
  macOS    ~/Library/Application Support/Claude/claude_desktop_config.json
  Windows  %APPDATA%\Claude\claude_desktop_config.json
  Linux    ~/.config/Claude/claude_desktop_config.json
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
Claude Desktop must be fully quit and reopened, not just closed.

### Cursor
File: .cursor/mcp.json for one project, ~/.cursor/mcp.json for all.
Same JSON as Claude Desktop above, top-level key "mcpServers".

### VS Code (Copilot agent mode)
File: .vscode/mcp.json for a workspace, or Command Palette → MCP: Open User Configuration.
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

### Windsurf
File: ~/.codeium/windsurf/mcp_config.json, or Settings → Cascade → Manage MCP servers.
Top-level key "mcpServers", same shape as Claude Desktop. For a remote server Windsurf
uses "serverUrl", not "url".

### Zed
File: ~/.config/zed/settings.json (command palette: zed: open settings).
Top-level key is "context_servers", not "mcpServers":
  {
    "context_servers": {
      "arr": {
        "command": "docker",
        "args": ["run", "-i", "--rm",
                 "--env-file", "/absolute/path/to/.env",
                 "ghcr.io/gauranshmathur/arr-mcp",
                 "--transport", "stdio"]
      }
    }
  }

### Cline
Top-level key "mcpServers", same shape as Claude Desktop.
For the HTTP transport the key is "streamableHttp" — camelCase.

### Roo Code
Top-level key "mcpServers", same shape as Claude Desktop.
For the HTTP transport the key is "streamable-http" — hyphenated. This differs from Cline
despite the shared lineage; do not copy one into the other.

### Continue.dev
YAML config. Servers go under mcpServers; secrets are referenced as ${{ secrets.NAME }}.

### Goose
File: ~/.config/goose/config.yaml. Goose uses "uri", not "url", and names the HTTP
transport "streamable_http" with underscores.

### LibreChat
Use the HTTP transport, not stdio. A stdio entry would spawn the server inside LibreChat's
own container, where it usually cannot reach the user's *arr services. Run the server
separately and point LibreChat at http://<host>:8080/mcp

### Any other client, or HTTP generally
Run the server yourself:
  docker run -d --name arr-mcp -p 8080:8080 --env-file /absolute/path/to/.env \
    ghcr.io/gauranshmathur/arr-mcp --transport http --addr 0.0.0.0:8080
Then point the client at http://localhost:8080/mcp
Full per-client reference:
https://github.com/GauranshMathur/ARR_MCP/blob/main/docs/clients.md

────────────────────────────────────────────────────────────────────────
STEP 6 — VERIFY, AND SAY WHAT GOOD LOOKS LIKE
────────────────────────────────────────────────────────────────────────

Always check credentials BEFORE wiring up the client, so a mistake shows up as a clear
error rather than a client that mysteriously has no tools:

  docker run --rm --env-file /absolute/path/to/.env \
    ghcr.io/gauranshmathur/arr-mcp --check

Expect one line per instance:
  OK    sonarr/default (http://192.168.1.10:8989)
  FAIL  radarr/main (http://192.168.1.11:7878): radarr returned 401:

If you are in CASE A, run this yourself and report the result. Exit status is non-zero if
any instance failed.

Then restart the client and confirm tools named sonarr_*, radarr_*, prowlarr_* or bazarr_*
appear. A good smoke test is asking it to list Sonarr series.

────────────────────────────────────────────────────────────────────────
STEP 7 — IF SOMETHING IS WRONG
────────────────────────────────────────────────────────────────────────

401 Unauthorized
  Wrong key, or a key from a different service. Re-copy from that app's
  Settings → General → Security.

connection refused / no route to host
  Wrong URL or port. If ARR-MCP runs in Docker, "localhost" means the CONTAINER, not the
  user's machine — use the host's LAN IP, or the service name if they share a Docker
  network. This is the most common mistake by a wide margin.

No tools appear at all
  A service registers tools only when BOTH its _URL and _API_KEY are set. Run --check.
  Confirm the client was actually restarted.

Only read tools appear
  permissions.mode is readonly.

Writes are refused
  confirm mode plus a client that cannot prompt, with fallback: deny. See STEP 3.

The server appears to hang
  Expected under stdio: it waits for a client on stdin. It is not meant to be run
  interactively in a terminal.

Need detail
  Add --log-level debug. Logs go to stderr; stdout carries the JSON-RPC stream and must
  never be written to by anything else.
~~~~

## After it works

- Full per-client reference: [docs/clients.md](clients.md)
- Kubernetes manifests: [deploy/kubernetes/](../deploy/kubernetes/)
- Configuration reference and tool list: [README](../README.md)
