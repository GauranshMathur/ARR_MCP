# Development

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

Five auth schemes exist: `AuthHeaderKey` (the \*arr apps, Bazarr, Seerr, Jellyfin and Jellystat, Jellyfin
setting `AuthHeaderFormat` to wrap the key as `MediaBrowser Token="%s"`), `AuthBasic`
(NZBGet), `AuthSession` (qBittorrent's form login, with the session cookie cached per
instance and refreshed once on a 403), `AuthNone` (Maintainerr) and `AuthDSM` (Synology's
form login, with the session id cached per instance and sent as a form field).

Releases are cut by [release-please](https://github.com/googleapis/release-please): conventional commits on `main` accumulate into a version-bump PR, and merging it tags the release and publishes multi-arch images to GHCR. Images are Trivy-scanned **before** push, so a vulnerable tag is never publicly pullable.
