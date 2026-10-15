# Quality and testing

## 1. Quality model

The assessment uses the product quality model of **ISO/IEC 25010:2023**
(second edition, which replaced the 2011 edition). Its nine characteristics
are functional suitability, performance efficiency, compatibility,
interaction capability (formerly usability), reliability, security,
maintainability, flexibility (formerly portability) and safety.
Maintainability has the subcharacteristics modularity, reusability,
analysability, modifiability and testability. Reliability has faultlessness,
availability, fault tolerance and recoverability.

The terminology was checked against secondary sources (arc42 quality model,
SonarSource and other summaries, see [STANDARDS.md](STANDARDS.md)); the
standard text itself was not available. One source (arc42) lists only four
maintainability subcharacteristics without testability, while others list
five; this document follows the five-subcharacteristic list.

The standard defines no thresholds. The targets below are the project's own
choices, and passing a tool's quality gate does **not** mean the product
"complies with ISO/IEC 25010".

## 2. Maintainability assessment

Baseline = the project as received (commit `481e8a9`, plus the untracked
`tests/` folder), measured on 2026-10-15 with the project's own metrics tool
(`go run ./tools/metrics`) and `go test` coverage. Final = the current
working tree.

| Sub-characteristic | Risk | Design decision | Metric / test | Baseline | Final | Remaining weakness |
|---|---|---|---|---|---|---|
| Modularity | A change to storage or HTTP breaks business rules | Layers with interfaces; domain has no I/O imports | `TestDependencyDirection` (forbidden imports) | domain imported `encoding/json`; `main.go` and `web` held the workflows | 0 violations | One Go module; contexts are separated by package only |
| Reusability | CLI and web implement rules twice | One `CharacterService` used by both | duplicated code (`tools/metrics`, 6-line blocks) | 4.8% | 2.5% (SonarQube: 0.0%) | `web/templates/charactersheet.html` JavaScript still re-implements some SRD formulas for live preview |
| Analysability | Large functions hide rules | Split `main()` into one function per command; extract helpers | % of unit code in functions > 60 lines / CC > 25 | 26.8% / 16.4% | 0% / 0% | 9% of unit code still has CC 11–25 (rule tables such as `armorClass`) |
| Analysability | High cognitive complexity | Same | SonarQube cognitive-complexity rule (go:S3776, limit 15) | not measured on the original code (the first scan ran after the restructuring and found 4 functions in project code and 4 in `tools/` above 15) | 0 | – |
| Modifiability | A rule change touches many files | SRD rules in one file (`rules.go`); single repository per data source | change locality example: correcting the spell-slot table touched only `rules.go` | rules spread over `character.go`, `main.go`, `web/server.go` | 1 file | – |
| Testability | Business rules only testable through the CLI binary | Pure functions; services take interfaces; fakes for the external API | statement coverage `internal/` + `web/` | 78.9% (2 tests failing) | 87.4% (all 47 tests pass) | `main.go` is measured only via the built binary (83.6%) |
| Testability | Untestable external dependency | `ResourceLookup` interface, `httptest` fake server, configurable API URL | tests run without network | external API called directly from `web` | no test uses the network | – |

Additional reliability and security evidence:

| Characteristic | Evidence | Result |
|---|---|---|
| Reliability – faultlessness | `go test ./...` | 47 tests pass |
| Reliability – faultlessness | `go test -race ./...` (Docker, Go 1.24.13) | no data races |
| Reliability – faultlessness | SonarQube bugs | 0 (first scan: 153, all accessibility findings in the HTML template) |
| Reliability – recoverability | atomic file replacement (`writeAll`) | no half-written file possible |
| Reliability – availability | Docker failover (M-05, M-06) | reads continued with instances stopped |
| Security | SonarQube vulnerabilities / hotspots | 0 / 0 |
| Static analysis | `go vet ./...`, `staticcheck` 2025.1.1 | no findings |

## 3. Test strategy

All automated tests use the standard `testing` package and live in `tests/`
(project convention), so they test the exported API only, like a client
would. Tests use temporary directories and fake servers; they never touch the
real `characters.json` or the internet.

