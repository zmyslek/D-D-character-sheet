# Requirements

Source: the assignment README (SRD 5.1 character sheet generator) plus the
quality requirements of the course. Structure follows the requirement
attributes recommended by ISO/IEC/IEEE 29148 (identifier, statement, rationale
or source, acceptance criteria, verification). Test files are in `tests/`.

## Functional requirements

| ID | Requirement | Actor / trigger | Preconditions | Expected behaviour and acceptance criteria | Interface | Verification |
|---|---|---|---|---|---|---|
| FR-01 | Create a character with name, race, class, background, level and ability scores | Player | race and class exist in the catalogue | Character stored; racial bonuses applied once; first N class skills and background skills added. Missing name → "name is required" (exit 2); level outside 1–20 or score outside 0–30 rejected | CLI `create`; `POST /api/characters`; form `POST /characters/new` | `TestServiceCreate`, `TestCLICreateViewListDelete`, `TestCLIValidation`, `TestCharactersAPI`, `TestPages` |
| FR-02 | Web/API creation uses the Standard Array (15, 14, 13, 12, 10, 8) | Player (web) | – | Other scores → 400 with the Standard Array message | `POST /api/characters` | `TestHasStandardArray`, `TestServiceCreate`, `TestCharactersAPI` |
| FR-03 | Read a character sheet with derived values | Player | character exists | Shows modifiers, proficiency bonus, skills, AC, initiative, passive perception, spell slots, spellcasting ability, save DC and attack bonus; unknown name → `character "X" not found` / 404 | CLI `view`; `GET /characters/{name}`; `GET /api/characters/{name}` | `TestCLICreateViewListDelete`, `TestCombatStats`, `TestCharacterAPIPutAndDelete` |
| FR-04 | List all characters | Player | – | One line per character: `Name - race class (level N)`; JSON array in the API | CLI `list`; `GET /`; `GET /api/characters` | `TestCLICreateViewListDelete`, `TestCharactersAPI`, `TestPages` |
| FR-05 | Update the level; proficiency bonus follows | Player | character exists | Level 1–20, never lower than before | CLI `level`; `PUT /api/characters/{name}` | `TestChangeLevel`, `TestServiceSpellsLevelAndDelete`, `TestCharacterAPIPutAndDelete` |
| FR-06 | Equip a weapon in the main or off hand | Player | item is a weapon; slot free | Occupied slot / non-weapon / bad slot rejected with the original messages | CLI `equip -weapon -slot`; `PUT /api/characters/{name}` | `TestEquipment`, `TestServiceEquipment`, `TestCLIEquip` |
| FR-07 | Equip armor and a shield | Player | item is armor / a shield | AC uses SRD armor formulas, +2 for a shield | CLI `equip -armor`, `-shield`; PUT | `TestEquipment`, `TestArmorClass`, `TestCLIEquip` |
| FR-08 | Learn (bard, ranger, sorcerer, warlock) or prepare (cleric, druid, paladin, wizard) spells from the class list that fit the available slot level | Player | class can cast at this level | Wrong mode, wrong class list, too high level rejected; adding twice stores once | CLI `learn-spell`, `prepare-spell`; PUT | `TestSpells`, `TestCLISpells`, `TestConcurrentChangesAreNotLost` |
| FR-09 | Show maximum spell slots per spell level | Player | caster | SRD 5.1 tables, including half casters and warlock Pact Magic | CLI `view`; web sheet | `TestSpellSlotsMatchTheSRD` |
| FR-10 | Delete a character | Player | character exists | Removed; second delete → "character not found" / 404 | CLI `delete`; `DELETE /api/characters/{name}`; form | `TestServiceSpellsLevelAndDelete`, `TestCharacterAPIPutAndDelete`, `TestPages` |
| FR-11 | Enrich spells (school, range) and items (category, range, two-handed, AC, Dex bonus) from dnd5eapi.co, max 5 requests/s | Web page | network access | Details returned as JSON; unknown name → 404; upstream failure → 502 | CLI `view` (sections "Spells:" and "Equipment details:"), CLI `enrich`; `GET /api/spells/{name}`, `GET /api/equipment/{name}`; character sheet page | `TestSRDAPIClient`, `TestSRDAPIClientRateLimit`, `TestEnrichmentEndpoints`, `TestCLIEquip`, `TestCLISpells`, `TestCLIViewWithoutExternalAPI` |
| FR-12 | Enrich all spells and items of one character in one request | Web client | character exists | All lookups run concurrently; each result has data or an error; cancelled after 30 s or client disconnect | `GET /api/characters/{name}/enrichment` | `TestEnrichCharacterRunsLookupsConcurrently`, `TestEnrichCharacterStopsWhenCancelled`, `TestEnrichmentEndpoints` |
| FR-13 | A create request may be retried safely with an `Idempotency-Key` | API client | – | Same key + same body → same character, `201` + `Idempotent-Replayed: true`, nothing created twice; same key + different body → 409 | `POST /api/characters` | `TestServiceIdempotencyKey`, `TestConcurrentRetriesCreateOnce`, `TestCreateWithIdempotencyKey` |
| FR-14 | Health endpoint for the load balancer and Docker | Docker, nginx | – | `GET /healthz` → 200 `{"status":"ok","instance":...}` | `/healthz` | `TestHealthAndInstanceHeader`, manual M-01 |

