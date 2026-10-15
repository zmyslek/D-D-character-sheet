# Design patterns

Only patterns that exist in the code are listed. Architectural *styles*
(layering) and language features (interfaces, goroutines) are named as such.
General explanations: [Design Gurus](https://www.designgurus.io/).

## Architectural style: layered architecture with dependency inversion

| | |
|---|---|
| Purpose | Keep business rules independent of files, HTTP and the CLI |
| Where | `internal/domain` ← `internal/application` ← `web`, `main.go`; `internal/infrastructure` implements interfaces of the inner layers |
| Why appropriate | The same rules serve two user interfaces (CLI and web) and two storages (JSON file, CSV files); the course example uses the same layout |
| Cost | More files and an extra hop (service) for simple reads |
| Tested by | `TestDependencyDirection` |

It is close to *ports and adapters* (the interfaces are the ports, the CSV/JSON
repositories, the API client, `web` and `main.go` are adapters), but the
project does not use that vocabulary in its package names.

## Repository

| | |
|---|---|
| Purpose | Hide how and where objects are stored behind a collection-like interface |
| Problem solved | The original `main.go` and `web` read files directly; now both use the same interface |
| Where | interfaces: `domain.CharacterRepository`, `ClassRepository`, `RaceAbilityRepository`, `EquipmentRepository`, `SpellRepository`, `application.CharacterReader` (read-only view). Implementations: `JSONCharacterRepository`, `CSVClassRepository`, `CSVRaceAbilityRepository`, `CSVEquipmentRepository`, `CSVSpellsRepository` |
| Why appropriate | One repository per aggregate root (`Character`) plus read-only repositories for reference data |
| Cost | Every query loads the whole file (acceptable for tens of characters) |
| Tested by | `TestJSONRepositoryFiles`, `TestCSVRepositoriesWithRealData`, all service tests (real repositories on temporary files) |

## Dependency injection (constructor injection)

`NewCharacterService(...)`, `NewCharacterQueries(...)` and `web.NewServer(...)`
receive their collaborators; `main.go` (`newApp`) is the only place that
creates concrete adapters (the *composition root*). Tests inject temporary
files and a fake lookup (`fakeLookup` in `tests/queries_test.go`). Cost: the
wiring is written by hand. No DI framework is needed.

## Adapter

`SRDAPIClient` adapts the external HTTP API to the `application.ResourceLookup`
interface; the CSV and JSON repositories adapt files to the repository
interfaces. Tested with an `httptest` fake server (`TestSRDAPIClient`).

## Command–query separation (CQRS, logical)

| | |
|---|---|
| Purpose | Separate operations that change state from operations that read it |
| Where | `CharacterService` = commands (Create, ChangeLevel, Equip*, AddSpell, Update, Delete); `CharacterQueries` = queries (GetAll, Get, Lookup, EnrichCharacter) |
| How queries are kept read-only | `CharacterQueries` depends on `CharacterReader`, which has no write methods; a query returns a freshly loaded character, so changing it saves nothing (`TestQueriesDoNotChangeStoredData`) |
| Read/write model | **Shared**: the same `Character` model and the same `characters.json`. Consistency is immediate (a query after a command sees the change) |
| Why not separate storage | No read is expensive enough to justify a projection; a second store would bring eventual consistency for no gain |
| Cost | Two service types instead of one |

## Factory (simple factory functions)

`NewCharacter` validates the invariants of a new character;
`CharacterService.buildCharacter` adds the creation steps that need
repositories (racial bonuses, class skills, background skills). Constructors
`NewEquipment`, `NewSpell`, `NewClass` keep fields unexported. A Factory
*class* would add nothing.

## Memento (snapshot)

`Character.Data()` returns a `CharacterData` copy; `RestoreCharacter` rebuilds
a character from it without re-running creation rules. Used by the JSON
repository and the web layer. Cost: the field list exists twice
(`Character` and `CharacterData`).

## Decorator / middleware

`Server.Handler` wraps the router in a handler that adds `X-Instance` to every
response (`TestHealthAndInstanceHeader`).

## Concurrency patterns

* **Monitor (mutex-guarded object)**: `CharacterService.mu`,
  `JSONCharacterRepository.mu`, `SRDAPIClient.mu`.
* **Fan-out / fan-in**: `EnrichCharacter` starts one goroutine per lookup and
  waits with a `WaitGroup`.
* **Idempotent receiver**: `Idempotency-Key` handling in `CharacterService.Create`.

## Not used, and why

| Pattern | Reason |
|---|---|
| Unit of Work | Each command saves exactly one aggregate |
| Domain events / observer | No component reacts to changes |
| Strategy | Each rule has exactly one SRD formula |
| Circuit breaker | The only external call is optional enrichment, already bounded by the rate limiter and timeouts |

## Leaky abstractions review

| # | Finding | Impact | Decision |
|---|---|---|---|
| L1 | *Fixed*: the domain `Character` had JSON tags and a custom `UnmarshalJSON` for the old file format | Storage format changes would have touched the domain | Moved to `characterRecord` / `legacyName` in `json_character_repository.go`; domain has no JSON import (`TestDependencyDirection`) |
| L2 | *Fixed*: `web/server.go` called the external API itself (HTTP client, rate limiter inside the web handler) | Transport layer owned an infrastructure concern | Moved to `infrastructure.SRDAPIClient` behind `application.ResourceLookup` |
| L3 | *Fixed*: `main.go` contained the business workflow (racial bonuses, skill selection, spell checks) | Rules duplicated between CLI and web | Moved to `CharacterService` and `Character` methods |
| L4 | *Fixed*: errors were matched by text | Fragile | `errors.Is` with `domain.ErrNotFound`, `ErrNotAWeapon`, ... |
| L5 | *Accepted*: the web layer decodes requests into `domain.CharacterData` and returns it as JSON | The API contract is tied to the snapshot's field names | Field names equal the original API contract, and `CharacterData` has no behaviour; a separate DTO would duplicate 18 fields. A field change must keep the API in mind |
| L6 | *Accepted*: enrichment returns the external API's JSON unchanged (`data`) | An upstream schema change reaches the browser script | The page script already depends on that shape; translating it would require rewriting the provided page |
| L7 | *Accepted*: storage errors (e.g. corrupt file) are returned with their message, which can include the file path (500) | Minor information leak | Inherited from the original program; acceptable for a local tool |
| L8 | *Documented*: the file store's locking is in-process only | An abstraction (`CharacterRepository`) could suggest database-like guarantees | Stated in the type comment and ARCHITECTURE.md; Docker uses a single writer |
| L9 | *Accepted*: the CSV repositories re-read their file on every lookup | Hidden I/O cost | Files are tiny; caching would add shared mutable state |
