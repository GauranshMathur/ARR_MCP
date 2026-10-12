# Configuration

## Find your API key

Each service shows its key in its own web UI. The key is a long hex string; copy it
exactly, with no surrounding whitespace.

| Service | Where to click | Default port |
|---|---|---|
| Sonarr | Settings → General → **Security** → API Key | 8989 |
| Radarr | Settings → General → **Security** → API Key | 7878 |
| Prowlarr | Settings → General → **Security** → API Key | 9696 |
| Bazarr | Settings → General → **Security** → API Key | 6767 |
| Seerr | Settings → General → API Key | 5055 |

The download clients have no API key; they take the same username and password you log
into their web UI with:

| Service | Where to click | Default port |
|---|---|---|
| qBittorrent | Tools → Options → **Web UI** → Authentication | 8080 |
| NZBGet | Settings → **Security** → ControlUsername / ControlPassword | 6789 |

Maintainerr (default port 6246) has no API authentication at all, so it takes only a
`url`. Anyone who can reach its port has full control of it, so keep it off untrusted
networks.

Jellyfin (default port 8096) takes an API key: Dashboard → **API Keys** → add one. It is
sent as `Authorization: MediaBrowser Token="<key>"`, the form Jellyfin 12 requires; the
older `X-Emby-Token` header and `api_key` query parameter are rejected.

Jellystat (default port 3000) takes an API key: Settings → **API Keys** → add one. It is
sent as the `X-Api-Token` header.

Synology DSM (default port 5000 for HTTP, 5001 for HTTPS) takes the username and password of
a DSM account. **That account has to be a NAS administrator** (DSM serves storage health only
to administrators), so its password is full control of the NAS: treat it that way. It must
not use 2-factor authentication, because there is nobody to type a code. See
[Synology](tools/synology.md) before pointing ARR-MCP at a NAS.

Every one of these is a *full-access* credential for that application. ARR-MCP never
needs more than one credential per instance, and the [permission model](permissions.md) is
what narrows down what the model can actually do with it.

## Do I need a config file?

No, unless you run more than one instance of a service.

| You have | Use | Why |
|---|---|---|
| One Sonarr, one Radarr, one Prowlarr, one Bazarr | **Environment variables** | Nothing to disambiguate. `<SERVICE>_URL` and `<SERVICE>_API_KEY` is the whole configuration. |
| Two Sonarrs (say `main` and `anime`), or a Bazarr per \*arr pair | **`config.yaml`** | Instances need names so tools can target them, and each can carry its own permission policy. |
| One instance, but you want per-instance permissions or non-default server settings | **`config.yaml`** | The environment-variable path always builds a single instance named `default` with the global policy. |

You can switch later without changing anything else: a config file supersedes the
environment variables entirely, it does not merge with them.

## Environment variables (one instance per service)

Set `<SERVICE>_URL` and `<SERVICE>_API_KEY` — or, for `QBITTORRENT`, `NZBGET` and `SYNOLOGY`,
`<SERVICE>_USERNAME` and `<SERVICE>_PASSWORD`; `MAINTAINERR` needs only its URL — and
run **without** `--config`:

```bash
SONARR_URL=http://192.168.10.12:8989
SONARR_API_KEY=...
RADARR_URL=http://192.168.10.14:7878
RADARR_API_KEY=...
PROWLARR_URL=http://192.168.10.18:9696
PROWLARR_API_KEY=...
BAZARR_URL=http://192.168.10.16:6767
BAZARR_API_KEY=...
QBITTORRENT_URL=http://192.168.10.20:8080
QBITTORRENT_USERNAME=admin
QBITTORRENT_PASSWORD=...
NZBGET_URL=http://192.168.10.21:6789
NZBGET_USERNAME=nzbget
NZBGET_PASSWORD=...
MAINTAINERR_URL=http://192.168.10.25:6246
SEERR_URL=http://192.168.10.26:5055
SEERR_API_KEY=...
JELLYFIN_URL=http://192.168.10.27:8096
JELLYFIN_API_KEY=...
JELLYSTAT_URL=http://192.168.10.28:3000
JELLYSTAT_API_KEY=...
SYNOLOGY_URL=http://192.168.10.30:5000
SYNOLOGY_USERNAME=...
SYNOLOGY_PASSWORD=...
```

A service is configured only when **all** its variables are set; setting just some of
them is treated as "not configured" rather than as an error. Each configured service gets a
single instance named `default`. If no service ends up configured at all, startup fails
with a message listing the variables it looked for.

## Config file (multiple instances)

Copy `config.example.yaml` to `config.yaml`. Secrets stay in the environment and are
referenced as `${VAR}`, so the file is safe to commit or mount from a ConfigMap:

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

Every tool then takes an optional `instance` argument. Omit it to use the one marked
`default`. The configured names are advertised as a schema `enum`, so the model picks
from a closed set rather than guessing:

```
sonarr_search_series{query: "Severance", instance: "anime"}
```

Rules the loader enforces at startup, so a mistake never surfaces mid-conversation:

- Every instance needs a `name`, a `url` and its credential: an `apiKey` for the \*arr
  services, Bazarr, Seerr, Jellyfin and Jellystat, a `username` and `password` for `qbittorrent`, `nzbget` and `synology`, and
  nothing for `maintainerr`, which has no authentication. Supplying the wrong kind is an
  error, not a silent fallback.
- Instance names must be unique within a service, and at most one may be `default`.
- With several instances and no `default`, a tool call that omits `instance` fails with a
  message listing the valid names — it does not silently pick the first one.
- Only `sonarr`, `radarr`, `prowlarr`, `bazarr`, `qbittorrent`, `nzbget`, `maintainerr`,
  `seerr`, `jellyfin`, `jellystat` and `synology` are accepted;
  anything else is rejected rather than ignored, so a typo like `sonar:` is caught
  immediately.

> An unset (or empty) `${VAR}` is a startup error, never a silent empty value — an empty
> API key would otherwise surface much later as a confusing 401.

## Server settings

```yaml
server:
  transport: stdio        # stdio | http
  addr: 0.0.0.0:8080      # only used by the http transport
  logLevel: info          # debug | info | warn | error
```

Those are the defaults. Command-line flags override the file, so one mounted config can
serve both a stdio and an HTTP deployment:

```bash
arr-mcp --config /etc/arr-mcp/config.yaml --transport http --addr 0.0.0.0:8080
```

Instead of passing `--config` you can set `ARR_MCP_CONFIG` to the same path. That is worth
doing in containers: it means bare `arr-mcp --check` also finds the config, which is what
makes the Docker healthcheck work.

## CLI

```
--config PATH        path to config.yaml (or set ARR_MCP_CONFIG)
--transport stdio    stdio or http
--addr HOST:PORT     listen address for http
--log-level LEVEL    debug, info, warn, error
--check              test connectivity to every configured instance and exit
--version            print version
```

Flags override the config file, and every flag except `--config` has a config-file
equivalent under `server:`. `--check` exits non-zero if any instance fails, so it works in
a healthcheck or a CI step as well as by hand.
