# Diagrams

All diagrams are Mermaid (text, version-controlled) and describe the code and
`docker-compose.yml` as they are. Nothing shown here is "planned" unless it is
marked *proposed*.

## 1. Architecture overview (layers and dependency direction)

Arrows mean "depends on / calls". Interfaces are implemented by the
infrastructure package, so the arrows from infrastructure point **into** the
domain and application packages.

```mermaid
flowchart TB
    user([Player])
    grader([Automated grader])
    subgraph adapters_in[Driving adapters]
        cli["main.go<br/>CLI + composition root"]
        web["web/server.go<br/>HTTP routes, templates, JSON"]
    end
    subgraph app[internal/application]
        cmd["CharacterService<br/>(commands, mutex, idempotency keys)"]
        qry["CharacterQueries<br/>(queries, concurrent enrichment)"]
        ports["interfaces: CharacterReader, ResourceLookup"]
    end
    subgraph dom[internal/domain]
        agg["Character (aggregate root)<br/>AbilityScores, Equipment, Spell, Class"]
        rules["rules.go (pure SRD formulas)"]
        repos["interfaces: CharacterRepository, ClassRepository,<br/>RaceAbilityRepository, EquipmentRepository, SpellRepository"]
    end
    subgraph infra[internal/infrastructure]
        json["JSONCharacterRepository"]
        csv["CSV repositories (classes, race bonuses,<br/>equipment, spells)"]
        api["SRDAPIClient (rate limited)"]
    end
    file[("characters.json")]
    data[("data/*.csv")]
    ext[["dnd5eapi.co"]]

    user --> cli & web
    grader --> cli
    cli --> cmd & qry
    web --> cmd & qry
    cmd --> agg & repos
    qry --> ports & agg
    agg --> rules
    json -. implements .-> repos
    json -. implements .-> ports
    csv -. implements .-> repos
    api -. implements .-> ports
    json --> file
    csv --> data
    api --> ext
```

## 2. C4 level 1 – System context

```mermaid
flowchart LR
    player(["<b>Player</b><br/>[Person]<br/>creates and views character sheets"])
    grader(["<b>CodeGrade</b><br/>[External system]<br/>runs the CLI with automated tests"])
    system["<b>Character Sheet Manager</b><br/>[Software system]<br/>stores D&D 5e characters and<br/>calculates their sheets"]
    srd[["<b>D&D 5e SRD API</b><br/>[External system, dnd5eapi.co]<br/>spell and equipment details"]]

    player -- "uses (browser / terminal)" --> system
    grader -- "runs CLI commands" --> system
    system -- "looks up details over HTTPS (max 5 req/s)" --> srd
```

## 3. C4 level 2 – Containers (Docker demonstration)

```mermaid
flowchart TB
    player(["Player<br/>[Person]"])
    subgraph host["Docker host (local demonstration)"]
        lb["<b>Load balancer</b><br/>[Container: nginx 1.27]<br/>port 8080; GET → readers, writes → writer"]
        writer["<b>Writer instance</b><br/>[Container: Go binary 'serve']<br/>all commands; idempotency keys"]
        r1["<b>Reader instance 1</b><br/>[Container: Go binary 'serve']"]
        r2["<b>Reader instance 2</b><br/>[Container: Go binary 'serve']"]
        vol[("<b>characters volume</b><br/>[Docker volume]<br/>characters.json")]
        dns{{"Docker embedded DNS<br/>127.0.0.11"}}
    end
    srd[["D&D 5e SRD API<br/>[External system]"]]

    player -- "HTTP :8080" --> lb
    lb -- "resolves 'reader', 'writer'" --> dns
    lb -- "POST/PUT/DELETE" --> writer
    lb -- "GET/HEAD round-robin" --> r1 & r2
    lb -. "GET backup" .-> writer
    writer -- "read-write" --> vol
    r1 & r2 -- "read-only" --> vol
    writer & r1 & r2 -- "HTTPS" --> srd
```

Without Docker the same binary is one container: `go run . serve` (web) or the
CLI commands, both using `characters.json` in the working directory.

## 4. C4 level 3 – Components of the application container

```mermaid
flowchart TB
    subgraph container["Go application (one instance)"]
        router["HTTP handler<br/>[web.Server.Handler]<br/>routes + X-Instance header"]
        pages["Pages<br/>[index, characterPage, newCharacter]"]
        api["JSON API<br/>[charactersAPI, characterAPI, enrich]"]
        health["Health endpoint<br/>[/healthz]"]
        commands["Command service<br/>[application.CharacterService]"]
        queries["Query service<br/>[application.CharacterQueries]"]
        domain["Domain model<br/>[domain.Character + rules.go]"]
        jsonrepo["Character repository<br/>[infrastructure.JSONCharacterRepository]"]
        csvrepos["Reference data repositories<br/>[CSV*Repository]"]
        client["SRD API client<br/>[infrastructure.SRDAPIClient]"]
    end
    router --> pages & api & health
    pages --> queries & commands
    api --> queries & commands
    commands --> domain
    commands --> jsonrepo & csvrepos
    queries --> jsonrepo & client
```

