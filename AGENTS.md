# AGENTS.md

Conventions for working in this repository. Read this before adding a service or a tool.

## Non-negotiables

1. **TDD.** Write the test, run it, watch it fail for the right reason, then implement. Tests written after the code prove nothing — you never saw them catch anything.
2. **Verify the API before implementing it.** Never write a client from memory. See "Researching a service" below. Most of the bugs already fixed in this repo came from assuming an API shape.
3. **Never write to stdout.** Under the stdio transport, stdout carries the JSON-RPC stream. A single stray `fmt.Println` corrupts the framing and kills the session. Logging goes to stderr; `TestLoggerDefaultsToStderr` pins this.
4. **Conventional commits.** `feat(scope):`, `fix(scope):`, `docs:`, `build:`, `ci:`, `test:`, `chore:`. release-please builds the changelog from these, so the subject line is user-facing. Explain *why* in the body.
5. **One feature per branch, one branch per worktree.** See "Worktrees".

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

1. Add a `ServiceSpec` in `pkg/arr/client.go` — base path, status path, auth scheme, and auth header if it is not `X-Api-Key`.
2. Add the service name to `config.KnownServices` in `pkg/config/loader.go` and to the `specs` map in `cmd/arr-mcp/main.go`.
3. Add `pkg/arr/<service>.go` with typed calls, and `pkg/arr/<service>_test.go` using `fakeService` from `client_test.go`.
4. Add `pkg/server/register_<service>.go` and call it from `registerAll`.
5. Add the service to `README.md`, `config.example.yaml` and `.env.example`.

If a service does not fit `ServiceSpec` at all, that is a signal — say so before writing a bespoke client. Download clients were rejected on exactly this basis; see the Scope section of the README.

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
git worktree add .claude/worktrees/<name> -b feat/<name> feat/mcp-rewrite
```

Branch from `feat/mcp-rewrite` until it merges — the foundation is not yet on `main`, so branching from `origin/main` loses everything.

Open PRs stacked on the branch you based on. GitHub retargets them automatically when the base merges.

## Before you claim done

```bash
go test ./... -race     # must pass
go vet ./...            # must be clean
gofmt -l .              # must be empty
```

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
