# AGENTS.md

Conventions for working in this repository. Read this before adding a service or a tool.

## Non-negotiables

1. **TDD.** Write the test, run it, watch it fail for the right reason, then implement. Tests written after the code prove nothing — you never saw them catch anything.
2. **Verify the API before implementing it.** Never write a client from memory. See "Researching a service" below. Most of the bugs already fixed in this repo came from assuming an API shape.
3. **Never write to stdout.** Under the stdio transport, stdout carries the JSON-RPC stream. A single stray `fmt.Println` corrupts the framing and kills the session. Logging goes to stderr; `TestLoggerDefaultsToStderr` pins this.
4. **Conventional commits.** `feat(scope):`, `fix(scope):`, `docs:`, `build:`, `ci:`, `test:`, `chore:`. release-please builds the changelog from these, so the subject line is user-facing. Explain *why* in the body.
5. **One feature per branch, one branch per worktree.** See "Worktrees".
6. **One release per feature.** See "Releasing".

## Researching a service

Do this in order, and stop at the first that gives an authoritative answer:

1. **Live OpenAPI/Swagger document.** Some services expose one unauthenticated:
   ```bash
   curl -s http://<host>:<port>/api/swagger.json | head -c 200
   ```
   Bazarr does. Sonarr 4.x and Radarr 6.x do **not** — they answer 404 even with
   a valid key, and unauthenticated they answer 401 for every path including
   nonexistent ones, which disguises the 404. For those, use the upstream repo's
   spec and verify each endpoint against the live instance.
2. **Probe the live instance.** Response schemas are often missing from the spec even when the paths are documented. When a key is needed, run the request *inside* the pod so the credential never leaves it:
   ```
   kubectl exec <pod> -- sh -c 'K=$(...); curl -s -H "X-API-KEY: $K" http://127.0.0.1:<port>/api/<path>'
   ```
3. **context7** for published library/API documentation.

Record anything surprising in the commit body. Real examples already found: Prowlarr serves `/api/v1` while Sonarr and Radarr serve `/api/v3`; Bazarr wraps most responses in `data` but `/badges` and `/system/languages` are bare; provider resources (indexers, download clients, notifications) embed credentials in a `fields` array that must never reach a tool result.

**Header case is not yours to choose.** `net/http` canonicalises header keys, so `ServiceSpec.AuthHeader` selects a header *name*, never its casing — `"X-API-KEY"` goes on the wire as `X-Api-Key`. A test asserting with `Header.Get` cannot detect this, because `Get` canonicalises the lookup too. Verify against the live service which spellings it accepts before concluding one is required.

## Adding a service

Adding a service should mean *describing* it, not writing a new client.

