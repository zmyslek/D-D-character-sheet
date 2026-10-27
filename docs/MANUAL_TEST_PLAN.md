# Manual test plan

Executed tests show the output that was actually observed on 2026-10-15
(Windows 11 + WSL2, Docker Desktop 4.95, Go 1.27.1). Tests that were not run
say **Not executed**. Container hostnames are random IDs and differ on every
run.

## Setup

```sh
# Docker tests (from mpp-zmyslek/)
docker compose up -d --build
docker compose ps                      # all services "healthy"
# CLI tests: an empty working directory with a link/copy of data/
go build -o mpp .
```

Reset: `docker compose down -v` (removes the containers **and** the data
volume). For the CLI, delete `data/characters.json`, or restore the committed
sample file with `git checkout data/characters.json`.

## A. Deployment (Docker, load balancer, DNS)

| ID | Objective | Preconditions | Steps | Expected result | Actual result | Status | Evidence |
|---|---|---|---|---|---|---|---|
| M-01 | Stack starts healthy | Docker running | `docker compose up -d --build`; `docker compose ps`; `curl localhost:8080/lb-health` | writer and both readers "healthy"; lb answers `ok` | all three app containers `(healthy)`; lb `ok` | Pass | `docker compose ps` output |
| M-02 | Docker DNS returns every replica | M-01 | `docker compose exec lb nslookup reader` and `... writer` | two addresses for `reader`, one for `writer` | reader → `172.18.0.4`, `172.18.0.3`; writer → `172.18.0.2` | Pass | nslookup output |
| M-03 | Create through the load balancer is idempotent | M-01 | `POST /api/characters` with header `Idempotency-Key: demo-1` and a Standard Array body; repeat the same request; then repeat without the key | 201 from the writer; retry 201 with `Idempotent-Replayed: true`; without key 409 | 1st: `201`, `X-Upstream: 172.18.0.2:8080` (writer); 2nd: `201`, `Idempotent-Replayed: true`; 3rd: `409 {"error":"a character with this name already exists"}` | Pass | response headers |
| M-04 | Reads are balanced over the readers | M-03 | 6 × `curl -sD- localhost:8080/api/characters`, compare `X-Instance` with `docker exec <c> hostname` | answers alternate between the two reader hostnames | alternated `0b077bdf5229` / `34a34db0a898` (= reader-2 / reader-1) | Pass (after adding `zone` to nginx, see note) | headers |
| M-05 | Reads survive a stopped reader | M-04 | `docker stop mpp-zmyslek-reader-1`; 6 GET requests | all 200, served by reader-2 | 6 × `200`, all `X-Instance: 0b077bdf5229` | Pass | headers |
| M-06 | Writer is the backup for reads | M-05 | also stop reader-2; GET | 200 from the writer | `200`, `X-Instance: 37f1c9b931fb` (writer) | Pass | headers |
| M-07 | Readers cannot write the data | M-01 | `docker exec mpp-zmyslek-reader-1 touch /data/x` | refused | `touch: /data/x: Read-only file system` | Pass | console |
| M-08 | Graceful shutdown | M-01 | `docker stop -t 15 mpp-zmyslek-writer-1`; inspect exit code | exit code 0 (server stopped by itself, not killed) | `exit=0` | Pass | `docker inspect` |
| M-09 | Containers run as non-root | M-01 | `docker exec mpp-zmyslek-writer-1 id` | not uid 0 | `uid=100(app) gid=101(app)` | Pass | console |
| M-10 | Writes go to the writer | M-01 | `PUT /api/characters/Docker%20Demo` (level 2) | 200 with the writer's hostname | `200`, `X-Instance: 37f1c9b931fb` (writer) | Pass | headers |
| M-11 | Data survives a rebuild | M-03 | `docker compose up -d --build`; `GET /api/characters/Docker%20Demo` | 200, character still there | 200 after the rebuild | Pass | headers |

Note for M-04: the first run sent all reads to one reader. Cause: each nginx
worker process kept its own round-robin position. Adding a shared `zone` to
the upstream blocks fixed it; the result above is from the run after the fix.

## B. CLI

Run in an empty directory containing `data/`.

| ID | Objective | Steps | Expected result | Actual result | Status |
|---|---|---|---|---|---|
| M-12 | Create and view | `mpp create -name Mira -race elf -class wizard -str 8 -dex 14 -con 13 -int 15 -wis 12 -cha 10`; `mpp view -name Mira` | `saved character Mira`; DEX 16 (+3) (elf +2); skills `arcana, history, insight, religion`; slots Level 0: 3, Level 1: 2; spell save DC 12 | as expected | Pass |
| M-13 | Occupied hand is rejected | `mpp equip -name Mira -weapon dagger -slot "main hand"`; then the same with `club` | `Equipped weapon dagger to main hand`; then `main hand already occupied` | as expected | Pass |
| M-14 | Spell level must fit the slots; level up unlocks it | `mpp prepare-spell -name Mira -spell fireball`; `mpp level -name Mira -level 5`; prepare again | `the spell has higher level than the available spell slots`; `Mira is now level 5 (proficiency bonus +3)`; `Prepared spell fireball` | as expected | Pass |
| M-15 | Level cannot decrease | `mpp level -name Mira -level 3` | `level cannot decrease` | as expected | Pass |
| M-16 | Missing name is a usage error | `mpp create -race elf -class wizard`; check exit code | `name is required`, exit code 2 | `name is required`, `[exit 2]` | Pass |
| M-17 | Level range | `mpp create -name X -race elf -class wizard -level 21` | `level must be between 1 and 20` | as expected | Pass |

