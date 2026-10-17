# Domain-Driven Design

## 1. The business problem

Creating a D&D 5e character by hand means applying many SRD 5.1 rules
correctly: racial ability bonuses, skill choices, proficiency by level, armor
class formulas, which classes learn or prepare spells, and how many spell
slots each class gets at each level. The application keeps characters valid
according to those rules and calculates the derived numbers of the sheet, so
the player cannot create an "impossible" character by accident.

## 2. Strategic design

| Subdomain | Type | Why | Implementation |
|---|---|---|---|
| **Character sheet** – building characters and keeping them valid | Core | This is what the assignment is about and where the rules are | `internal/domain/character.go`, `rules.go`, `internal/application` |
| **SRD reference catalogue** – races, classes, equipment, spells | Supporting | Needed by the core, but it is just lookup data supplied with the assignment | `data/*.csv`, `internal/infrastructure/csv_*_repository.go` |
| **External SRD details** – school, range, weapon category, armor class | Generic | Off-the-shelf public API | `internal/infrastructure/srd_api_client.go` |

**Bounded contexts.** The project is small, so it is one deployable unit and
one Go module. Inside it, the three subdomains above act as three bounded
contexts with their own language for the same words (an "item" is a CSV row
`name,type` in the catalogue, but a weapon *in a slot* in the character
sheet). The relationships are drawn in the context map
([DIAGRAMS.md §6](DIAGRAMS.md#6-context-map)):

* Catalogue → Character sheet: **Conformist**. The sheet stores catalogue
  names exactly as they are and asks the catalogue to validate them.
* SRD API → Character sheet: **Open host service** consumed as-is. The API's
  JSON is passed through to the web page without translation (no
  anti-corruption layer). This is a deliberate simplification; see the leaky
  abstractions review in [DESIGN_PATTERNS.md](DESIGN_PATTERNS.md).

Splitting the contexts into separate modules or services would add
complexity without any benefit at this size.

## 3. Aggregate: Character

| Aspect | Description |
|---|---|
| Root | `domain.Character` (`internal/domain/character.go`) |
| Identity | the character **name**, unique in the store (natural key) |
| Owned parts | `AbilityScores`, skill proficiencies, the four equipment slots, known spells, prepared spells |
| Referenced, not owned | classes, race bonuses, equipment and spells from the catalogue (stored by name only) |
| Repository | `domain.CharacterRepository` → `infrastructure.JSONCharacterRepository` (the only repository that saves; the CSV repositories are read-only) |
| Persistence boundary | one JSON object per character holding all owned parts; saved and loaded as a whole |
| Transaction boundary | one command changes one character, inside `CharacterService.mu` |

**Invariants and where they are enforced**

| # | Invariant | Enforced in | Test |
|---|---|---|---|
| I1 | A character has a name, race and class | `NewCharacter` | `TestNewCharacterValidates` |
| I2 | Level is 1–20 and never decreases | `NewCharacter`, `ChangeLevel` | `TestChangeLevel` |
| I3 | Ability scores stay within 0–30 (also after racial bonuses) | `NewCharacter`, `ApplyAbilityBonus` | `TestNewCharacterValidates`, `TestApplyAbilityBonus` |
| I4 | Weapons only go into a **free** main or off hand; armor and shields only into their own slot | `EquipWeapon`, `EquipArmor`, `EquipShield` | `TestEquipment` |
| I5 | A spell must be on the class list and castable at the current level; prepared casters prepare, others learn; classes without cantrips (paladin, ranger) get none; a spell is stored once. This also holds for spells sent at creation and on the web sheet, which reads the rules from `SpellcastingFor` instead of keeping its own tables | `CanAddSpell`, `addSpell`, `SpellcastingFor` | `TestSpells`, `TestSpellcastingFor`, `TestServiceCreateWithSpells`, `TestSheetSpellcastingComesFromTheDomain` |
| I6 | Spells stay castable after a level change | follows from I2 (slot levels never shrink as level rises) | `TestSpellSlotsMatchTheSRD` |
| I7 | Race, class and ability scores do not change after creation | `CharacterService.Update` | `TestServiceUpdate` |
| I8 | Names are unique (API) | `CharacterService.Create` with `RejectDuplicate` | `TestServiceCreate`, `TestCharactersAPI` |

**Why outside code cannot bypass the root.** All fields of `Character` are
unexported. The getters for slices return copies (`slices.Clone`), and
`Data()` returns a copy, so changing a returned value never changes the
character (`TestGettersReturnCopies`). The only way to change a character is
through its methods.

**Lifecycle.** Created by `CharacterService.Create` (via `NewCharacter` plus
the creation rules in `buildCharacter`), changed only by commands, removed by
`Delete`. `RestoreCharacter` rebuilds a stored character *without* running
the creation rules again (racial bonuses must not be applied twice).

**Is this really an aggregate?** Yes in the DDD sense: the spell lists, slots
and level must change together (I5/I6), so they belong to one consistency
boundary with one root. There is only one aggregate; equipment and spells are
not aggregates in this application because nothing ever changes them.

## 4. Tactical building blocks

| Pattern | In this project | Notes |
|---|---|---|
| Entity | `Character` | identity = name; mutable state behind methods |
| Value object | `AbilityScores`, `RaceAbilityBonus` | compared by value (`==` works on `AbilityScores`), copied, no identity |
| Reference data objects | `Equipment`, `Spell`, `Class` | immutable after construction (unexported fields, getters only); identified by name in the catalogue. From the character's point of view they behave like values |
| Factory | `NewCharacter` (validation) + `CharacterService.buildCharacter` (applies racial bonuses, class and background skills once) | creation needs reference data from repositories, which is why part of it sits in the application service |
| Repository | `CharacterRepository` for the aggregate; read-only repositories for reference data | interfaces in `domain`, as in the course example |
| Domain logic as pure functions | `rules.go` (`AbilityModifier`, `spellSlots`, `armorClass`, ...) | stateless rules; a separate "domain service" type would add nothing |
| Application services | `CharacterService` (commands), `CharacterQueries` (queries) | orchestration only; no business rule is duplicated here except I7/I8, which concern the whole store or the edit workflow |

**Patterns deliberately not used**

* **Domain events**: no part of the system reacts to "character changed"
  (no notifications, no read model to update). Adding events would be code
  with no consumer.
* **Separate domain service types**: the SRD formulas need no state or
  dependencies, so plain functions are simpler.
* **Multiple aggregates / sagas**: no business operation changes two
  characters at once.

## 5. Entities and value objects in detail

**Character (entity)** – a player's character. Identity: its name (two
characters with the same scores but different names are different
characters; renaming is not supported). Lifecycle: created → modified (level,
equipment, spells) → deleted. Persisted as one object in `characters.json`,
loaded with `RestoreCharacter`.

**AbilityScores (value object)** – the six scores Str, Dex, Con, Int, Wis,
Cha. Defined entirely by its six numbers; equality is `==`. Validation
(0–30) happens when a character is created (`AbilityScores.valid`).
Immutable by design in the sense that it is always passed **by value**: the
character returns a copy from `Scores()`, so a caller can change its copy but
never the character's scores. It is not an entity: "a set of 15, 14, 13, 12,
10, 8" has no identity of its own. Behaviour: `HasStandardArray`.

**RaceAbilityBonus (value object)** – "+2 Dexterity". Read from the CSV,
applied once at creation, never stored separately.

**CharacterData** – not a value object but a *snapshot* (memento) used to
move state between the aggregate and the repository or the web layer.

## 6. Persistence decision

The existing project stored characters in a JSON file, and the grader runs
the CLI against the committed `characters.json`. Replacing it with a database
would add a dependency (the grader compiles on one CPU core within two
minutes) and risk the CLI contract. The file store was therefore kept and
hardened (mutex, atomic rename, legacy-format reading). The price is that
safety against concurrent writers only holds inside one process, which the
deployment handles with a single writer (see
[ARCHITECTURE.md §5](ARCHITECTURE.md#5-concurrency-transactions-and-consistency)).

## 7. Assumptions that need confirmation

| # | Assumption | Reason |
|---|---|---|
| A1 | Level never decreases | The SRD has no level loss for player characters, and it keeps invariant I6 simple. Correcting a typo requires delete + create |
| A2 | The CLI keeps accepting any scores 0–30 and replaces an existing name on `create` | Original CLI behaviour that the automated grader may depend on; the web API is strict |
| A3 | SRD 5.1 tables are authoritative where the old code differed (spell slots, cantrips, padded armor, monk + shield, warlock Pact Magic, passive perception with proficiency) | The assignment requires the SRD 5.1 rules |
| A4 | Duplicate skill proficiencies are kept | The assignment says to ignore that problem |
