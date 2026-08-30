# ARR-MCP

An [MCP](https://modelcontextprotocol.io) server for the \*arr media stack. Connect Claude, Cursor, VS Code or any other MCP client to Sonarr, Radarr, Prowlarr, Bazarr, qBittorrent and NZBGet — including **multiple instances of each**.

- **Real MCP** — JSON-RPC 2.0 over stdio and Streamable HTTP, built on the official Go SDK
- **Multi-instance** — run two Sonarrs (4K and 1080p) and address them by name
- **Permission controls** — read-only, confirm-before-write, or full access
- **215 tools** across Sonarr, Radarr, Prowlarr, Bazarr, qBittorrent and NZBGet — what you would otherwise do by clicking through each web UI
- **Single static binary**, distroless container, multi-arch image

**Jump to:** [Install with your AI](#install-with-your-ai) · [Manual quickstart](#60-second-quickstart) · [Find your API key](#find-your-api-key) · [Configuration](#configuration) · [Client setup](docs/clients.md) · [Permissions](#permissions) · [Tools](#tools) · [Troubleshooting](#troubleshooting)

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
[Find your API key](#find-your-api-key) if you don't know where they live.

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

## Find your API key

Each service shows its key in its own web UI. The key is a long hex string; copy it
exactly, with no surrounding whitespace.

| Service | Where to click | Default port |
|---|---|---|
| Sonarr | Settings → General → **Security** → API Key | 8989 |
| Radarr | Settings → General → **Security** → API Key | 7878 |
| Prowlarr | Settings → General → **Security** → API Key | 9696 |
| Bazarr | Settings → General → **Security** → API Key | 6767 |

The download clients have no API key; they take the same username and password you log
into their web UI with:

| Service | Where to click | Default port |
|---|---|---|
| qBittorrent | Tools → Options → **Web UI** → Authentication | 8080 |
| NZBGet | Settings → **Security** → ControlUsername / ControlPassword | 6789 |

Every one of these is a *full-access* credential for that application. ARR-MCP never
needs more than one credential per instance, and the [permission model](#permissions) is
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

## Configuration

### Environment variables (one instance per service)

Set `<SERVICE>_URL` and `<SERVICE>_API_KEY` — or, for `QBITTORRENT` and `NZBGET`,
`<SERVICE>_USERNAME` and `<SERVICE>_PASSWORD` — and run **without** `--config`:

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
```

A service is configured only when **all** its variables are set; setting just some of
them is treated as "not configured" rather than as an error. Each configured service gets a
single instance named `default`. If no service ends up configured at all, startup fails
with a message listing the variables it looked for.

### Config file (multiple instances)

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
  services and Bazarr, or a `username` and `password` for `qbittorrent` and `nzbget`.
  Supplying the wrong kind is an error, not a silent fallback.
- Instance names must be unique within a service, and at most one may be `default`.
- With several instances and no `default`, a tool call that omits `instance` fails with a
  message listing the valid names — it does not silently pick the first one.
- Only `sonarr`, `radarr`, `prowlarr`, `bazarr`, `qbittorrent` and `nzbget` are accepted;
  anything else is rejected rather than ignored, so a typo like `sonar:` is caught
  immediately.

> An unset (or empty) `${VAR}` is a startup error, never a silent empty value — an empty
> API key would otherwise surface much later as a confusing 401.

### Server settings

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

`confirmScope` selects what gets confirmed: `write` covers both writes and deletes,
`destructive` covers deletes only.

`fallback` decides what happens when the client **cannot** prompt. Confirmation is
delivered over MCP elicitation, which a client must advertise support for during
initialisation, and many clients still don't. The default `deny` fails closed, because the
alternative is worse than it looks: without it, connecting a client that lacks elicitation
would quietly downgrade `confirm` mode into `full` — you would believe every write was
being approved while none of them ever were. Setting `allow` grants that unprompted write
access deliberately, which is a reasonable choice for a trusted local client but should be
a decision, not an accident.

Any instance can override the global policy:

```yaml
  - name: anime
    url: http://192.168.10.13:8989
    apiKey: ${SONARR_ANIME_API_KEY}
    permissions:
      mode: readonly
```

A tool is advertised if *any* instance of that service allows it; the per-instance policy
is then applied when the call actually runs. So a read-only `anime` instance alongside a
writable `main` still shows `sonarr_add_series` in the tool list, and refuses it for
`anime` at call time.

All tools also carry MCP `readOnlyHint` / `destructiveHint` annotations, so clients can
render their own warnings independently of this gating.

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

## Tools

Sonarr and Radarr are kept at parity: 48 of their tool names are common to both,
and the rest differ only where the APIs genuinely do (seasons and episodes
versus movies and collections).

The interactive search and manual import tools mirror what the two web UIs call
"interactive search" and "manual import": `*_list_releases` runs a real query
against every configured indexer and reports why each release would be accepted
or rejected, `*_grab_release` takes one anyway, and the `*_manual_import` pair
previews and then imports files the automatic importer could not place.

### Sonarr (60)

| Area | Tools | Access |
|---|---|---|
| Library | `sonarr_list_series`, `sonarr_get_series`, `sonarr_search_series`, `sonarr_list_episodes`, `sonarr_calendar` | read |
| Wanted | `sonarr_wanted_missing`, `sonarr_wanted_cutoff` | read |
| Search & import | `sonarr_list_releases`, `sonarr_manual_import_preview` | read |
| Files | `sonarr_list_episode_files`, `sonarr_rename_preview` | read |
| Profiles | `sonarr_list_quality_profiles`, `sonarr_list_quality_definitions`, `sonarr_list_custom_formats`, `sonarr_list_delay_profiles`, `sonarr_list_release_profiles` | read |
| Config | `sonarr_list_root_folders`, `sonarr_naming_config`, `sonarr_list_indexers`, `sonarr_list_download_clients`, `sonarr_list_import_lists`, `sonarr_list_notifications` | read |
| Providers | `sonarr_provider_schemas`, `sonarr_get_provider` | read |
| Tags | `sonarr_list_tags`, `sonarr_tag_details` | read |
| Operations | `sonarr_queue`, `sonarr_queue_status`, `sonarr_history`, `sonarr_blocklist`, `sonarr_health`, `sonarr_disk_space`, `sonarr_system_status`, `sonarr_list_tasks`, `sonarr_list_updates` | read |
| Add & edit | `sonarr_add_series`, `sonarr_edit_series`, `sonarr_set_season_monitored`, `sonarr_monitor_episodes`, `sonarr_create_tag`, `sonarr_update_tag` | write |
| Search & import | `sonarr_grab_release`, `sonarr_grab_queue_item`, `sonarr_manual_import`, `sonarr_mark_history_failed` | write |
| Files | `sonarr_rename_files`, `sonarr_update_files` | write |
| Providers | `sonarr_add_provider`, `sonarr_update_provider`, `sonarr_test_provider` | write |
| Automation | `sonarr_trigger_search`, `sonarr_refresh_series`, `sonarr_run_command` | write |
| Deletion | `sonarr_delete_series`, `sonarr_delete_episode_files`, `sonarr_delete_queue_item`, `sonarr_delete_queue_items`, `sonarr_delete_blocklist_item`, `sonarr_delete_provider`, `sonarr_delete_tag` | destructive |

### Radarr (59)

| Area | Tools | Access |
|---|---|---|
| Library | `radarr_list_movies`, `radarr_get_movie`, `radarr_search_movies`, `radarr_list_collections`, `radarr_calendar` | read |
| Wanted | `radarr_wanted_missing`, `radarr_wanted_cutoff` | read |
| Search & import | `radarr_list_releases`, `radarr_manual_import_preview` | read |
| Files | `radarr_list_movie_files`, `radarr_rename_preview` | read |
| Profiles | `radarr_list_quality_profiles`, `radarr_list_quality_definitions`, `radarr_list_custom_formats`, `radarr_list_delay_profiles`, `radarr_list_release_profiles` | read |
| Config | `radarr_list_root_folders`, `radarr_naming_config`, `radarr_list_indexers`, `radarr_list_download_clients`, `radarr_list_import_lists`, `radarr_list_notifications` | read |
| Providers | `radarr_provider_schemas`, `radarr_get_provider` | read |
| Tags | `radarr_list_tags`, `radarr_tag_details` | read |
| Operations | `radarr_queue`, `radarr_queue_status`, `radarr_history`, `radarr_blocklist`, `radarr_health`, `radarr_disk_space`, `radarr_system_status`, `radarr_list_tasks`, `radarr_list_updates` | read |
| Add & edit | `radarr_add_movie`, `radarr_edit_movies`, `radarr_update_collection`, `radarr_create_tag`, `radarr_update_tag` | write |
| Search & import | `radarr_grab_release`, `radarr_grab_queue_item`, `radarr_manual_import`, `radarr_mark_history_failed` | write |
| Files | `radarr_rename_files`, `radarr_update_files` | write |
| Providers | `radarr_add_provider`, `radarr_update_provider`, `radarr_test_provider` | write |
| Automation | `radarr_trigger_search`, `radarr_refresh_movies`, `radarr_run_command` | write |
| Deletion | `radarr_delete_movie`, `radarr_delete_movie_files`, `radarr_delete_queue_item`, `radarr_delete_queue_items`, `radarr_delete_blocklist_item`, `radarr_delete_provider`, `radarr_delete_tag` | destructive |

### Bazarr (33)

Subtitle management, including two instances if you run one per Sonarr/Radarr pair.

| Tool | Access |
|---|---|
| `bazarr_badges` — outstanding counts, cheapest first call | read |
| `bazarr_wanted_episodes`, `bazarr_wanted_movies` | read |
| `bazarr_list_series`, `bazarr_list_movies` | read |
| `bazarr_list_episode_subtitles` — the only source of subtitle file paths | read |
| `bazarr_list_providers`, `bazarr_list_languages`, `bazarr_list_language_profiles` | read |
| `bazarr_health`, `bazarr_system_status`, `bazarr_list_tasks` | read |
| `bazarr_manual_search_episode`, `bazarr_manual_search_movie` — candidates with scores; downloads nothing | read |
| `bazarr_episode_history`, `bazarr_movie_history`, `bazarr_list_blacklist` | read |
| `bazarr_subtitle_info` — audio and embedded tracks, for a sync reference | read |
| `bazarr_search_episode_subtitles`, `bazarr_search_movie_subtitles` — automatic, best-match | write |
| `bazarr_download_episode_subtitle`, `bazarr_download_movie_subtitle` — one named search result | write |
| `bazarr_set_series_profile`, `bazarr_set_movie_profile` — assign a languages profile | write |
| `bazarr_series_action`, `bazarr_movie_action` — scan-disk, search-missing, search-wanted | write |
| `bazarr_modify_subtitle` — sync, translate, remove_HI and the other mods | write |
| `bazarr_reset_providers`, `bazarr_run_task` | write |
| `bazarr_delete_episode_subtitle`, `bazarr_delete_movie_subtitle` | destructive |
| `bazarr_blacklist_subtitle` — deletes the file, then searches for a replacement | destructive |
| `bazarr_delete_blacklist_item` | destructive |

The automatic search tools report success whether or not a provider had a
match, because that is all Bazarr tells them. `bazarr_manual_search_episode`
lists the candidates with their scores and lets
`bazarr_download_episode_subtitle` take a named one, which is the only way to
know what was downloaded.

Blacklisting is destructive rather than a write: Bazarr deletes the subtitle
file from disk before starting the replacement search.

Subtitle *upload* is deliberately out of scope. Bazarr's upload endpoint takes
a multipart file body, and a model has no file bytes to send — only paths it
read from a tool result.

### Prowlarr (23)

Indexer management, application sync and release grabbing. Prowlarr serves
`/api/v1`, not the `/api/v3` Sonarr and Radarr use.

| Area | Tools | Access |
|---|---|---|
| Indexers | `prowlarr_list_indexers`, `prowlarr_get_indexer`, `prowlarr_list_indexer_schemas`, `prowlarr_get_indexer_schema`, `prowlarr_indexer_stats` | read |
| Search | `prowlarr_search` | read |
| Config | `prowlarr_list_applications`, `prowlarr_list_app_profiles`, `prowlarr_list_download_clients` | read |
| Tags | `prowlarr_list_tags` | read |
| Operations | `prowlarr_health`, `prowlarr_history`, `prowlarr_system_status` | read |
| Indexers | `prowlarr_add_indexer`, `prowlarr_update_indexer`, `prowlarr_test_indexer`, `prowlarr_test_all_indexers` | write |
| Operations | `prowlarr_sync_applications`, `prowlarr_grab_release`, `prowlarr_run_command` | write |
| Tags | `prowlarr_create_tag` | write |
| Deletion | `prowlarr_delete_indexer`, `prowlarr_delete_tag` | destructive |

Adding an indexer is a three-step flow: find the definition with
`prowlarr_list_indexer_schemas`, read the settings it accepts with
`prowlarr_get_indexer_schema`, then pass those field names to
`prowlarr_add_indexer`. `prowlarr_search` results carry a `guid` and an
`indexerId`, which is what `prowlarr_grab_release` needs.

### qBittorrent (22)

Every mutating tool takes `hashes` from `qbittorrent_list_torrents`; the single
element `"all"` targets every torrent, which is how the WebUI's select-all buttons work.

| Tool | Access |
|---|---|
| `qbittorrent_list_torrents` | Read |
| `qbittorrent_torrent_files` | Read |
| `qbittorrent_transfer_info` | Read |
| `qbittorrent_list_categories` | Read |
| `qbittorrent_list_tags` | Read |
| `qbittorrent_system_status` | Read |
| `qbittorrent_add_torrent` | Write |
| `qbittorrent_start_torrents` | Write |
| `qbittorrent_stop_torrents` | Write |
| `qbittorrent_recheck_torrents` | Write |
| `qbittorrent_set_category` | Write |
| `qbittorrent_create_category` | Write |
| `qbittorrent_edit_category` | Write |
| `qbittorrent_add_tags` | Write |
| `qbittorrent_remove_tags` | Write |
| `qbittorrent_set_location` | Write |
| `qbittorrent_rename_torrent` | Write |
| `qbittorrent_set_priority` | Write |
| `qbittorrent_set_torrent_limits` | Write |
| `qbittorrent_set_global_limits` | Write |
| `qbittorrent_delete_torrents` | Destructive |
| `qbittorrent_delete_categories` | Destructive |

### NZBGet (18)

Usenet download client, spoken to over its JSON-RPC API with basic auth
(username and password, not an API key).

| Tool | Access |
|---|---|
| `nzbget_status` — speed, limit, disk space, pause states | read |
| `nzbget_list_queue` — source of the NZBIDs every editing tool takes | read |
| `nzbget_history` — finished, failed and deleted downloads | read |
| `nzbget_add_nzb` — by URL or base64 nzb content | write |
| `nzbget_pause_download`, `nzbget_resume_download` — whole queues: download, post or scan | write |
| `nzbget_pause_items`, `nzbget_resume_items` | write |
| `nzbget_move_items`, `nzbget_set_priority` | write |
| `nzbget_set_category`, `nzbget_rename_item` | write |
| `nzbget_retry_history_items` — return to the queue, optionally redownloading | write |
| `nzbget_mark_history_items` — good or bad for duplicate handling | write |
| `nzbget_set_rate_limit`, `nzbget_scan` | write |
| `nzbget_delete_items` — to history by default; `final` discards permanently | destructive |
| `nzbget_delete_history_items` — hides by default; `final` removes permanently | destructive |

### What responses contain

Upstream payloads are far too large to return as they arrive — a single Sonarr
custom format list is 283 KB, one series' episode files 252 KB, and Radarr's
collection list 259 KB. Every tool returns a projection: identities, counts and
the fields that answer a question, never overviews, artwork, alternate titles or
embedded media info.

Indexer, download client, import list and notification listings never return the
provider `fields` array. That array holds each provider's own credentials —
indexer API keys, download client passwords, notification webhook URLs — and
none of it belongs in a model's context.

Prowlarr's indexer tools do return that array, because you cannot configure an
indexer without knowing its field names — but every field whose upstream
`privacy` is anything other than `normal` reports `***` in place of its value,
which covers Prowlarr's `apiKey`, `password` and `userName` fields.

Sonarr's and Radarr's provider tools (`*_get_provider`, `*_add_provider`,
`*_update_provider`) redact that same array by the same rule, so an indexer's
API key or a download client's password is reported as `***` while its field
name still is not. Editing a provider sends the stored values back untouched:
the mask is never written over a credential the caller did not change.

Services with no configured instances register no tools at all, so the advertised list always reflects what is actually reachable.

## Troubleshooting

Start with `--check`. It exercises exactly the credentials and URLs the tools will use, and
prints one line per instance, so it separates "my configuration is wrong" from "my client
is wrong" in a single command:

```bash
docker run --rm --env-file .env ghcr.io/gauranshmathur/arr-mcp --check
# or, from a local binary
arr-mcp --config config.yaml --check
```

### `401 Unauthorized`

```
FAIL  radarr/main (http://192.168.10.14:7878): radarr returned 401: Unauthorized
```

The URL is right — something answered — but the API key is not. Re-copy it from
[Settings → General → Security](#find-your-api-key); a trailing space or a truncated paste
is the usual cause. Check you did not swap keys between two instances of the same service,
which produces exactly this error on both.

### `connection refused` or a timeout

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

### The client connects but shows no tools

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

### Writes are refused without ever prompting

```
client does not support elicitation: cannot confirm write tool sonarr_add_series;
set permissions.fallback=allow or permissions.mode=full to permit it
```

The default `confirm` mode asks for approval through MCP elicitation, and your client does
not implement it. There is no prompt to answer, so the call fails closed rather than
running unapproved — see [Permissions](#permissions) for why that default is the safe one.
Either switch to a client that supports elicitation, or make the decision explicit:

```yaml
permissions:
  mode: confirm
  confirmScope: destructive   # only deletes need confirming
  fallback: allow             # writes proceed unprompted
```

`confirmScope: destructive` is usually the better trade: adds and command triggers run
freely, and the calls that remove things still fail closed.

### The server starts, then exits immediately under stdio

That is normal. A stdio server lives for exactly as long as its client holds the pipe
open; running it by hand in a terminal ends as soon as stdin closes. Test it with
`--check`, or with the MCP inspector (see [Development](#development)), not by launching
it bare.

### Startup fails with `references unset environment variable(s)`

A `${VAR}` in `config.yaml` has no value in the process environment. Under Docker this
almost always means the variable is in your shell but was never passed into the container
— add it to `--env-file` / the compose `env_file`. Empty counts as unset, on purpose.

### Nothing at all appears in the logs

Logging always goes to **stderr**. Under the stdio transport, stdout carries the JSON-RPC
stream and must not be written to by anything else. Most clients file that stderr away
under their own logs — Claude Desktop, for example, writes
`~/Library/Logs/Claude/mcp-server-arr.log` on macOS. Raise the detail with
`--log-level debug`.

## Scope

ARR-MCP covers the \*arr-named applications that share the common \*arr API contract: the
same versioned `/api` shape, the same API-key header, the same `/system/status` and health
endpoints. That shared contract is the whole reason the project is cheap to extend — a
service is described by a `ServiceSpec` rather than a bespoke client, so it inherits the
transport, the instance registry and the permission model for free. The two download
clients are the deliberate exceptions: each needed exactly one extra auth scheme on that
shared transport, not a client of its own. Anything that would need more than that is
where the line is drawn.

### Planned

Maintainerr, Cleanuparr and Notifiarr.

### Not planned: media servers and request managers (Jellyfin, Overseerr, Plex)

None of them are \*arr-named, and none of them speak the \*arr API contract — Plex uses
its own token scheme and XML-flavoured API, and the request managers wrap their own
approval workflows around a different data model. Supporting any of them means a bespoke
client with its own auth handling, response shapes and tests, for a capability that
overlaps heavily with what the \*arr tools already expose.

### Download clients (qBittorrent, NZBGet)

The \*arr queue tools already show what is downloading, and
`sonarr_delete_queue_item` can drop a stuck download with `removeFromClient`.
What they cannot do is anything the \*arr app did not initiate: add a torrent
or NZB by hand, pause or reprioritise the client's queue, change categories
and speed limits, or clean up the client's history. Those are the operations
the download-client tools cover, so you can do from an MCP client what you
would otherwise do in the client's own web UI.

Neither client speaks the \*arr contract, so each got exactly one addition to
the shared transport rather than a bespoke client: qBittorrent logs in with a
username and password and replays the session cookie (`AuthSession`), and
NZBGet's JSON-RPC rides on HTTP basic auth (`AuthBasic`). Everything else —
the instance registry, permission tiers, `--check`, credential redaction — is
inherited unchanged.

### Not planned: SABnzbd

SABnzbd's query-parameter `mode=` API would need a third transport shape for
an application NZBGet already covers in this stack. Open an issue if you run
SABnzbd and want it.

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

## Development

```bash
go test ./... -race -cover
go build -o arr-mcp ./cmd/arr-mcp
```

Inspect the tool surface interactively:

```bash
npx @modelcontextprotocol/inspector ./arr-mcp --transport stdio --config config.yaml
```

Adding a service means describing its API rather than writing a new client — `ServiceSpec` carries the base path, health path and auth scheme, so services on different API versions and auth headers share one transport:

```go
var BazarrSpec = ServiceSpec{
    Name: "bazarr", BasePath: "/api", StatusPath: "/system/status",
    Auth: AuthHeaderKey,
}
```

Three auth schemes exist: `AuthHeaderKey` (the \*arr apps and Bazarr), `AuthBasic`
(NZBGet) and `AuthSession` (qBittorrent's form login, with the session cookie cached per
instance and refreshed once on a 403).

Releases are cut by [release-please](https://github.com/googleapis/release-please): conventional commits on `main` accumulate into a version-bump PR, and merging it tags the release and publishes multi-arch images to GHCR. Images are Trivy-scanned **before** push, so a vulnerable tag is never publicly pullable.

## License

See [LICENSE](LICENSE).