## C. Browser (web UI)

| ID | Objective | Steps | Expected result | Actual result | Status |
|---|---|---|---|---|---|
| B-01 | List page | `go run . serve`, open `http://localhost:8080` | list of saved characters with links | – | Not executed |
| B-02 | Create via the sheet | "New character", fill name, race, `Wizard 1`, Standard Array, Save | redirected to the new sheet | – | Not executed |
| B-03 | Non-Standard-Array scores rejected | as B-02 with scores 18, 14, 13, 12, 10, 8 | error message, nothing saved | – | Not executed |
| B-04 | Edit equipment and spells | open a sheet → Edit → add `Weapon: Dagger` and `Prepared spells: fire bolt` → Save | sheet shows the changes | – | Not executed |
| B-05 | Invalid edit | Edit → `Weapon: Banana` → Save | error shown, sheet unchanged | – | Not executed |
| B-06 | Enrichment | open a sheet with spells (needs internet) | spell school and range shown below the spell list | – | Not executed |
| B-07 | Delete | Delete on the list page, confirm | character removed from the list | – | Not executed |
| B-08 | Concurrent edits | edit the same character in two tabs, save both | both saves succeed; the second overwrites the first (no optimistic locking, see limitations) | – | Not executed |

The behaviour behind B-01 – B-07 is covered by the automated HTTP tests in
`tests/web_test.go`; the browser tests check the page script and layout.

## D. Exam feature 1 – encumbrance

The exam's examples: a level 1 halfling rogue with Str 8 (capacity
20 + 2 × (−1) = 18 lb) and Dex 15. SRD weights: shield 6 lb, studded leather
armor 13 lb, heavy crossbow 18 lb. Executed on 2026-10-27 in an empty
directory with a copy of `data/*.csv`, `MPP_SRD_API_URL=http://127.0.0.1:1`
(offline).

| ID | Objective | Steps | Expected result | Actual result | Status |
|---|---|---|---|---|---|
| E-01 | No items, then a shield | `mpp create -name Pip -race halfling -class rogue -str 8 -dex 15`; `mpp view -name Pip -full`; `mpp equip -name Pip -shield shield`; view again | `STR: 8 (-1)`; `Condition: none (0 lb carried, capacity 18 lb)`; then `Condition: none (6 lb carried, capacity 18 lb)` | as expected | Pass |
| E-02 | Shield and studded leather armor | `mpp equip -name Pip -armor "studded leather armor"`; view | `Condition: encumbered (19 lb carried, capacity 18 lb)` | as expected | Pass |
| E-03 | Shield, armor and a heavy crossbow | `mpp equip -name Pip -weapon "crossbow, heavy" -slot "main hand"`; view | `Condition: heavily encumbered (37 lb carried, capacity 18 lb)` | as expected | Pass |
| E-04 | Plain `view` is unchanged | `mpp view -name Pip` | no `Condition` line (the CodeGrade output stays identical) | as expected; also `TestCLIViewExactOutput` | Pass |
| E-05 | Shown on the sheet | `mpp serve`, open `/characters/Pip` | "Features & Traits" starts with `Condition: heavily encumbered (37 lb carried, capacity 18 lb)` | – | Not executed (the page data is checked by `TestSheetShowsEncumbrance`) |

## F. Exam feature 2 – CSV storage

Executed on 2026-10-27 in an empty directory with a copy of `data/*.csv`.

| ID | Objective | Steps | Expected result | Actual result | Status |
|---|---|---|---|---|---|
| S-01 | CLI on CSV storage | `export MPP_STORAGE=csv`; `mpp create -name Mira -race elf -class wizard -int 15`; `mpp equip -name Mira -weapon dagger -slot "main hand"`; `mpp prepare-spell -name Mira -spell "fire bolt"`; `mpp list`; `cat data/characters.csv` | every command succeeds; `data/characters.csv` has a header and one readable row; no `characters.json` | see result below the table | Pass |
| S-02 | Open in a spreadsheet | open `data/characters.csv` in Excel / LibreOffice Calc | one column per field, lists readable | – | Not executed |
| S-03 | Edit in a spreadsheet and read back | change the level cell to 3, save as CSV, `mpp view -name Mira` | `Level: 3` | – | Not executed (the same edits are automated in `TestCSVFileEditedInASpreadsheet`) |
| S-04 | Export the JSON storage | `unset MPP_STORAGE`; create a character; `mpp export -file party.csv`; again | `exported 1 character to party.csv`; then `party.csv already exists; choose a new file` (exit 1) | as expected | Pass |

S-01 output (`saved character Mira`, `Equipped weapon dagger to main hand`,
`Prepared spell fire bolt`, `Mira - elf wizard (level 1)`); `data/` contains
only `characters.csv`, no `characters.json`:

```
Name,Race,Class,Level,Background,STR,DEX,CON,INT,WIS,CHA,Skill Proficiencies,Main Hand,Off Hand,Armor,Shield,Known Spells,Prepared Spells
Mira,elf,wizard,1,acolyte,10,2,0,15,0,0,arcana; history; insight; religion,Dagger,,,,,fire bolt
```
