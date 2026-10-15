# Assessment traceability

For each checklist item: what exists, where, why, how it is verified, its
limits, and how confident the evidence makes us. Paths are relative to
`mpp-zmyslek/`. **Status**: Full = implemented and verified; Partial =
implemented with a documented gap; Not implemented = explained.

| # | Requirement | Implementation (files, symbols) | Rationale | Verification | Docs | Limitations | Status / confidence |
|---|---|---|---|---|---|---|---|
| 3.1 | Encapsulation | `internal/domain/character.go`: unexported fields, `NewCharacter`, behaviour methods (`ChangeLevel`, `EquipWeapon`, `LearnSpell`, ...), slice getters return copies | Callers can only change a character through rule-checking operations | `TestNewCharacterValidates`, `TestGettersReturnCopies`, `TestEquipment`, `TestSpells` | GO_CONCEPTS §1, DDD §3 | `RestoreCharacter` accepts stored data without re-validating (trusted storage) | Full / high |
| 3.2 | Pointers | pointer receivers on `*Character`, services and repositories (mutex inside); value receiver on `AbilityScores`; `&results[i]` per goroutine | Identity + mutation vs. copyable value; mutex must not be copied | `go vet` (copylocks), `-race`, `TestGettersReturnCopies` | GO_CONCEPTS §2 | – | Full / high |
| 3.3 | Aggregates | `Character` = root; owns scores, skills, slots, spell lists; references catalogue items by name; `CharacterRepository` per root | Level, slots and spell lists must stay consistent together | `TestSpells`, `TestChangeLevel`, `TestServiceUpdate` | DDD §3, DIAGRAMS §5/§7 | "fixed after creation" (race/class/scores) is checked in `CharacterService.Update`, not in the root | Full / high |
| 3.4 | DDD | Strategic: 3 subdomains/contexts + context map. Tactical: entity, value objects, aggregate, repositories, factory, application services | Rules of the SRD are the core | as 3.1–3.3 + `TestDependencyDirection` | DDD.md, DOMAIN_GLOSSARY.md, DIAGRAMS §6 | Contexts are packages, not separate modules; **domain events not implemented** (no consumer) | Partial (justified) / high |
| 3.5 | Interfaces and repositories | `domain.CharacterRepository`, `ClassRepository`, `RaceAbilityRepository`, `EquipmentRepository`, `SpellRepository`; consumer-side `application.CharacterReader`, `ResourceLookup`; implementations in `internal/infrastructure` | Dependency inversion; testability with fakes (`fakeLookup`) | `TestDependencyDirection`, `TestQueriesDoNotChangeStoredData`, `TestEnrichCharacter*` | ARCHITECTURE §2, DESIGN_PATTERNS | Repository interfaces live in `domain` (course example style); two consumer-side interfaces in `application` | Full / high |
| 3.6 | Load balancing and DNS | `docker-compose.yml` (lb, writer, 2 readers), `deploy/nginx.conf` (method-based pools, round-robin, passive health checks, `zone`), `/healthz`, Docker `HEALTHCHECK` | Reads scale out; writes need one process (in-process locks) | M-01 – M-06, M-10 (executed) | ARCHITECTURE §7, DIAGRAMS §3 | Local demo only: nginx resolves names at start, one lb, no TLS, Docker DNS ≠ production discovery | Full (demo) / high |
| 3.7 | Asynchronous processing | `CharacterQueries.EnrichCharacter` (goroutine per lookup, `WaitGroup`, ctx timeout 30 s, per-item errors); `serve` goroutine + graceful `Shutdown` | Independent slow lookups; clean stop | `TestEnrichCharacterRunsLookupsConcurrently`, `TestEnrichCharacterStopsWhenCancelled`, M-08 | ARCHITECTURE §6 | No background workers or persisted jobs (not needed); awaited, not retried | Full / high |
| 3.8 | Concurrency | `CharacterService.mu`, `JSONCharacterRepository.mu`, `SRDAPIClient.mu`; ownership of slice elements in enrichment | Prevent lost updates and races on shared state | `TestConcurrentChangesAreNotLost`, `TestConcurrentRetriesCreateOnce`, `TestSRDAPIClientRateLimit`; `go test -race` clean | ARCHITECTURE §5 | Protection is per process; two processes writing at once can lose an update | Full in-process / high; Partial across processes |
| 3.9 | Idempotency | `Idempotency-Key` on `POST /api/characters` (`CharacterService.Create`, `created` map); natural idempotency: unique name, adding a spell twice, PUT of a whole sheet, `ChangeLevel` to the same level | Clients retry after timeouts | `TestServiceIdempotencyKey`, `TestConcurrentRetriesCreateOnce`, `TestCreateWithIdempotencyKey`, M-03 | REQUIREMENTS FR-13, ARCHITECTURE §8 | Keys are in memory: lost on restart (the unique name still prevents a duplicate → 409), no expiry, not stored atomically with the character | Partial / high |
| 3.10 | Stateless design | Readers hold no session state; durable state in the volume; writer deliberately stateful | Any reader can serve any read | M-04, M-05, M-11 | ARCHITECTURE §8 | Writer instance is a single stateful component; rate limiter is per instance | Partial (by design) / high |
| 3.11 | ISO/IEC 25010 and maintainability | Assessment table with baseline and final measurements | Evidence-based quality claims | `tools/metrics`, coverage, SonarQube | QUALITY_AND_TESTING §1–2 | Standard text not consulted directly; targets are our own | Full / medium-high |
| 3.12 | Heap and stack | Escape-analysis output with explanation | – | `go build -gcflags=-m` (reproducible) | GO_CONCEPTS §3 | – | Full / high |
| 3.13 | CQRS | `CharacterService` (commands) / `CharacterQueries` (queries); read-only `CharacterReader` | Reads can't change state; clear use-case list | `TestQueriesDoNotChangeStoredData` | DESIGN_PATTERNS (CQRS) | Shared model and storage (logical separation only) | Full (logical) / high |
| 3.14 | API and local database with race protection | JSON API with status codes 200/201/400/404/409/502; JSON file store with mutex + atomic rename; legacy format reading (`legacyName`); unique names; env configuration | Kept the existing JSON storage (user decision; grader constraints) | `TestCharactersAPI`, `TestCharacterAPIPutAndDelete`, `TestJSONRepositoryFiles`, concurrency tests | ARCHITECTURE §4–5, DDD §6, DIAGRAMS §7 | **No real database**: no schema migrations beyond reading the old format, no DB-level constraints or indexes, transactions = in-process lock; no optimistic locking for web edits (last save wins); requests decode into `domain.CharacterData` (leak L5) | Partial / high |
| 3.15 | SonarQube, static analysis, testing | `sonar-project.properties`; scan run with pinned versions; `go vet`, `staticcheck`; 47 automated tests; manual plan | – | Quality gate passed; results in QUALITY_AND_TESTING §5 | QUALITY_AND_TESTING, MANUAL_TEST_PLAN | Browser tests B-01 – B-08 not executed; remaining Sonar findings are in the supplied HTML template | Full / high |
| 3.16 | Docker | `Dockerfile` (multi-stage, static binary, non-root, healthcheck), `.dockerignore`, `docker-compose.yml` (volume, health-based startup order) | Reproducible local environment | M-01 – M-11 | ARCHITECTURE §7, README | Images pinned by tag, not digest | Full / high |
| 4 | Functional and imperative programming | pure functions in `rules.go`; imperative services, I/O and handlers | Calculation separated from effects | table tests on the pure functions | GO_CONCEPTS §4 | – | Full / high |
| 5 | Entities and value objects | `Character` (entity), `AbilityScores`, `RaceAbilityBonus` (value objects), catalogue reference objects | – | `TestHasStandardArray`, `TestGettersReturnCopies` | DDD §4–5 | `AbilityScores` has exported fields (immutability by copying, not by hiding) | Full / high |
| 6.1 | Architecture overview | – | – | – | DIAGRAMS §1, ARCHITECTURE | – | Full |
| 6.2 | C4 diagrams | context, container, component | – | matches `docker-compose.yml` and packages | DIAGRAMS §2–4 | Mermaid flowcharts with C4 labels, not the C4-PlantUML notation | Full |
| 6.3 | ER diagram | JSON document + CSV tables | – | – | DIAGRAMS §7 | No FKs/indexes exist (file store) | Full |
| 6.4 | Domain model and bounded contexts | – | – | – | DDD, DIAGRAMS §5–6 | – | Full |
| 6.5 | Ubiquitous language | glossary + naming issues | – | – | DOMAIN_GLOSSARY | – | Full |
| 6.6 | FRs and NFRs | 14 FRs, 15 NFRs with tests | – | traceability matrix | REQUIREMENTS, QUALITY_AND_TESTING §3 | Some NFR targets are proposed, not agreed | Full |
| 6.7 | Design patterns | only patterns present in the code, with costs | – | – | DESIGN_PATTERNS | – | Full |
| 6.8 | Leaky abstractions | 4 fixed, 5 accepted with reasons | – | `TestDependencyDirection` | DESIGN_PATTERNS (review) | L5–L9 remain | Full |
| 6.9 | Determinism | analysis per level | – | tests avoid real time/network except lower-bound timing | GO_CONCEPTS §5 | No injectable clock (time is only used by the rate limiter) | Full / medium-high |
| 6.10 | IEEE/ISO standards | 5 standards, applied practices, no conformance claim | – | – | STANDARDS | Editions from catalogue pages | Full |
| 7 | Testing strategy and evidence | strategy, matrix, commands, results | – | – | QUALITY_AND_TESTING, MANUAL_TEST_PLAN | see 3.15 | Full |

