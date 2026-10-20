# SmartCourse — Plan

Roadmap, open decisions and progress. For how the system works see [docs/design.md](docs/design.md); for conventions and workflow see [AGENTS.md](AGENTS.md). Scope comes from [Requirements.md](Requirements.md); the 3-week delivery structure (mentor review at the end of each week) from [Guidelines.md](Guidelines.md).

**Now:** Week 1, item 1.9 (tests). Next after that: Week 1 mentor review.

## Open decisions

| #   | Decision                   | Proposal                                                                                         | Needed by |
| --- | -------------------------- | ------------------------------------------------------------------------------------------------ | --------- |
| T1  | Test assertions            | stdlib + `go-cmp` vs `testify`                                                                   | 1.9       |
| T2  | Integration test infra     | `testcontainers-go` (Postgres, Mongo, Redis) vs reuse `task infra:up` with throwaway databases   | 1.9       |
| M1  | Unused `refresh_tokens` table | Migration 000003 still creates it (tokens moved to Redis, 000004 deleted). Delete 000003 too (local DB back to v2) or keep | 1.9 |
| D1  | Editing a published course | Draft revision + republish; students see the last published version                             | Week 2    |
| D2  | Lesson types               | Video, document, article, external link; no quizzes                                              | Week 2    |
| D3  | Lesson files               | Uploads to MinIO via presigned URLs                                                              | Week 2    |
| D4  | Enrollment rules           | Free self-enrollment; optional capacity and prerequisites; drop and re-enroll with history        | Week 2    |
| D6  | Search                     | MongoDB text index inside Catalog                                                                | Week 2    |
| W1  | Workers                    | Which jobs use Asynq/Temporal vs a hand-rolled worker pool (the PRD wants concurrency demonstrated) | Week 3 (3.3) |
| D5  | Completion                 | Student marks lessons complete → enrollment completed → certificate                              | Stretch   |

## Roadmap

### Week 1 — Foundation + core services
- [x] 1.1 Architecture: service map, data ownership, access rules (design.md §2, §5)
- [x] 1.2 Restructure into services (`cmd/<svc>`, `internal/<svc>`, `internal/platform`)
- [x] 1.3 Identity service
- [x] 1.4 Catalog service on MongoDB with visibility/ownership rules
- [x] 1.5 API gateway (reverse proxy, request IDs, header stripping)
- [x] 1.6 Auth: register/login/refresh/logout, JWT + rotating refresh tokens (Redis), admin bootstrap
- [x] 1.7 Local setup: `task up` runs infra, migration jobs and all services in Docker
- [x] 1.8 Docs: README, docs/design.md, AGENTS.md, OpenAPI 3.1 contract served at `/docs`
- [ ] 1.9 Tests (plan below)
- [ ] Week 1 mentor review

### Week 2 — Enrollment + publishing workflow
- [ ] 2.0 Decide D1–D4, D6
- [ ] 2.1 Content structure: modules and lessons embedded in the course (create, list, rename, delete, reorder)
- [ ] 2.2 Course state machine: `draft → publishing → published | publish_failed`, `published → archived`
- [ ] 2.3 Temporal publish workflow: parallel validation, then parallel side effects; idempotent activities, retries, partial failure; goroutines/WaitGroup/channels/context demonstrated
- [ ] 2.4 Enrollment service: enroll, drop, history; one active enrollment per student+course, `Idempotency-Key`; capacity and prerequisite checks
- [ ] 2.5 Enrollment → Catalog course-state lookup (sync; replaced by an event-fed replica in 3.2)
- [ ] 2.6 Tests: concurrent enroll (duplicate and capacity races), Temporal workflow tests
- [ ] Week 2 mentor review

### Week 3 — Events + observability
- [ ] 3.1 Kafka + Schema Registry (Protobuf); transactional outbox in Catalog and Enrollment; `twmb/franz-go`
- [ ] 3.2 Consumers: Enrollment course replica, idempotent handling, dead-letter topic
- [ ] 3.3 Notification service + workers (Asynq; decide W1); bounded pool, graceful shutdown, retries, backpressure
- [ ] 3.4 Analytics service (MongoDB): PRD metrics from events; `sync/atomic` counters
- [ ] 3.5 Redis course cache (+ `sync.RWMutex` in-memory layer) and gateway rate limiting (incl. login)
- [ ] 3.6 Observability: `slog` with request/trace IDs, OpenTelemetry → Jaeger, Prometheus + Grafana
- [ ] 3.7 Final deliverables: docs complete, end-to-end flows, `go test -race` clean
- [ ] Week 3 mentor review