## Non-functional requirements

Numeric targets marked *(proposed)* are the team's own targets, not
stakeholder-agreed figures.

| ID | Quality (ISO/IEC 25010:2023) | Requirement | Target | Verification | Evidence |
|---|---|---|---|---|---|
| NFR-01 | Maintainability – modularity | Domain does not depend on application, infrastructure, web, `os`, `net/http` or JSON; application does not depend on infrastructure or web | 0 violations | `TestDependencyDirection` | passes |
| NFR-02 | Maintainability – analysability | No function above cyclomatic complexity 25 or 60 lines | 0% of unit code *(proposed)* | `go run ./tools/metrics` | 0% / 0% (baseline 16.4% / 26.8%) |
| NFR-03 | Maintainability – modifiability | Duplicated code | < 5% *(proposed)* | `tools/metrics`, SonarQube | 2.5% (tools), 0.0% (SonarQube) |
| NFR-04 | Maintainability – testability | Statement coverage of `internal/` and `web/` | ≥ 80% *(proposed)* | `go test -coverpkg` | 87.4% |
| NFR-05 | Reliability – faultlessness | All automated tests pass; no data races | 100% pass, 0 races | `go test ./...`, `go test -race ./...` (Docker) | see QUALITY_AND_TESTING.md |
| NFR-06 | Reliability / concurrency | Concurrent changes in one process never lose an update; concurrent retries never create duplicates | 0 lost updates in tests | `TestConcurrentChangesAreNotLost`, `TestConcurrentRetriesCreateOnce` | passes, also under `-race` |
| NFR-07 | Reliability – recoverability | A crash or concurrent read never sees a half-written `characters.json` | atomic replace | write-to-temp + rename (`writeAll`); `TestJSONRepositoryFiles` checks no temp file is left | passes |
| NFR-08 | Reliability – availability | Reads keep working when one reader instance stops | no failed reads | manual M-05, M-06 | observed |
| NFR-09 | Performance efficiency | Enriching N items takes about one lookup time, not N × lookup time (excluding the upstream rate limit) | < 200 ms for 5 × 50 ms lookups *(proposed)* | `TestEnrichCharacterRunsLookupsConcurrently` | passes |
| NFR-10 | Compatibility / co-existence | At most 5 requests per second to the volunteer-run SRD API per instance | ≥ 200 ms between requests | `TestSRDAPIClientRateLimit` | passes |
| NFR-11 | Security | No secrets in the repository; containers run as non-root; readers cannot write data | – | review, `id` in container, read-only volume (M-07) | `uid=100(app)`; write fails with "Read-only file system" |
| NFR-12 | Security | SonarQube reports no vulnerabilities or security hotspots | 0 | SonarQube scan | 0 / 0 |
| NFR-13 | Flexibility – scalability | Read capacity grows by adding reader instances; writes stay on one instance | – | `docker compose up --scale reader=N` + restart lb | demonstrated with 2 readers; **limitation**: writes do not scale |
| NFR-14 | Compatibility | CLI output and exit codes stay compatible with the original program, except the documented SRD corrections | identical transcript apart from listed fixes | golden CLI transcript comparison (see QUALITY_AND_TESTING.md) | identical apart from SRD fixes and new range checks |
| NFR-15 | Portability / build | Standard library only; builds within the grader's limit (1 core, 2 min) | 0 external modules | `go.mod` has no `require` | holds |
