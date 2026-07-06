# spanner

> The gnomish engineer's SimulationCraft workbench. Simulate locally. Explode occasionally.

**spanner** is a free, local-first frontend for [SimulationCraft](https://github.com/simulationcraft/simc):
paste your `/simc` export, spanner runs the sim on *your* machine, and renders a
report that doesn't look like it was riveted together in 2011. No queue, no
subscription, no shipping your character to someone else's cloud.

## Status

Phase 0 (proof of pipeline): run the SimC CLI → parse `json2` output → render a
friendly single-file HTML report. Expect sparks.

## How it fits together

One Go core, thin clients on top:

| Module | Job |
|---|---|
| `internal/forge` | SimC binary management — locate (later: fetch and verify) the `simc` executable |
| `internal/crank` | Run orchestration — turn a profile into TCI input, spawn and supervise `simc` |
| `internal/gauge` | Parse SimC `json2` output into a stable internal model |
| `internal/blueprint` | Render the internal model as a self-contained HTML report |
| `cmd/spanner` | The terminal client |

## Install

```sh
brew install zandoh/tap/spanner        # macOS / Linux
```

or grab a binary from [releases](https://github.com/zandoh/spanner/releases)
(Windows/Linux/macOS, amd64 + arm64). On macOS and Windows, spanner fetches
SimulationCraft for you (`spanner forge update`); on Linux, build simc
[from source](https://github.com/simulationcraft/simc) and put it on PATH.

## Quick start

```sh
spanner forge update                   # install/refresh the simc nightly (rerun after game patches)
pbpaste | spanner char save zandy      # save your /simc export once (type /simc in-game, copy)
spanner sim -char zandy                # sim → report opens in browser
spanner serve                          # or use the local web workbench
```

More tools:

```sh
pbpaste | spanner sim -import -        # one-off sim straight from the clipboard
spanner weights -char zandy            # stat weights (slower: one sim per stat)
spanner runs                           # past runs, newest first
spanner compare -char zandy \
  -vs "Crafted boots=feet=,id=219911" \
  -vs "New talents=talents=CoPAAAA..."  # rank variations against your setup
```

Binary resolution order: `-simc` flag → `SPANNER_SIMC` env → newest cached
nightly (`spanner forge which` shows it) → `PATH`. Reports land in
`./reports/`.

## Development

Requires Go.

```sh
make build   # → bin/spanner
make tools   # install goimports, golangci-lint, govulncheck
make check   # fmt-check + vet + lint (staticcheck/gosec/depguard) + vuln scan + tests
```

CI runs the same `make check` on every push.

## License

spanner is MIT licensed. SimulationCraft is a separate GPL-3.0 project which
spanner invokes as an external process; spanner does not embed or link SimC.
World of Warcraft is a trademark of Blizzard Entertainment; spanner is an
unaffiliated fan-made tool.
