# SmartCourse — Build Plan & Progress

> **Purpose of this file:** single source of truth for where this project is going and where it currently stands, so any session/agent picking this up (fresh context, no chat history) can continue without re-deriving decisions already made. Update the **Progress Log** and checkboxes as work lands — don't let this drift from reality.
>
> **How the owner wants to work (read this before doing anything):**
> - Learner-driven, incremental build — one small vertical slice at a time (e.g. "just linting", "just .env", "just Gin + one route"), never several phases at once.
> - **Guide, don't implement.** The owner writes the code themselves. Explain *what* to do, *why*, library alternatives/trade-offs, and give code snippets to reference — but don't create/edit application files unless explicitly asked to. (This PLAN.md file itself was an explicit exception — the owner asked for it directly.)
> - When reviewing "done" work: actually read the files and run `task check` / build / a quick manual request — don't take "done" at face value, verify it.
> - New to Go (only completed the Tour of Go) — explain idiomatic patterns as they come up (struct tags, error wrapping, `internal/` visibility, etc.), don't assume prior Go-specific knowledge.

---

## 1. What SmartCourse is

Backend for EduCorp's learning platform: course management + publishing workflow, student enrollment, analytics, background/event-driven processing, built to demonstrate Go concurrency (goroutines, channels, worker pools, sync primitives, context propagation) and production-grade observability/reliability patterns.

Full original PRD (business goals, functional requirements, analytics metrics, tech stack wishlist) was provided by the owner at project kickoff — not reproduced here in full; ask the owner if the source PRD text is needed verbatim.

## 2. Architecture decision: Path A — Modular Monolith

Explicitly chosen over a true microservices split (evaluated both, owner picked this).

- **One Go module, one deployable API binary for now** (`cmd/api`), more binaries later as needed (e.g. `cmd/worker` once Asynq/Temporal land) — still the same module, still deployed together.
- **`internal/` organized by domain, not by technical layer.** i.e. `internal/course`, `internal/enrollment`, `internal/user`, each eventually owning its own handler/service/repository/model — never a global `internal/handlers`, `internal/services`, `internal/repositories`.
- **Extraction-ready discipline** (so a domain can become a real microservice later without a rewrite):
  1. A domain package talks to another domain only through an exported interface/function — never by reaching into another domain's DB tables or internals directly.
  2. Cross-domain side effects (course published → search/cache/analytics; student enrolled → progress/analytics/notify) go through an event or a defined function boundary, not shared ad-hoc state.
  3. Each domain owns its own persistence code, even while sharing one physical Postgres instance.
- **No `pkg/` directory** — only add it if something is genuinely meant for external/other-module reuse. Not the case yet.

## 3. Running conventions (established, keep following these)