### Stretch
- Lesson progress, completion, certificates (D5) and the metrics they unlock
- RS256/JWKS; services verifying JWTs themselves; separate DB users per service (A9)
- Graceful shutdown for all services; refresh-token grace window for parallel clients
- Dedicated search engine if Mongo text search is not enough

## 1.9 Test plan

Goal: prove the rules that matter (access control, token security, session rotation, validation) at the cheapest layer that can prove them, and keep the OpenAPI contract honest.

| Layer | Scope | Deps | Build tag / task |
| ----- | ----- | ---- | ---------------- |
| Unit | `policy.go` (both services); `auth.Tokens` (valid, expired, wrong secret, `alg` none/HS512, wrong issuer, no `exp`, bad `sub`/role, short secret); password/email rules; `web.Actor`, `web.WriteError` status mapping; gateway `RequestID`, `StripIdentityHeaders`, `Authenticate` via `httptest`; Catalog `Service` and Identity `Service`/`AuthService` with in-memory fakes (role checks, 404 vs 403, list filters per role, limit clamping, login/refresh/logout paths) | none | `task test` (`go test -race ./...`) |
| Integration | Identity Postgres repository (unique email → `ErrEmailTaken`), Catalog Mongo repository (UUID codec round-trip, visibility filter, pagination order), Redis `TokenStore` (rotate, reuse revokes family, TTLs, N concurrent rotations → exactly one wins) | real Postgres/Mongo/Redis (T2) | `-tags integration`, `task test:integration` |
| End-to-end + contract | Go port of the 45-check smoke script: register → login → courses → refresh/reuse → logout → admin, every response validated against `api/openapi.yaml` (`kin-openapi`) | `task up` | `-tags e2e`, `task test:e2e` |

**Test utilities** (`internal/platform/testutil`, imported only from `_test.go` files):
- `testutil.Actor(role)` / fixed UUIDs; `testutil.Tokens(t)` with a fixed 32-byte secret and a controllable clock.
- HTTP: `testutil.Do(t, handler, method, path, body, headers)` → recorder; `testutil.JSON[T](t, rec)`.
- Containers (T2): `testutil.Postgres(t, "identity")`, `testutil.Mongo(t)`, `testutil.Redis(t)`, each applying `migrations/<svc>` and cleaning up with `t.Cleanup`.
- Contract: `testutil.OpenAPI(t)` loading `api/openapi.yaml` and validating `*http.Response`s.
- Fakes stay in each package's `_test.go` files (in-memory `Repository` / `TokenStore`), not shared.

**Small refactors for testability:** inject a clock into `auth.Tokens` and `AuthService` (expiry tests without sleeping); expose `restrictToVisible`'s filter to assert per-role queries.

**Done when:** all three layers pass locally, `go test -race ./...` is clean, Taskfile has `test`, `test:integration`, `test:e2e`, and the contract test fails if the spec and the API drift.

## Progress log

| Date       | Milestone | Commit |
| ---------- | --------- | ------ |
| 2026-09-07 → 10-20 | Monolith foundation: tooling, Gin, Postgres + GORM + migrations, users and courses CRUD, shared error/web helpers | `6d02497` … `71333ec` |
| 2026-10-20 | Switched to microservices + MongoDB; architecture and access rules approved (1.1) | `79ac37b` |
| 2026-10-20 | 1.2 + 1.3: `internal/platform`, Identity service | `e972e47` |
| 2026-10-20 | 1.4: Catalog on MongoDB with visibility/ownership rules | `38821b7` |
| 2026-10-20 | 1.5: Gateway reverse proxy, request IDs | `dadd26a` |
| 2026-10-20 | 1.6: Auth (register/login/refresh/logout, JWT, admin CLI) | `039010b` |
| 2026-10-20 | Refresh tokens moved to Redis (atomic Lua rotation) | `417ce64` |
| 2026-10-20 | 1.7: Whole stack in Docker (`task up`) | `a132521` |
| 2026-10-20 | 1.8: README, design doc, AGENTS.md, OpenAPI 3.1 + Swagger UI at `/docs`; PLAN trimmed; migration 000004 deleted | *(this commit)* |
