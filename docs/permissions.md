# Permissions

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
