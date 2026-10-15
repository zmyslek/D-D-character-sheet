# Architecture

This document describes the architecture as implemented. Diagrams are in
[DIAGRAMS.md](DIAGRAMS.md); the domain design is in [DDD.md](DDD.md).

## 1. What the system does

A D&D 5e (SRD 5.1) character sheet manager. A player creates characters
(name, race, class, background, level, ability scores), equips weapons, armor
and a shield, learns or prepares spells, and reads a calculated sheet (ability
modifiers, proficiency bonus, armor class, initiative, passive perception,
spell slots, spell save DC and attack bonus). Two user interfaces exist:

* a **CLI** (`create`, `view`, `list`, `delete`, `equip`, `learn-spell`,
  `prepare-spell`, `level`, `serve`), also used by the automated grader;
* a **web UI and JSON API** started with `serve`.

## 2. Layers and dependency rule

The structure follows the course example `lesson3/library`: a flat
`domain` / `application` / `infrastructure` split, wired in `main.go`.

```
main.go (CLI + composition root)      web/ (HTTP adapter)
            \                          /
             v                        v
          internal/application  (use cases: commands + queries)
                     |
                     v
          internal/domain       (Character aggregate, SRD rules,
                     ^           repository interfaces)
                     |
          internal/infrastructure (JSON + CSV repositories, SRD API client)
```

| Package | Responsibility | May import |
|---|---|---|
| `internal/domain` | Business rules: `Character` aggregate, value objects, SRD formulas (`rules.go`), repository interfaces | standard library only (no `os`, `net/http`, `encoding/json`) |
| `internal/application` | Use cases. `CharacterService` = commands, `CharacterQueries` = queries. Mutex, idempotency keys, concurrent enrichment | `domain` |
| `internal/infrastructure` | Adapters for files and the external API: `JSONCharacterRepository`, four CSV repositories, `SRDAPIClient` | `domain` |
| `web` | HTTP routing, templates, JSON encoding, status codes | `application`, `domain` |
| `main.go` | Reads configuration, wires concrete adapters into the services, runs the CLI and the server | everything |

Dependencies point **inward**: infrastructure implements interfaces declared
in `domain` (`CharacterRepository`, `ClassRepository`, ...) and in
`application` (`CharacterReader`, `ResourceLookup`), so neither the domain nor
the application layer imports infrastructure. The rule is enforced by
`tests/architecture_test.go` (`TestDependencyDirection`).

## 3. Request flow

**Command** (example: `PUT /api/characters/Mira`):

1. `web.Server.characterAPI` parses the URL and JSON body (`updateCharacter`).
2. `application.CharacterService.Update` takes the service mutex, loads the
   character through `domain.CharacterRepository`, and calls domain methods
   (`ChangeLevel`, `EquipWeapon`, `PrepareSpell`, ...). Any rule violation
   returns an error and nothing is saved.
3. `infrastructure.JSONCharacterRepository.Update` takes the repository mutex,
   rewrites the file into a temporary file and renames it over
   `characters.json` (atomic replacement).
4. `web.statusFor` maps errors to status codes (404 / 409 / 400).

**Query** (example: `GET /api/characters/Mira`): `characterAPI` →
`CharacterQueries.Get` → `CharacterReader.GetByName` → JSON. Queries never
call a write method; `CharacterReader` does not even contain one.

## 4. Configuration

| Variable | Default | Used for |
|---|---|---|
| `MPP_CHARACTERS_FILE` | `characters.json` | character storage |
| `MPP_DATA_DIR` | `data` | SRD CSV files |
| `MPP_ADDR` | `:8080` | `serve` listen address (`-addr` overrides) |
| `MPP_SRD_API_URL` | `https://www.dnd5eapi.co/api/2014` | external SRD API |

The defaults reproduce the original behaviour, so the CLI contract used by the
grader is unchanged. No secrets exist in this project.

## 5. Concurrency, transactions and consistency

| Shared state | Protection | Where |
|---|---|---|
| A character during load → change → save | `CharacterService.mu` (one command at a time per process) | `internal/application/character_service.go` |
| The characters file | `JSONCharacterRepository.mu` + write-to-temp-and-rename | `internal/infrastructure/json_character_repository.go` |
| Idempotency keys (`created` map) | `CharacterService.mu` | `character_service.go` |
| SRD API rate limit (`lastRequest`) | `SRDAPIClient.mu` | `internal/infrastructure/srd_api_client.go` |
| Enrichment results | each goroutine owns one slice element; `WaitGroup` | `CharacterQueries.EnrichCharacter` |

**Transaction boundary**: one command changes one character (one aggregate).
The whole load-change-save cycle runs under the service mutex, which is a
*pessimistic* lock: there are no concurrent writers inside a process, so
lost updates are impossible (`TestConcurrentChangesAreNotLost`). The file is
replaced atomically, so a reader or a crash sees either the old or the new
file, never a half-written one.

