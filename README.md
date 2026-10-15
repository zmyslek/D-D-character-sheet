# HZ HBO-ICT Dungeons & Dragons character sheet generator
If you like D&D, yay! If you dislike it, apologies! As you might have guessed from the title, you are going to build a character sheet generator for Dungeons and Dragons.

## Running the application

Requirements: Go 1.23 or newer (standard library only). Run commands from this folder.

```sh
go run . serve                  # web UI on http://localhost:8080
go run .                        # list the CLI commands
go run . create -name Mira -race elf -class wizard -str 8 -dex 14 -con 13 -int 15 -wis 12 -cha 10
go run . view -name Mira
go run . level -name Mira -level 5
go test ./tests/                # automated tests (all tests live in tests/)
```

Configuration (optional environment variables): `MPP_CHARACTERS_FILE`
(default `characters.json`), `MPP_DATA_DIR` (default `data`), `MPP_ADDR`
(default `:8080`), `MPP_SRD_API_URL` (default `https://www.dnd5eapi.co/api/2014`).

Load-balanced demo with Docker (nginx + one writer + two readers):

```sh
docker compose up -d --build    # http://localhost:8080
docker compose down -v          # stop and delete the data volume
```

JSON API: `GET/POST /api/characters`, `GET/PUT/DELETE /api/characters/{name}`,
`GET /api/characters/{name}/enrichment`, `GET /api/spells/{name}`,
`GET /api/equipment/{name}`, `GET /healthz`. `POST /api/characters` accepts an
`Idempotency-Key` header so retries never create a character twice.

## Documentation

| Document | Contents |
|---|---|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | layers, request flow, concurrency, async work, Docker/DNS/load balancing, statelessness |
| [docs/DDD.md](docs/DDD.md) | subdomains, bounded contexts, the Character aggregate and its invariants |
| [docs/DOMAIN_GLOSSARY.md](docs/DOMAIN_GLOSSARY.md) | ubiquitous language |
| [docs/REQUIREMENTS.md](docs/REQUIREMENTS.md) | functional and non-functional requirements |
| [docs/DESIGN_PATTERNS.md](docs/DESIGN_PATTERNS.md) | patterns used, leaky abstractions review |
| [docs/GO_CONCEPTS.md](docs/GO_CONCEPTS.md) | encapsulation, pointers, stack/heap, paradigms, determinism |
| [docs/QUALITY_AND_TESTING.md](docs/QUALITY_AND_TESTING.md) | ISO/IEC 25010 assessment, test strategy, commands, SonarQube, results |
| [docs/MANUAL_TEST_PLAN.md](docs/MANUAL_TEST_PLAN.md) | manual tests with observed results |
| [docs/STANDARDS.md](docs/STANDARDS.md) | ISO/IEC/IEEE standards and how they were used |
| [docs/DIAGRAMS.md](docs/DIAGRAMS.md) | architecture, C4, domain, context map, ER diagrams |
| [docs/ASSESSMENT_TRACEABILITY.md](docs/ASSESSMENT_TRACEABILITY.md) | requirement → code → test map and handover report |

## Dungeons and dragons?
(Feel free to skip this part if you've ever played D&D, Pathfinder, Baldur's Gate, Neverwinter Nights, Star Wars: Knights of the Old Republic, or any other D&D inspired (video) game).

Dungeons & Dragons (D&D from now on), is a tabletop party roleplaying game. That means it's a game you play with at least three people, on a literal table top. There are modern video games and online solutions for playing with friends far away. The game consists of a Dungeon Master (DM) and a party of Player Characters (PC). The DM prepares the story, the dungeoning of dragons, the quests, the loot, the world, the people that live in it and everything more you would expect from an epic fantasy roleplaying game. The PCs are players like you or me. At the start of a campaign (a game of D&D can run for years), the players create characters that they will roleplay. This is where the character sheet comes into play.

See, the DM has a hard enough job already. If you as a player would generate a Goku-like character that would just melt every single enemy's brain by just looking at them, the game would be incredibly unbalanced and, frankly, boring. Combat is about 50% of the D&D experience, and adventuring with your friends is the other 50%. It would be nice to have a character that is interesting to combat and adventure with then! That means they must be good at some stuff, but also bad at other things. They might have a very complicated relationship with their family, or they might be on the verge of a new business adventure. They might go the the gym 9 days a week, or they might be the most comedic friend you'll ever have. To make sure that every player can bring something unique to the table without completely upsetting the game balance, there are some rules to adhere to when creating characters. This is what we call the character sheet, and it is what you will create an application for.

## The Systems Reference Document
During this course you'll implement most of the Systems Reference Document 5.1 (SRD). This is a 403-page (yes, you read that right) document detailing all the open-source rules of character sheet generation in D&D. If you've every played D&D, you might notice that some stuff from the Player Handbook is missing. That's correct since that material is under license by Wizards of the Coast. But some stuff in the Player Handbook is so obvious, or generic to the gaming world, that they've tried to open source as much as possible. This results in the SRD. As example: a Wizard is a class (duh, every fantasy game has a wizard) and a Wizard can cast Fireball (duh, every fantasy game has fire spells).

Now why did I pick D&D and the SRD for you to implement? Because the SRD is probably the most detailed and complete requirements document you'll ever read. No "the client doesn't know what he wants" in my class! It also helps that, because it is so well-documented, you can easily search for specific things in search engines or LLMs. Just google "formula proficiency bonus dnd" for example.

