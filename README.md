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

## Quick start

Requires Go and a SimulationCraft CLI binary ([nightlies](http://downloads.simulationcraft.org/nightly/)
for Windows/macOS; build from source on Linux).

```sh
make build

# from a /simc addon export (type /simc in-game, copy, paste into a file — or pipe it)
./bin/spanner sim -import mychar.txt
pbpaste | ./bin/spanner sim -import -

# or from a .simc profile file
./bin/spanner sim -profile mychar.simc
```

`-simc /path/to/simc` is optional if `simc` is on your `PATH` or
`SPANNER_SIMC` is set. The report lands in `./reports/` and opens in your
browser.

## Development

```sh
make tools   # install goimports, staticcheck, govulncheck, gosec
make check   # fmt-check + vet + lint + security scans + tests
```

CI runs the same `make check` on every push.

## License

spanner is MIT licensed. SimulationCraft is a separate GPL-3.0 project which
spanner invokes as an external process; spanner does not embed or link SimC.
World of Warcraft is a trademark of Blizzard Entertainment; spanner is an
unaffiliated fan-made tool.