| Level | What | Files |
|---|---|---|
| Unit – domain | invariants, value objects, SRD tables, encapsulation | `domain_test.go` |
| Integration – application + real repositories | commands with the real JSON and CSV repositories on temporary files; idempotency; concurrency; legacy file format | `service_test.go` |
| Unit/integration – queries and external API | concurrent enrichment, cancellation, rate limiting, error translation (fake HTTP server) | `queries_test.go` |
| API / contract | status codes, JSON bodies, headers (`Idempotent-Replayed`, `X-Instance`), pages, form posts | `web_test.go` |
| System – CLI | the built binary: output text and exit codes | `cli_test.go` |
| Architecture | dependency direction, all tests in `tests/` | `architecture_test.go` |
| Tooling | the metrics tool | `metrics_test.go` |
| Concurrency | `TestConcurrentChangesAreNotLost` (24 goroutines, every spell must be stored exactly once), `TestConcurrentRetriesCreateOnce` (20 goroutines, one creation + 19 replays), `TestCreateWithIdempotencyKey` (10 concurrent HTTP retries), `TestSRDAPIClientRateLimit`, `TestEnrichCharacterRunsLookupsConcurrently` | – |
| Characterisation (golden) | 3446-line transcript of the original CLI (every class at 8 levels, every race, all error paths) compared with the new CLI; done during the refactoring, not part of `go test` | see §6 |
| Manual | deployment, browser UI | [MANUAL_TEST_PLAN.md](MANUAL_TEST_PLAN.md) |
| Deployment | Docker DNS, load balancing, failover, read-only volume, non-root user, graceful stop | manual tests M-01 – M-11 |

Tests are table-driven where several cases share one shape, include negative
and boundary cases (level 0/21, score 31/−1, occupied slots, wrong spell
list), and assert results rather than only running code.

### Traceability matrix

| Req. | Requirement (short) | Implementation | Verification | Test / evidence | Status |
|---|---|---|---|---|---|
| FR-01 | create character | `CharacterService.Create`, `buildCharacter`, `NewCharacter` | automated | `TestServiceCreate`, `TestCLICreateViewListDelete`, `TestCLIValidation` | pass |
| FR-02 | Standard Array (web) | `AbilityScores.HasStandardArray`, `RequireStandardArray` | automated | `TestHasStandardArray`, `TestCharactersAPI` | pass |
| FR-03 | read sheet | `Character` derived methods, `printCharacter`, `CharacterQueries.Get` | automated | `TestCombatStats`, `TestCLICreateViewListDelete` | pass |
| FR-04 | list | `CharacterQueries.GetAll` | automated | `TestCLICreateViewListDelete`, `TestPages` | pass |
| FR-05 | update level | `Character.ChangeLevel`, `CharacterService.ChangeLevel`, CLI `level` | automated + manual | `TestChangeLevel`, `TestServiceSpellsLevelAndDelete`, M-14, M-15 | pass |
| FR-06/07 | equipment | `EquipWeapon/Armor/Shield`, `armorClass` | automated | `TestEquipment`, `TestArmorClass`, `TestCLIEquip` | pass |
| FR-08/09 | spells and slots | `addSpell`, `spellSlots` | automated | `TestSpells`, `TestSpellSlotsMatchTheSRD`, `TestCLISpells` | pass |
| FR-10 | delete | `CharacterService.Delete` | automated | `TestServiceSpellsLevelAndDelete`, `TestCharacterAPIPutAndDelete` | pass |
| FR-11/12 | enrichment | `SRDAPIClient`, `EnrichCharacter` | automated | `TestSRDAPIClient*`, `TestEnrichCharacter*`, `TestEnrichmentEndpoints` | pass |
| FR-13 | idempotent create | `CharacterService.Create` (`created` map) | automated + manual | `TestServiceIdempotencyKey`, `TestConcurrentRetriesCreateOnce`, `TestCreateWithIdempotencyKey`, M-03 | pass |
| FR-14 | health | `Server.health` | automated + manual | `TestHealthAndInstanceHeader`, M-01 | pass |
| NFR-01 | modularity | package structure | automated | `TestDependencyDirection` | pass |
| NFR-02/03 | analysability, duplication | refactoring | tool | `tools/metrics`, SonarQube | met |
| NFR-04 | coverage ≥ 80% | – | tool | `go test -coverpkg` | 87.4% |
| NFR-05/06 | no races, no lost updates | mutexes | automated | `-race`, concurrency tests | pass |
| NFR-08 | availability of reads | nginx + 2 readers | manual | M-05, M-06 | pass |
| NFR-11/12 | security | non-root, read-only volume, no secrets | manual + SonarQube | M-07, M-09, scan | met |
| NFR-14 | CLI compatibility | – | golden transcript | §6 | met (documented differences) |

## 4. Commands

Run from the project folder (`mpp-zmyslek/`).

```sh
gofmt -l .                     # formatting: prints nothing when everything is formatted
go vet ./...                   # static checks
go build ./...                 # compile
go test ./tests/               # all automated tests (about 15 s)

# coverage of the in-process packages
go test ./tests/ -coverpkg=./internal/...,./web/... -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -1

# coverage of main.go and the metrics tool, measured while the CLI tests run the built programs
mkdir -p .cover/cli
MPP_CLI_COVERDIR="$PWD/.cover/cli" go test ./tests/ -coverpkg=./internal/...,./web/... -coverprofile=coverage.out
go tool covdata textfmt -i=.cover/cli -o=coverage-cli.out

go run ./tools/metrics         # size, complexity, duplication
```