## Handover report

### What existed before

A working Go program (standard library only): a CLI and a web UI/JSON API for
SRD 5.1 characters, storing them in `characters.json`. `internal/domain` held
a `Character` struct with exported fields and JSON tags; the workflows (racial
bonuses, skills, equip and spell checks) lived inside a 238-line `main()` and
inside `web/server.go`, which also called the external API directly. An
untracked `tests/` folder had 2 failing tests (they expected the CLI to
enforce the Standard Array). Several SRD tables were wrong (see
QUALITY_AND_TESTING §6). There was no Docker, no SonarQube setup and no
architecture documentation.

### What changed

* **Domain**: `Character` encapsulated (unexported fields, constructor,
  behaviour methods, copy-returning getters); SRD rules moved to pure
  functions in `rules.go` and corrected to the SRD tables; `Equipment`,
  `Spell`, `Class` made immutable; JSON decoding moved out of the domain.
* **Application** (new): `CharacterService` (commands, mutex, idempotency
  keys) and `CharacterQueries` (queries, concurrent enrichment).
* **Infrastructure**: JSON repository with mutex and atomic writes;
  `SRDAPIClient` (rate limited, context-aware); unused `CSVRaceRepository`
  removed.
* **CLI** (`main.go`): one function per command, same messages and exit codes,
  new `level` command, environment configuration, graceful shutdown.