## 5. Domain model

```mermaid
classDiagram
    class Character {
        <<aggregate root, entity>>
        -name string
        -race string
        -class string
        -level int
        -background string
        -scores AbilityScores
        -skillProficiencies []string
        -mainHand, offHand, armor, shield string
        -knownSpells, preparedSpells []string
        +NewCharacter(...) *Character
        +ChangeLevel(level) error
        +EquipWeapon(*Equipment, slot) error
        +EquipArmor(*Equipment) error
        +EquipShield(*Equipment) error
        +LearnSpell(*Spell) error
        +PrepareSpell(*Spell) error
        +ArmorClass() int
        +SpellSlots() map
        +PassivePerception() int
    }
    class AbilityScores {
        <<value object>>
        Str Dex Con Int Wis Cha int
        +HasStandardArray() bool
    }
    class Equipment {
        <<reference data>>
        -name, equipmentType
        +IsWeapon() IsArmor() IsShield()
    }
    class Spell {
        <<reference data>>
        -name, level, classes
        +AvailableTo(class) bool
    }
    class Class {
        <<reference data>>
        -name, skillOptions, skillChoiceCount
    }
    class RaceAbilityBonus {
        <<value object>>
        Ability string
        Points int
    }
    Character *-- "1" AbilityScores : owns
    Character ..> Equipment : refers to by name
    Character ..> Spell : refers to by name
    Character ..> Class : class name
    Character ..> RaceAbilityBonus : applied once at creation
```

## 6. Context map

```mermaid
flowchart LR
    core["<b>Character Sheet</b><br/>(core domain)<br/>Character aggregate, sheet rules"]
    catalogue["<b>SRD Reference Catalogue</b><br/>(supporting)<br/>data/*.csv: classes, race bonuses,<br/>equipment, spells"]
    external["<b>D&D 5e SRD API</b><br/>(generic, external)<br/>dnd5eapi.co"]

    catalogue -- "Conformist: Character uses the<br/>catalogue's names as they are" --> core
    external -- "Open host service (public JSON);<br/>passed through, no translation" --> core
```

## 7. Entity-relationship diagram (persistence)

The project stores characters in one JSON document, `characters.json`: an
array of character objects. There are no foreign keys; references to the
reference data are **names** that are checked when a command runs. The CSV
files are read-only tables.

```mermaid
erDiagram
    CHARACTER {
        string Name PK "unique (enforced by CharacterService, RejectDuplicate)"
        string Race "refers to RACE_ABILITY_BONUS.race (by name)"
        string Class "refers to CLASS.class (by name)"
        int Level "1-20"
        string Background
        string_array SkillProficiencies "embedded list"
        string MainHand "refers to EQUIPMENT.name"
        string OffHand "refers to EQUIPMENT.name"
        string Armor "refers to EQUIPMENT.name"
        string Shield "refers to EQUIPMENT.name"
        string_array KnownSpells "embedded, refers to SPELL.name"
        string_array PreparedSpells "embedded, refers to SPELL.name"
        int Str "and Dex Con Int Wis Cha"
    }
    CLASS {
        string class PK
        string skill_options "semicolon list"
        int skill_choice_count
    }
    RACE_ABILITY_BONUS {
        string race "part of key"
        string ability "part of key"
        int points
    }
    EQUIPMENT {
        string name PK
        string type "Weapon, Armor, ..."
    }
    SPELL {
        string name PK
        int level "0 = cantrip"
        string class "comma list of classes"
    }
    CHARACTER }o--|| CLASS : "plays"
    CHARACTER }o--|{ RACE_ABILITY_BONUS : "race grants"
    CHARACTER }o--o{ EQUIPMENT : "equips (0-4 slots)"
    CHARACTER }o--o{ SPELL : "knows / prepares"
```

**Persistence model vs. domain model**: one `CHARACTER` object is exactly one
`Character` aggregate, including its owned lists (skills, spells) embedded in
the same object, so the aggregate is always loaded and saved as a whole
(`characterRecord` in `json_character_repository.go`). The reference tables
are *not* part of the aggregate; the character only stores their names.
**Indexes**: none. Lookups scan the array; with tens of characters this is
cheaper than maintaining an index (a database would add a unique index on
`Name`).
