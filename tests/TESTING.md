# Testing and code metrics (standard library only)

All tests live in the `tests/` folder (package `tests`), so they can only use the
exported API of the code under test. `main.go` and `tools/metrics` are `package main`
and cannot be imported, so `tests/cli_test.go` and `tests/metrics_test.go` build them
and run them as real programs.

| File | What it checks |
|---|---|
| `domain_test.go` | SRD rules (modifiers, proficiency, spell slots, armor class), `Character` invariants, encapsulation (getters return copies) |
| `service_test.go` | `CharacterService` with the real JSON/CSV repositories: creation, equipment, spells, edits, idempotency keys, concurrent updates, legacy/corrupt files |
| `queries_test.go` | read side: queries never save, concurrent enrichment, cancellation, SRD API client (fake server, rate limit) |
| `web_test.go` | HTTP pages and JSON API: status codes, `Idempotency-Key`, concurrent retries, `/healthz`, enrichment endpoints |
| `architecture_test.go` | dependency direction between packages; all tests live in `tests/` |
| `cli_test.go` | the built CLI binary: output text and exit codes |
| `metrics_test.go` | the metrics tool |

    go test ./tests/                   # run all tests
    go test ./tests/ -coverpkg=./internal/...,./web/... -coverprofile=coverage.out
    go tool cover -html=coverage.out   # line-by-line coverage report
    go vet ./...                       # static checks
    go run ./tools/metrics             # total lines, unit size, cyclomatic complexity, duplication

Note: `-coverpkg` is required, otherwise Go only measures the `tests` package itself.
The CLI runs as a separate process. To measure `main.go` too, let the CLI tests
build instrumented binaries:

    mkdir -p .cover/cli
    MPP_CLI_COVERDIR="$PWD/.cover/cli" go test ./tests/ -coverpkg=./internal/...,./web/... -coverprofile=coverage.out
    go tool covdata textfmt -i=.cover/cli -o=coverage-cli.out

## Race detector

`go test -race` needs cgo (a C compiler). Without one locally, run it in Docker
(PowerShell, from the project folder):

    docker run --rm -v "${PWD}:/src" -w /src golang:1.24 go test -race ./...

SonarQube settings are in `../sonar-project.properties`; the scan commands are in
`../docs/QUALITY_AND_TESTING.md`.
