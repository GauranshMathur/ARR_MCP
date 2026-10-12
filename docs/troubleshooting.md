# Troubleshooting

Start with `--check`. It exercises exactly the credentials and URLs the tools will use, and
prints one line per instance, so it separates "my configuration is wrong" from "my client
is wrong" in a single command:

```bash
docker run --rm --env-file .env ghcr.io/gauranshmathur/arr-mcp --check
# or, from a local binary
arr-mcp --config config.yaml --check
```

## `401 Unauthorized`

```
FAIL  radarr/main (http://192.168.10.14:7878): radarr returned 401: Unauthorized
```

The URL is right — something answered — but the API key is not. Re-copy it from
[Settings → General → Security](configuration.md#find-your-api-key); a trailing space or a truncated paste
is the usual cause. Check you did not swap keys between two instances of the same service,
which produces exactly this error on both.

## `connection refused` or a timeout

Nothing is listening at that address. In order of likelihood:

1. **Wrong port.** Sonarr 8989, Radarr 7878, Prowlarr 9696, Bazarr 6767 by default.
2. **`localhost` used from inside a container.** This is by far the most common mistake.
   Inside a container, `localhost` means *that container*, not your machine — so
   `SONARR_URL=http://localhost:8989` tells ARR-MCP to look for Sonarr inside its own
   otherwise-empty container, and it finds nothing. Use the LAN IP of the host
   (`http://192.168.10.12:8989`), or the other container's service name if they share a
   Docker network (`http://sonarr:8989`), or `http://host.docker.internal:8989` on Docker
   Desktop. The same reasoning applies in Kubernetes: use the Service DNS name
   (`http://sonarr.media.svc.cluster.local:8989`), never `localhost`.
3. **A URL base path was dropped.** If you reach Sonarr at `/sonarr` behind a reverse
   proxy, that prefix belongs in the URL: `http://192.168.10.12/sonarr`.
4. **`https://` with a self-signed certificate.** The certificate must be trusted;
   plain `http://` on the LAN avoids the problem entirely.

## The client connects but shows no tools

Tools are registered per service, and a service with no configured instances registers
nothing. An empty tool list therefore means nothing was configured, not that registration
failed.

- Check the startup log on **stderr** — it prints one `sonarr: 1 instance(s) configured
  [[default]]` line per service. No lines means no services were configured.
- With environment variables, remember that **both** `<SERVICE>_URL` and
  `<SERVICE>_API_KEY` must be set for that service to count.
- Check the client is passing the environment through. A `claude_desktop_config.json`
  entry with no `env` block and no `--env-file` starts the server with an empty
  environment; your shell's exported variables are not inherited.
- If only the *write* tools are missing, that is `permissions.mode: readonly` doing its
  job — read-only mode does not register them at all.

## Writes are refused without ever prompting

```
client does not support elicitation: cannot confirm write tool sonarr_add_series;
set permissions.fallback=allow or permissions.mode=full to permit it
```

The default `confirm` mode asks for approval through MCP elicitation, and your client does
not implement it. There is no prompt to answer, so the call fails closed rather than
running unapproved — see [Permissions](permissions.md) for why that default is the safe one.
Either switch to a client that supports elicitation, or make the decision explicit:

```yaml
permissions:
  mode: confirm
  confirmScope: destructive   # only deletes need confirming
  fallback: allow             # writes proceed unprompted
```

`confirmScope: destructive` is usually the better trade: adds and command triggers run
freely, and the calls that remove things still fail closed.

## The server starts, then exits immediately under stdio

That is normal. A stdio server lives for exactly as long as its client holds the pipe
open; running it by hand in a terminal ends as soon as stdin closes. Test it with
`--check`, or with the MCP inspector (see [Development](development.md)), not by launching
it bare.

## Startup fails with `references unset environment variable(s)`

A `${VAR}` in `config.yaml` has no value in the process environment. Under Docker this
almost always means the variable is in your shell but was never passed into the container
— add it to `--env-file` / the compose `env_file`. Empty counts as unset, on purpose.

## Nothing at all appears in the logs

Logging always goes to **stderr**. Under the stdio transport, stdout carries the JSON-RPC
stream and must not be written to by anything else. Most clients file that stderr away
under their own logs — Claude Desktop, for example, writes
`~/Library/Logs/Claude/mcp-server-arr.log` on macOS. Raise the detail with
`--log-level debug`.