**Limitation (documented, not hidden)**: the mutexes only work inside one
process. Two *separate* processes writing at the same moment (for example two
CLI commands, or two server instances) could still lose one update. The
Docker setup therefore uses a **single writer** (section 7). A real database
with transactions would remove this limit; the project deliberately stays on
the JSON file (see [DDD.md §6](DDD.md#6-persistence-decision)).

## 6. Asynchronous processing

| Operation | Why asynchronous | Lifecycle and failure handling |
|---|---|---|
| `GET /api/characters/{name}/enrichment` → `CharacterQueries.EnrichCharacter` | A character has several spells and items; looking them up one after another would add the waits together | One goroutine per lookup, **awaited** with a `WaitGroup` (not fire-and-forget). The request context plus a 30 s timeout cancel waiting lookups. A failed lookup is reported in that entry's `error` field, never dropped. Upstream load is bounded by the client's rate limiter (5 requests/s) |
| HTTP server in `serve` | The server must run while `main` waits for SIGINT/SIGTERM | `serve` starts `Serve` in a goroutine; on a signal it calls `Shutdown` with a 10 s limit so in-flight requests (and their file writes) finish. Verified: `docker stop` exits with code 0 |

Nothing is persisted for later execution or retried automatically: lookups are
read-only and cheap to repeat, so the client simply asks again.

## 7. Deployment: DNS and load balancing (local demonstration)

`docker-compose.yml` starts:

| Service | Instances | Role |
|---|---|---|
| `lb` (nginx 1.27) | 1 | the only published port (`localhost:8080`) |
| `writer` | 1 | receives every `POST`/`PUT`/`DELETE`; volume mounted read-write |
| `reader` | 2 | receive `GET`/`HEAD` round-robin; volume mounted **read-only** |

**How a request reaches an instance**

1. The client resolves `localhost` (local demo). In production a DNS record
   (for example `sheets.example.org`) would point to a load balancer's public
   address; that part is **not** part of this demo.
2. nginx receives the request. `deploy/nginx.conf` chooses the pool by method
   (`map $request_method $pool`): reads → `readers`, everything else → `writer`.
3. Inside the Compose network, Docker's embedded DNS server (`127.0.0.11`)
   resolves the service name `reader` to the IP of **every** reader replica
   (observed: `172.18.0.3` and `172.18.0.4`). nginx resolves it when it starts
   and balances round-robin across the addresses. A `zone` shares the
   round-robin position between nginx worker processes.
4. The instance handles the request and adds `X-Instance: <hostname>`, which
   makes the distribution visible.

**Health checks**

* Each app container has a Docker `HEALTHCHECK` (`wget /healthz` every 10 s);
  `lb` and `reader` only start after their dependencies are healthy.
* nginx performs *passive* checks: a connection error, timeout, 502 or 503
  makes it retry the request on the next server and avoid the failed one for
  10 s (`max_fails=1 fail_timeout=10s`). The writer is a `backup` for reads.
* nginx never retries a `POST` on another server, so a create is not sent
  twice by the load balancer; clients that retry use an `Idempotency-Key`.
* Observed: with one reader stopped, all reads went to the other; with both
  stopped, the writer answered (see [MANUAL_TEST_PLAN.md](MANUAL_TEST_PLAN.md)).

**What this is not**: Docker service names are not a production DNS or
service-discovery solution. nginx resolves `reader` only at start-up, so
replicas added later need an nginx reload; Docker does not remove unhealthy
containers from DNS; there is one load balancer (a single point of failure)
and no TLS. A production setup would use a managed load balancer or an
orchestrator (for example Kubernetes Services) with active health checks, and
a database server instead of a shared file.

## 8. Stateless instances

* **No session state**: there is no login, no cookies and no server-side
  session; every request carries all it needs (name in the URL, sheet in the
  body). The browser keeps an unsaved draft in its own `localStorage`, which
  never reaches the server's memory.
* **Durable state** lives in `characters.json` on the shared volume, so any
  reader can answer any read (observed in manual tests M-04 and M-05: a
  character written through the writer is read back from both readers).
* **Intentionally stateful parts of an instance**:
  * the idempotency keys (in-memory map) and the command mutex → this is why
    only **one** instance accepts writes;
  * the SRD API rate limiter (per instance, so N instances could send N×5
    requests/s; only reads trigger lookups);
  * parsed templates (read-only, identical in every instance).

So: the reader instances are stateless and interchangeable; the writer is a
deliberate single stateful component; the volume is the stateful supporting
service.

## 9. Error translation

| Error | HTTP status | CLI |
|---|---|---|
| `domain.ErrNotFound` (wrapped, e.g. `character "X" not found`) | 404 | prints the message |
| `application.ErrAlreadyExists`, `ErrIdempotencyKeyReused` | 409 | (CLI replaces instead) |
| rule violations, unknown race/class/item/spell, bad JSON | 400 | prints the message |
| external API failure | 502 (404 if the API does not know the name) | – |

Storage failures (for example a corrupt `characters.json`) are returned as
500 with the repository's message, which can include the file path. This is
inherited from the original program and listed as a known leak in
[DESIGN_PATTERNS.md](DESIGN_PATTERNS.md#leaky-abstractions-review).