1. Add a `ServiceSpec` in `pkg/arr/client.go` — base path, status path, auth scheme, and auth header if it is not `X-Api-Key`. Auth schemes: `AuthHeaderKey` (the \*arr apps, Bazarr), `AuthBasic` (NZBGet), `AuthSession` (qBittorrent's form login + cookie).
2. Add the service name to `config.KnownServices` in `pkg/config/loader.go` and to the `specs` map in `cmd/arr-mcp/main.go`. If it authenticates with a username and password rather than an API key, add it to `serviceCredentials` too — that drives config validation and the `<SERVICE>_USERNAME`/`<SERVICE>_PASSWORD` env fallback.
3. Add `pkg/arr/<service>.go` with typed calls, and `pkg/arr/<service>_test.go` using `fakeService` from `client_test.go`. Two call shapes exist: REST services use `GetJSON`/`Post`/`PostForm`; NZBGet's JSON-RPC goes through `nzbCall` in `nzbget.go`.
4. Add `pkg/server/register_<service>.go` and `pkg/server/tools_<service>.go` (input/output structs), and call the register function from `registerAll`. Server-level tests go in `register_<service>_test.go`, not `server_test.go`.
5. Add the service to `README.md`, `config.example.yaml`, `.env.example` and `deploy/kubernetes/`.

If a service does not fit any existing auth scheme, that is a signal — say so before adding one. The two that exist beyond `AuthHeaderKey` were added deliberately, one each.

## Tool design

**Naming:** `<service>_<verb>_<noun>`, snake_case — `sonarr_list_series`, `bazarr_wanted_episodes`.

**Access tiers.** Every tool declares one, and it drives both the permission gate and the MCP annotations:

| Tier | Meaning |
|---|---|
| `AccessRead` | Reads only. Never prompts. |
| `AccessWrite` | Creates or modifies. |
| `AccessDestructive` | Deletes state or files. |

Be honest about the tier. Getting it wrong is how a library gets deleted without a prompt.

**Every input embeds `InstanceArg`.** That is what makes multi-instance work — the registration helper resolves the instance and populates a schema `enum` of configured names.

**Trim responses.** Upstream payloads are enormous: a Sonarr series is ~40 fields including a paragraph-long `overview`, artwork paths and alternate titles. A library listing returned raw would consume most of the model's context before it could answer anything. Define a trimmed struct and a `raw*` struct, and project between them. Ask: does this field answer a question a user would actually ask?

**Errors are part of the interface.** The caller is a model that can self-correct — but only from a specific error. `Resolve` appends the valid instance names for exactly this reason. Prefer `unknown instance "typo"; configured instances: main, anime` over `not found`.

**Outputs are structs, not slices.** MCP structured content wants an object at the top level. Wrap lists (`SeriesList`, `WantedEpisodeList`) and include a count.

## Worktrees

Feature work happens in parallel worktrees under `.claude/worktrees/` (gitignored):

```bash
git worktree add .claude/worktrees/<name> -b feat/<name> origin/main
```

Branch from `origin/main` unless the work depends on an unmerged branch; then base on that branch and open the PR stacked on it. GitHub retargets stacked PRs automatically when the base merges.

Keep each branch to its own files where possible: new services get their own `register_<service>.go`, `tools_<service>.go` and test files, so parallel worktrees only ever conflict on the one-line `registerAll` entry and their own README section.

## Releasing

**Merge with an empty body:**

```bash
gh pr merge <n> --merge --body ""
```

GitHub's default merge commit repeats the PR title in its body, and release-please parses that as a second conventional commit — which is why every entry in the changelog before v1.1.0 appears twice. An empty body leaves only `Merge pull request #n from …`, which is not a conventional commit, so each change is listed once.

**One release per feature.** release-please keeps a single rolling release PR that every merged `feat:`/`fix:` updates; a release is cut when *that* PR merges, not when a feature merges. So after merging a feature PR, wait for the release PR to regenerate and merge it too. Letting several features pile into one release PR is not wrong, just a different cadence — this repo's is one feature, one version, one image.

Merging the release PR tags the version and publishes to GHCR in the same run. Never hand-edit a version or `CHANGELOG.md`; release-please owns both.

## Before you claim done

```bash
go test ./... -race     # must pass
go vet ./...            # must be clean
gofmt -l .              # must be empty
```

Those three are not the whole gate. CI also runs three tools that catch things
`go vet` does not, so run them here rather than discovering them in a red PR:

```bash
golangci-lint run       # pinned to the version in .github/workflows/ci.yml
gosec -quiet ./...
trivy fs --ignorefile .trivyignore.yaml --scanners vuln,secret,misconfig \
  --severity CRITICAL,HIGH,MEDIUM .
```

Two of these have bitten already. staticcheck rejects `!(a && b)` and wants the
De Morgan form. gosec reports any `http.Cookie` literal as missing `Secure`,
`HttpOnly` and `SameSite`, even on an *outbound request* cookie where those
attributes do not exist and are never serialised — set the `Cookie` header
directly instead. Trivy's misconfiguration checks match ConfigMap **key names**
(`password`, `apiKey`, `username`), not values, so the deployment example trips
them despite holding only `${VAR}` placeholders; the exception lives in
`.trivyignore.yaml`, scoped to that path with its reason recorded. Scope any new
exception the same way, and never ignore a secret-scanner finding.

Then verify the protocol actually works, because unit tests do not prove MCP compliance:

```bash
printf '%s\n' \
 '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"probe","version":"1"}}}' \
 '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
 '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' > /tmp/in.jsonl
{ cat /tmp/in.jsonl; sleep 2; } | ./arr-mcp --config config.yaml --transport stdio --log-level error
```

Keep stdin open briefly — an immediate EOF closes the session before responses flush, which looks like a hang but is not one.

Report what you ran and what it printed. Do not claim a test passes without having seen it pass.