* **Web**: uses the services; added `GET /api/characters/{name}`,
  `/api/characters/{name}/enrichment`, `/healthz`, `X-Instance`,
  `Idempotency-Key`; unused templates and `view.go` removed; template inputs
  given `id`s/labels for accessibility; an undefined click handler removed.
* **Tests**: all in `tests/`; 47 tests including concurrency, idempotency,
  architecture and HTTP contract tests; the two old failing tests updated.
* **Added**: `Dockerfile`, `docker-compose.yml`, `deploy/nginx.conf`,
  `.dockerignore`, `.gitignore`, `sonar-project.properties`, `docs/`.

### Checks actually run (2026-10-15)

`gofmt`, `go vet`, `go build`, `go test ./tests/` (47 pass),
`go test -race ./...` in `golang:1.24` (no races), `staticcheck` 2025.1.1
(no findings), coverage 87.4%, SonarQube 26.8 scan (gate passed; 0 bugs,
0 vulnerabilities, 30 code smells, all in the supplied HTML template), Docker Compose demo M-01 – M-11 (all pass), CLI manual tests
M-12 – M-17 (all pass), golden CLI transcript comparison. Browser tests were
**not** executed.

### Known defects and technical debt

1. Writes are only safe inside one process (CLI and server writing the same
   file at the same moment could lose an update).
2. Idempotency keys live in memory: no expiry, lost on restart.
3. No optimistic locking for web edits: two tabs saving the same character →
   last save wins.
4. `charactersheet.html` JavaScript re-implements some SRD formulas for live
   preview, and has remaining SonarQube code smells.
5. Enrichment passes the external JSON through unchanged.
6. Storage error messages can include the server's file path.
7. The SRD corrections change numbers that the automated grader sees
   (deliberate, listed in QUALITY_AND_TESTING §6).

### Commands

```sh
go run . serve                         # web UI on http://localhost:8080
go run . list                          # CLI (see `go run .` for all commands)
go test ./tests/                       # tests
docker run --rm -v "${PWD}:/src" -w /src golang:1.24 go test -race ./...   # race detector
docker compose up -d --build           # load-balanced demo on http://localhost:8080
docker compose down -v                 # stop and remove data
```

SonarQube: see [QUALITY_AND_TESTING.md §4](QUALITY_AND_TESTING.md#sonarqube).
