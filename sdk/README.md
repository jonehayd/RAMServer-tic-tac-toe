# sdk

The RAMServer's public Go API for writing rulesets, provided here so this repository builds without
access to the server.

| Package | Import path | Contents |
| --- | --- | --- |
| `game` | `github.com/stevenlagoy/ramserver-core/server/game` | The `Game` interface and its types, `Registry`, roles, and the `ErrIllegalAction` / `ErrMalformedAction` errors. A copy of the server's package, unchanged apart from one comment that referred to code not included here. |
| `gametest` | `github.com/stevenlagoy/ramserver-core/server/game/gametest` | Compliance tests for a `Game`. See [Writing a ruleset](../docs/writing-a-ruleset.md#testing). |

This directory is its own Go module, with the same module path as the server, and the repository's
`go.mod` points that path here:

```
replace github.com/stevenlagoy/ramserver-core/server => ./sdk
```

So rulesets import exactly the same paths here as inside the server, and can move into it unchanged.

- **Don't edit anything in this directory.** If the interface seems wrong or incomplete, report it.
- `go test ./...` from `sdk/` runs the compliance tests' own tests.
