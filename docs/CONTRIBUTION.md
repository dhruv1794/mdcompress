# Contributing

## Prerequisites

- Go 1.22 or later
- Git
- (Optional) Ollama running locally for Tier-3 and eval tests

Clone and build:

```sh
git clone https://github.com/dhruv1794/mdcompress
cd mdcompress
go build ./cmd/mdcompress
go test ./...
```

## Project conventions

- No comments that describe what the code does — well-named identifiers handle that. Add a comment only when the *why* is non-obvious.
- No speculative abstractions. Solve the problem at hand; don't build for hypothetical future requirements.
- Error handling at system boundaries only. Trust internal package guarantees.
- Golden tests live in `internal/testdata/golden/`. Update them with `-update` when output intentionally changes (see testing section below).

## Adding a new compression rule

1. Create `pkg/rules/<rule_name>.go`. Implement the `Rule` interface:

```go
type MyRule struct{}

func (r *MyRule) Name() string { return "my-rule-name" }
func (r *MyRule) Tier() rules.Tier { return rules.TierSafe } // or TierAggressive

func (r *MyRule) Apply(doc ast.Node, ctx *rules.Context) (rules.ChangeSet, error) {
    var cs rules.ChangeSet
    // walk doc, collect render.Edit{} or render.Range{} values
    // increment cs.Stats.NodesAffected per affected node
    return cs, nil
}
```

2. Register the rule in `pkg/rules/registry.go` by adding it to `allRules` in the correct position. Rules run in declaration order; position matters for rules that may interact.

3. Write a test file `pkg/rules/<rule_name>_test.go`. Cover the happy path, the no-op path (rule finds nothing to remove), and at least one edge case (fenced code blocks, inline vs block context, etc.).

4. If the rule is Tier 2 (`TierAggressive`) it must pass the faithfulness eval before merging:

```sh
mdcompress eval --repo=. --rule=my-rule-name --backend=ollama --model=llama3.1:8b
```

   A score below 0.95 is a hard block.

5. If the rule should be opt-in by default even when its tier is active, add its name to `DefaultDisabled` in `pkg/rules/registry.go`.

## Running tests

```sh
# All tests
go test ./...

# A specific package
go test ./pkg/rules/...

# Update golden files after intentional output changes
go test ./pkg/compress/... -update

# Run the faithfulness eval locally (requires Ollama)
mdcompress eval --repo=. --backend=ollama --model=llama3.1:8b
```

## Changing the CLI

All commands are registered in `cmd/mdcompress/main.go` (general commands) or `cmd/mdcompress/serve.go` (MCP-related commands). Each command is a function returning `*cobra.Command`. Keep command functions small — delegate logic to packages under `pkg/`.

Config parsing lives in `readProjectConfig` inside `main.go`. The config format is a hand-rolled YAML subset; avoid introducing a YAML library dependency to keep the binary lean.

## Changing the cache or manifest format

`pkg/cache` and `pkg/manifest` are the only packages that touch the `.mdcompress/` directory. Changes to the manifest JSON structure must remain backwards-compatible with existing manifests (add fields; don't rename or remove them). If a breaking change is unavoidable, bump the manifest version field and add a migration in `manifest.Read`.

## MCP server changes

The MCP server lives in `pkg/server/`. It communicates over stdio using JSON-RPC and must remain stateless between requests (the LRU cache is the only in-process state). When adding a new MCP tool:

1. Add the handler method to `pkg/server/server.go`.
2. Register the tool in the `tools/list` response.
3. Dispatch the call in the `tools/call` handler.
4. Add a test in `pkg/server/server_test.go`.

## Pull request checklist

- `go test ./...` passes
- `go vet ./...` reports no issues
- Golden files are updated if output changed
- New Tier-2 rules have a passing faithfulness eval result attached to the PR
- No new external dependencies without discussion first
- Commit messages explain *why*, not *what*

## Releasing

Releases are built via GoReleaser (`.goreleaser.yaml`). Tags trigger the release workflow. Do not manually edit `version` in `main.go`; GoReleaser injects it at build time via `-ldflags`.