- **Config grows with consumers, not ahead of them.** Don't add an env var / config struct field until the code that actually reads it exists in the same change. (Applied so far: `AppEnv` added with the config step itself; `Port` added exactly when Gin needed to bind one.)
- **`.env.example` is committed, `.env` is gitignored.** Real env vars in prod; `.env` is a local-dev-only convenience (`godotenv.Load()` errors are deliberately ignored — see `internal/config/config.go`).
- **Task runner is `go-task`** (`Taskfile.yml`), not Make. Standard tasks: `task tidy`, `task fmt`, `task lint`, `task check` (fmt+lint, run before every commit), `task build`, `task run`, `task test`.
- **Linting via `golangci-lint` v2** (`.golangci.yml` — note v2's schema split `linters:` from `formatters:`; don't paste v1-style flat config from older tutorials).
- **Commit at clean checkpoints** — after a vertical slice works end-to-end and `task check` is clean, not mid-slice.
- **Module path:** `github.com/all2pie/go-smart-course`. Go version: 1.27.1 (satisfies the PRD's "Go 1.25+").

## 4. Tech stack — decided vs. deferred

### Decided / in use
| Concern | Choice | Why (brief — full reasoning was given in chat, ask owner if it needs restating) |
|---|---|---|
| HTTP framework | Gin | Given in original PRD wishlist |
| .env loading | `joho/godotenv` | Minimal, single-purpose |
| Env → struct config | `caarlos0/env/v11` | Zero-dep, struct-tag based, pairs with godotenv |
| Linting | `golangci-lint` v2 | Aggregates `govet`, `staticcheck`, `errcheck`, `unused`, `ineffassign` (via `default: standard`) + `misspell`, `unconvert`, `unparam` |
| Formatting | `gofmt` + `goimports` (via `golangci-lint fmt`) | Standard Go formatting + import grouping |
| Task runner | `go-task` (Taskfile) | Chosen over Make/plain scripts by owner |

### Decided in principle, not yet implemented
| Concern | Choice | Notes |
|---|---|---|
| Relational DB | PostgreSQL + GORM | Users/courses/enrollment — relational, transactional |
| Document DB | MongoDB | Deferred until something genuinely unstructured needs it (content blocks, raw analytics events) — not day one |
| Cache/queues backing | Redis | Not yet wired |
| Event streaming | Kafka + Confluent Schema Registry | Not yet wired |
| Durable workflows | Temporal (Go SDK) | For the multi-step course-publish and enrollment sagas specifically |
| Background jobs | Asynq (Redis-based) | For simple fire-and-forget jobs (notifications, analytics bumps) — **not** RabbitMQ+native workers, redundant given Redis is already in-stack |
| Kafka client | `twmb/franz-go` | Pure Go, no CGO, chosen over `confluent-kafka-go` (CGO/librdkafka) and `segmentio/kafka-go` (less actively maintained) |
| Logging | stdlib `log/slog` | Not yet wired |
| Observability | OpenTelemetry + Prometheus/Grafana + Jaeger | Not yet wired |
| Validation | `go-playground/validator` | Not yet wired |
| Migrations | `golang-migrate/migrate` | Not yet wired — do NOT rely on GORM `AutoMigrate` beyond prototyping |

### Open architectural question — flagged, not resolved
The PRD explicitly wants hand-rolled goroutines/channels/worker-pools/sync primitives *demonstrated*, but Asynq/Temporal would otherwise implement that for you. **Decision needed before background processing work starts:** which specific workloads get a hand-rolled worker pool (learning exercise) vs. which lean on Asynq/Temporal (reliability, no reinvention). Suggested split floated in chat: Temporal for the stateful multi-step workflows (publish, enrollment saga), hand-rolled worker pools for simpler high-volume jobs (e.g. notification dispatch) where the concurrency patterns are the point. **Not yet decided — revisit when reaching section 6's background-processing phase.**

## 5. Progress log

| Date | Milestone | Commit |
|---|---|---|
| 2026-09-07 | Project scaffolded: `cmd/api`, `internal/config`, module init | `6d02497` |
| 2026-09-07 | Linting (`golangci-lint` v2) + formatting + Taskfile set up | `6d02497` |
| 2026-09-07 | `.env` support end-to-end (godotenv + caarlos0/env), trimmed to only `APP_ENV` at the time | `6d02497` |
| 2026-09-14 | Gin HTTP server added: `internal/server`, `GET /ping`, `Port` config field added (config-grows-with-consumer applied), `gin.SetMode` driven by `AppEnv`, trusted proxies disabled | `ef3195a` |

**Current repo state:**
```
cmd/api/main.go          — thin: config.Load() → server.Run()
internal/config/config.go — AppEnv, Port
internal/server/server.go — Gin engine, GET /ping only
.golangci.yml, Taskfile.yml, .gitignore, .env.example — all in place
```
No database, no other domain packages, no background processing, no observability yet.

## 6. Roadmap — remaining phases, in intended order

- [x] **Phase 0 — Tooling bootstrap:** lint, format, Taskfile, `.env`/config
- [x] **Phase 1 — HTTP server:** Gin, one health route
- [ ] **Phase 2 — Database layer (next up):**
  - [ ] Postgres connection config (`DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASSWORD`/`DB_NAME`) added to `Config`
  - [ ] `docker-compose.yml` with local Postgres (first Docker Compose file in the project)
  - [ ] `internal/db` (or similar) package: opens/exposes the GORM connection
  - [ ] First domain: `internal/user` (id, email, role, timestamps) — chosen first because course/enrollment both depend on users existing
  - [ ] `golang-migrate` wired, one migration for the users table
  - [ ] One proven round-trip (create + fetch a user) through a real route
- [ ] **Phase 3 — Course domain:** `internal/course` — courses, modules, learning assets; CRUD; still no publishing workflow yet (that's Phase 5)
- [ ] **Phase 4 — Enrollment domain:** `internal/enrollment` — enroll/duplicate rules/history; still synchronous only (no async side effects yet — that's Phase 6)
- [ ] **Phase 5 — Content publishing workflow (concurrency showcase #1):** validation, asset verification, search-index update, cache refresh, analytics init — run concurrently where dependencies allow; goroutines + WaitGroups + channels + context propagation; "Ready" state only after all steps succeed; partial-failure handling
- [ ] **Phase 6 — Enrollment workflow + background processing:** progress init, analytics update, notifications; resolve the Asynq/Temporal-vs-hand-rolled-pool question from section 4 before starting; worker pool implementation (fixed size, job queue, graceful shutdown, retries, backpressure)
- [ ] **Phase 7 — Shared resource synchronization:** in-memory caches, enrollment/analytics counters, popularity stats, rate-limiter state — `sync.Mutex`/`sync.RWMutex`/`sync/atomic`, explicitly tested against races (`go test -race`)
- [ ] **Phase 8 — Redis + caching layer**
- [ ] **Phase 9 — Kafka + Schema Registry:** event-driven propagation for publish/enrollment side effects
- [ ] **Phase 10 — Temporal:** durable workflow orchestration for publish + enrollment sagas
- [ ] **Phase 11 — MongoDB:** wherever unstructured data actually shows up (content blocks / raw analytics events) — don't introduce speculatively
- [ ] **Phase 12 — Analytics metrics:** the specific metrics list from the PRD (total students/instructors/courses, enrollments over time, completion rate, avg time-to-complete, most popular courses, avg courses/student, failed events)
- [ ] **Phase 13 — Observability:** `log/slog` structured logging → OpenTelemetry → Prometheus/Grafana → Jaeger
- [ ] **Phase 14 — Dockerization:** Dockerfile(s) + full `docker-compose.yml` (all services), replacing the Phase 2 Postgres-only compose file
- [ ] **Phase 15 — Hardening pass:** migrations review, `golangci-lint` strictness review, test coverage review, `-race` sweep across concurrency code

## 7. How to resume this project in a fresh session

1. Read this file in full first.
2. Run `git log --oneline` and `find . -type f` (excluding `.git`) to confirm reality matches section 5's stated state — **the code is the source of truth if this file and the code ever disagree.**
3. Check the roadmap in section 6 for the next unchecked item — that's the next slice, not the whole remaining phase at once.
4. Re-read section "How the owner wants to work" at the top before doing anything.
5. After landing a slice: verify it (`task check`, build, a manual request/test), update section 5's progress log and tick the relevant box(es) in section 6, then stop and hand back — don't cascade into the next phase unprompted.