## Requirements
Now for the actual requirements that you'll implement. But first, take notice of four preconditions.

### The preconditions
1. We are implementing the SRD 5.1 and not the SRD 5.2. In other words: we are implementing D&D 5e 2014 rules. There is a new ruleset since 2024. We will completely ignore this 2024 ruleset. (Reasoning is that later on, you need to use some external tools. Not all of those tools have been updated to 2024 rules already. Also Baldur's Gate 3 uses 2014 rules so they are what most of you probably recognize best.)
2. Every time the requirements state "we'll ignore xxx", interpret this as "we'll ignore xxx, but it might be the final assignment".
3. When the SRD states you can choose 2 or 3 from a collection, pick the first 2 or 3 from that collection, in alphabetical order. We'll ignore the problem of duplicate proficiencies in race+class combination.
4. CodeGrade uses 1 singlethreaded CPU core to compile your program. CodeGrade tests time out after 2 minutes. That means you code should compile within 2 minutes on 1 singlethreaded CPU core. Practically: you can pull in 1, maaaaaaaybe 2 external libraries before running over 2 minute compilation times. This limitation is super arbitrary and super fun for me :-)

### The actual requirements
My list of requirements might be somewhat vague. That's deliberate. All you need to know is in the SRD. Also, if you pass the tests connected to the requirement, you've probably implemented it correctly no? Long live TDD. (Note, I've added checkboxes so you can easily copy-paste the list as markdown and check off the items)

1. Character CRU
- [x] Create new characters
- [x] Read their character sheet
- [x] Update relevant bits: level and proficiency bonus
- [x] Characters have the following attributes: name, race, class, level, ability scores, background and proficiency bonus
- [x] Ability scores are determined through Standard Array (15/14/13/12/10/8) and Race
- [x] Classes only deal with main classes, we'll ignore subclasses
- [x] Add skill proficiencies on character creation. We'll ignore racial skill proficiencies.
- [x] Determine skill modifiers based on ability scores and proficiency bonus
- [x] We'll ignore level-up bonuses and unlocks other than proficiency bonus and spell slots.
- [x] Anything else listed in the SRD we'll ignore

2. Equipment management
- [x] Add equipment to/from a character: weapons, armor and shield
- [x] We'll ignore the concept of "inventory": unequipped items such as a backup dagger, rope, torches, etc.
- [x] Any other gear listed in the SRD we'll ignore

3. Spellcasting (When applicable :))
- [x] Add known/prepared spells from spell list for your class. It's okay to pick random spells as long as they fit the slot level
- [x] Add max spell slots per level
- [x] Anything else related to spells listed in the SRD we'll ignore
      
4. Integrate external information
- [x] Enrich spells and items from external API: https://www.dnd5eapi.co/
PLEASE READ CAREFULLY: this is an API built and maintained by volunteers. They've rate-limited the API to 50 requests per second. Theoretically, you could gather all info on spells and equipment in about 7.5 seconds. But be nice to these people and don't run up their server costs. Send 5-10 requests per second at most, and only test with small batches (8 or 10 spells) instead of your whole database at a time. If it works for 10 spells, it will work for all 319. You can also download their Docker image to run the API locally: https://github.com/5e-bits/5e-srd-api?tab=readme-ov-file#how-to-run
- [x] Spells: school, range
- [x] Weapons: category, range (normal range, ignore long range), two-handed
- [x] Armor: armor class, dexterity bonus
- [] If you want to do more and build a fully fledged character sheet generator, feel free! We will only look at these properties for this course and exam.

5. Combat stat calculation
- [x] Armor class, initiative, passive perception (character sheet calls it "passive wisdom")
- [x] Spell casting: spellcasting ability, spell save DC, spell attack bonus

6. User interfaces
- [x] CLI: create a character, view it and list all characters
- [x] HTML frontend: a list of all characters that links to their character sheets (HTML template provided on Learn)

## External help for creating character sheets
D&D can be quite overwhelming, especially if you've never played it before. Below is a list of sources I deem good, that will hopefully speedrun you through the SRD, so you more-or-less know how it works.

* JoCat's Crap Guide to D&D: Character Sheet. Note that this video contains strong language. It is, however, the best explanation of character sheet creation that I know of. If you can deal with the language, this is probably the single best source you'll find.
* Ginny Di's intro to D&D. Great in explaining the broad strokes, so you know where/how to find the details.
* D&D Beyond's Character Creator. The creators of D&D have their own Character Sheet builder tool. Might as well learn from the best right? (I'm fairly sure this builder uses 2024 rules but it still serves as a great example. The rule changes are mostly details in formulas, not entire systems that have been changed)
* D&D Beyond's Guide on creating characters. Text-based, if you prefer that. There is a more visually pleasing 2024 guide, but I think the 2014 explains it better. Still, here is the 2024 guide: https://www.dndbeyond.com/posts/1059-how-to-create-your-first-dungeons-dragons

## On min-maxing
I like D&D and it's okay if you don't. I don't mind if you create nonsense characters. For example, a Fighter with 16 Charisma and 8 Strength is absolutely bad to play with, but no one is stopping you to do it. Throughout this course, there is no need to create "optimized" characters. You don't need to minmax them. Just fill in some numbers. It's fine.

## The end
GLHF!
