# Go concepts in this project

Encapsulation, pointers, stack and heap, programming paradigms and
determinism, with references to the code. Compiler output below was produced
with Go 1.27.1 (`go version`), on 2026-10-15.

## 1. Encapsulation

* All fields of `domain.Character`, `Equipment`, `Spell` and `Class` are
  unexported (lowercase); other packages use constructors (`NewCharacter`,
  `NewEquipment`, ...) and methods.
* There are **no setters**. State changes are business operations that check
  rules: `ChangeLevel`, `EquipWeapon`, `EquipArmor`, `EquipShield`,
  `LearnSpell`, `PrepareSpell`, `ApplyAbilityBonus`.
* Getters exist where a caller needs to *read* a value (printing a sheet,
  saving a character). Slice getters return copies so callers cannot modify
  internal lists:

  ```go
  // internal/domain/character.go
  func (character *Character) KnownSpells() []string { return slices.Clone(character.knownSpells) }
  ```

* The protected invariants are listed in [DDD.md §3](DDD.md#3-aggregate-character);
  `TestGettersReturnCopies` shows that changing returned data does not change
  the character.

## 2. Pointers

| Where | Pointer or value | Why |
|---|---|---|
| `*Character` (all methods use pointer receivers) | pointer | An entity with identity whose methods change it; a copy would silently fork its state |
| `AbilityScores` (value receiver `HasStandardArray`, returned by value from `Scores()`) | value | A value object: 6 ints (48 bytes), cheap to copy, and copying is what makes it safe to hand out |
| `*Equipment`, `*Spell`, `*Class` | pointer | Returned by constructors and repositories like the course example; never mutated after construction, so sharing is safe |
| `*CharacterService`, `*JSONCharacterRepository`, `*SRDAPIClient` | pointer | They contain a `sync.Mutex`, which must not be copied (`go vet` checks this) and must be shared by all callers |
| `ApplyAbilityBonus` uses `score := &character.scores.Str` | pointer to a field | Chooses which field to change once, then updates it; the pointer never leaves the method |
| `addSpell` uses `list := &character.knownSpells` | pointer to a slice header | The same append code updates either list |
| `EnrichCharacter` passes `&results[index]` to each goroutine | pointer to one element | Each goroutine writes only its own element → no data race and no lock |

**Pointer vs. value receivers.** A pointer receiver (`func (c *Character)`)
works on the original; a value receiver (`func (s AbilityScores)`) works on a
copy, so it cannot change the caller's value.

**Nil handling.** Constructors either return a valid pointer or an error
(`NewCharacter`), so callers never get a nil character with a nil error.
Repositories return `nil, err` on failure and callers check the error first.
`CharacterQueries` is created with a nil `ResourceLookup` only by code paths
that never call `Lookup` (the CLI).

**Aliasing.** `RestoreCharacter` and `Data()` clone slices, so a
`CharacterData` and the character never share a backing array.

## 3. Stack and heap (escape analysis)

Go decides where a value lives. If the compiler can prove a value is not
used after the function returns, it can stay on the **stack** (freed
automatically when the function returns, no garbage-collector work).
Otherwise it **escapes** to the **heap**, and the garbage collector frees it
when nothing references it any more. The source code alone does not decide
this: `&T{}` may stay on the stack, and a plain value may escape.

Reproduce with:

```sh
go build -gcflags=-m ./internal/domain/ ./internal/application/ ./internal/infrastructure/
```

Selected real output (Go 1.27.1):

```text
internal/domain/character.go:37:17: []int{...} does not escape          # HasStandardArray: temporary slice on the stack
internal/domain/character.go:39:35: []int{...} does not escape
internal/domain/character.go:88:9: &Character{...} escapes to heap       # NewCharacter returns a pointer: must outlive the call
internal/domain/character.go:77:25: &errors.errorString{...} escapes to heap   # returned error values
internal/domain/character.go:117:31: append escapes to heap              # RestoreCharacter: cloned slices are stored in the character
internal/domain/rules.go:9:6: can inline AbilityModifier                 # small pure function, inlined at call sites
internal/application/character_queries.go:86:6: moved to heap: wg       # WaitGroup is used by goroutines that outlive the loop
internal/application/character_queries.go:89:6: func literal escapes to heap   # the goroutine closure
internal/infrastructure/srd_api_client.go:66:6: moved to heap: document # decoded JSON is returned to the caller
```

**Why it matters here.** A character lives for one request, so its heap
objects become garbage right after the response; Go's concurrent collector
handles this without tuning. The hot pure functions (`AbilityModifier`,
`HasStandardArray`) do not allocate on the heap. Nothing in the project was
changed just to influence allocation: correctness and clarity come first.

## 4. Functional and imperative programming

**Functional style – pure functions** (result depends only on the arguments,
no side effects, same input → same output):

| Function | File | Tested by |
|---|---|---|
| `AbilityModifier`, `ProficiencyBonus` | `internal/domain/rules.go` | `TestAbilityModifier`, `TestProficiencyBonus` |
| `spellSlots`, `pactMagicSlots`, `cantripsKnown`, `highestSpellLevel` | `rules.go` | `TestSpellSlotsMatchTheSRD` |
| `armorClass` | `rules.go` | `TestArmorClass` |
| `AbilityScores.HasStandardArray` (value receiver, works on a copy) | `character.go` | `TestHasStandardArray` |
| `splitValues`, `formatModifier` (CLI), `titleCase`, `statusFor` (web) | `main.go`, `web/server.go` | via CLI and web tests |

The calculation (rules) is separated from side effects: the `Character`
derived values (`ArmorClass`, `SpellSlots`, ...) only *call* these functions;
reading and writing files happens elsewhere. Function values are also used:
the service passes `(*domain.Character).EquipArmor` as a function value to
the shared `equip` helper.

**Imperative style** – where effects and order matter:

* Sequential workflows: `CharacterService.Create` (check key → validate →
  build → save → remember key), `Update` (step by step through the edit).
* State changes: `Character.EquipWeapon`, `ChangeLevel`.
* File I/O: `JSONCharacterRepository.writeAll` (temp file, write, chmod, rename).
* Network I/O: `SRDAPIClient.Lookup`.
* HTTP handling and CLI parsing: `web/server.go`, `main.go`.

Side effects are kept at the edges (infrastructure, `web`, `main.go`) and
behind mutexes; the domain rules in the middle are pure.

## 5. Determinism

| Level | Deterministic? | How |
|---|---|---|
| Business logic | **Yes** | Rules are pure functions; `Character` methods depend only on the character and their arguments |
| Map iteration | handled | `SpellSlots` returns a map; the CLI sorts the keys before printing (`printCharacter`) |
| API responses | Yes for the same stored state | `GetAll` returns file order; enrichment results keep sheet order because each goroutine writes its own index |
| Concurrent execution | **No fixed order** | Which of two simultaneous requests runs first is up to the scheduler. The *result* is still correct: the mutex serialises commands, so both changes are applied (`TestConcurrentChangesAreNotLost`); with the same idempotency key exactly one creation happens (`TestConcurrentRetriesCreateOnce`) |
| Time | partly | The rate limiter uses the real clock; its test only checks a lower bound (≥ 600 ms for 4 requests), which cannot fail on a slow machine |
| Randomness | none used | – |
| External service | not deterministic | Tests use `httptest` fakes and `fakeLookup`; no test contacts dnd5eapi.co |
| Environment | controlled | Defaults for every variable; tests use temporary directories |
| Builds | reproducible inputs | No third-party modules; `go build -trimpath` in Docker; images pinned by version tag (not by digest, so a re-pushed tag could change) |