Race detector (needs cgo; on a machine without a C compiler run it in Docker,
PowerShell syntax shown):

```powershell
docker run --rm -v "${PWD}:/src" -w /src golang:1.24 go test -race ./...
```

staticcheck (the 2025.1.1 release supports up to Go 1.24, so run it in the
same container):

```powershell
docker run --rm -v "${PWD}:/src" -w /src golang:1.24 sh -c "go install honnef.co/go/tools/cmd/staticcheck@2025.1.1 && staticcheck ./..."
```

### SonarQube

`sonar-project.properties` holds the project key, source/test layout,
exclusions and the coverage reports. The token is never stored; pass it as
`SONAR_TOKEN`. Versions are pinned (SonarQube Community Build
26.8.0.126808, scanner CLI 12.1.0.3233_8.0.1) instead of `latest`.

```powershell
# 1. server in its own Docker network
docker network create sonarnet
docker run -d --name sonarqube --network sonarnet -p 9000:9000 sonarqube:26.8.0.126808-community
# 2. open http://localhost:9000 (admin/admin, change the password), create a token
# 3. create both coverage files (commands above), then scan.
#    The scanner runs in the same Docker network, so it reaches the server as
#    http://sonarqube:9000. "localhost" inside the scanner container would be
#    the scanner itself. From the host (e.g. a locally installed scanner) use
#    http://localhost:9000 instead.
docker run --rm --network sonarnet -e SONAR_HOST_URL=http://sonarqube:9000 -e SONAR_TOKEN=<token> -v "${PWD}:/usr/src" sonarsource/sonar-scanner-cli:12.1.0.3233_8.0.1
```

## 5. Results (actually run on 2026-10-15)

| Check | Result |
|---|---|
| `gofmt -l .` | no files listed |
| `go vet ./...` | no findings (Go 1.27.1 locally, Go 1.24.13 in Docker) |
| `go test ./tests/` | ok, 47 tests |
| `go test -race ./...` (Docker, Go 1.24.13) | ok, no data races |
| `staticcheck ./...` 2025.1.1 (Docker) | exit code 0, no findings |
| Coverage (`internal/`, `web/`) | 87.4% of statements |
| Coverage of the built CLI binary (`main.go`) | 83.6% |
| SonarQube quality gate (default "Sonar way") | **Passed**: coverage on new code 91.2%, duplication on new code 0.0%, 0 new issues |
| SonarQube overall (final scan) | coverage 88.3%, duplication 0.0%, 0 bugs, 0 vulnerabilities, 0 security hotspots; maintainability, reliability and security rating A |
| SonarQube remaining issues | 30 code smells, all in `web/templates/charactersheet.html` (the character sheet page supplied with the assignment): nested ternaries and intentionally empty `catch` blocks in its script, `<label>` elements used as headings without a form control. First scan: 153 bugs (unlabelled inputs, an undefined click handler) – fixed |
| Go code in SonarQube | 0 bugs, 0 vulnerabilities, 0 code smells (after the clean-up; the first scan reported 16 Go code smells, all fixed) |
| Docker Compose demo | all containers healthy; see MANUAL_TEST_PLAN.md |

## 6. CLI compatibility (characterisation test)

The original binary and the new binary were run through the same 3446-line
script (create every class at levels 1, 2, 3, 5, 9, 11, 17, 20; every race;
all equip/spell/error paths; all characters in the committed
`characters.json`). Differences, all intended:

| Difference | Reason |
|---|---|
| Spell slots for full casters at levels 3–6 and 12–19 | the old table was shifted; now equal to the SRD 5.1 table |
| Cantrips known for druids (2/3/4) and sorcerers (4/5/6) | SRD 5.1 class tables |
| Passive perception +proficiency when proficient in Perception | SRD "passive Wisdom (Perception)" |
| Padded armor AC 11 + Dex; monk loses Unarmored Defense with a shield | SRD armor table / Monk feature |
| Warlocks may learn spells below their Pact Magic slot level | Pact Magic slots cast lower-level spells |
| `create` rejects level outside 1–20 and scores outside 0–30 | aggregate invariants |
| usage text lists the new `level` command | new feature |

Everything else, including all messages and exit codes, is byte-identical.
These number changes are visible to the automated grader; they are
corrections towards the SRD, which the assignment requires.

## 7. How the evidence supports maintainability and reliability

* The architecture test turns the dependency rule into a failing test, so
  modularity cannot silently erode.
* Tests run against real files and the real CSV data, so persistence and
  rule behaviour are verified, not mocked.
* Concurrency tests assert the final state (all spells present, exactly one
  creation), and the race detector checks the same code for unsynchronised
  access.
* Complexity and duplication went down while coverage went up, measured with
  the same tool before and after.
