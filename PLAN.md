# SmartCourse — Plan

Roadmap, open decisions and progress. For how the system works see [docs/design.md](docs/design.md); for conventions and workflow see [AGENTS.md](AGENTS.md). Scope comes from [Requirements.md](Requirements.md); the 3-week delivery structure (mentor review at the end of each week) from [Guidelines.md](Guidelines.md).

**Now:** Week 1 mentor review; then Week 2 starts with 2.0 (decide D1–D4, D6).

## Open decisions

| #   | Decision                   | Proposal                                                                                            | Needed by    |
| --- | -------------------------- | --------------------------------------------------------------------------------------------------- | ------------ |
| D1  | Editing a published course | Draft revision + republish; students see the last published version                                 | Week 2       |
| D2  | Lesson types               | Video, document, article, external link; no quizzes                                                 | Week 2       |
| D3  | Lesson files               | Uploads to MinIO via presigned URLs                                                                 | Week 2       |
| D4  | Enrollment rules           | Free self-enrollment; optional capacity and prerequisites; drop and re-enroll with history          | Week 2       |
| D6  | Search                     | MongoDB text index inside Catalog                                                                   | Week 2       |
| W1  | Workers                    | Which jobs use Asynq/Temporal vs a hand-rolled worker pool (the PRD wants concurrency demonstrated) | Week 3 (3.3) |
| D5  | Completion                 | Student marks lessons complete → enrollment completed → certificate                                 | Stretch      |

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
- [x] 1.9 Code-first API with Huma + RFC 9457 errors (plan below)
- [x] 1.10 Tests (plan below)
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
- CI pipeline running `task check`, `api:check`, `test` and `test:integration` on every push (all already non-interactive), plus `test:e2e` against a composed stack
- Lesson progress, completion, certificates (D5) and the metrics they unlock
- RS256/JWKS; services verifying JWTs themselves; separate DB users per service (A9)
- Graceful shutdown for all services; refresh-token grace window for parallel clients
- Dedicated search engine if Mongo text search is not enough

## 1.9 Code-first API with Huma

**Decided (O1, 2026-10-20):** generate the OpenAPI spec from Go types with [Huma v2](https://huma.rocks) (`github.com/danielgtaylor/huma/v2`, on Gin via `adapters/humagin`), so routing, validation and docs come from one source and can't drift. Errors switch to **RFC 9457 problem details** (`application/problem+json`) with shared helpers. Done before 1.10 so tests target the final handler shape. Rejected: swaggest/openapi-go (operation list separate from routes, duplicated validation tags), swaggo (Swagger 2.0 only in stable), Fuego (pre-1.0), keeping the hand-written spec (drifts).

**Steps**

1. **`internal/platform/problem`: RFC 9457 helpers** (no Gin import, so the gateway and future workers can use it)
   - One error shape everywhere: Huma's `ErrorModel` (`type`, `title`, `status`, `detail`, `instance`, `errors[]` with `location`/`message`/`value`), overriding `huma.NewError` once so Huma's own validation errors and ours match.
   - `problem.From(err)`: maps `errs.Kind` sentinels to status + `detail` (replaces `web.statusFor`); unknown errors become a generic 500 and are logged, never exposed.
   - Constructors for the common cases: `problem.BadRequest/NotFound/Forbidden/Conflict/Unauthorized(detail)`, `problem.Field(location, message, value)` for field errors.
   - `problem.Write(w, status, detail)` for non-Huma paths: gateway 401/502, Gin 404/405, panic recovery.
   - `type` stays `about:blank` (RFC default, `title` = HTTP status text); `instance` carries the request path. Add the request ID as an extension once logging lands (3.6).
2. **`internal/platform/httpapi`: Huma setup shared by services**
   - `httpapi.New(router, title, version) huma.API`: humagin, OpenAPI 3.1, `bearerAuth` scheme, service docs routes disabled (the gateway serves the single public doc).
   - Actor middleware: reads the gateway's `X-User-*` into the request context; `httpapi.Actor(ctx)` replaces `web.Actor`. Internal headers stay out of the public spec.
   - Small registration helpers (`httpapi.Public(op)`, `httpapi.Protected(op)`) that set tags and security consistently.
3. **Convert handlers to typed operations** (Identity: 6, Catalog: 4)
   - Request DTOs: `struct{ Body ...; ID string \`path:"id" format:"uuid"\`; Limit int \`query:"limit" minimum:"1" maximum:"100" default:"20"\` }` with validation tags (`minLength`, `maxLength`, `format:"email"`, `enum`) instead of Gin `binding` tags.
   - **Response DTOs separate from storage models** (`UserResponse`, `CourseResponse`, `TokenResponse`) so schema changes in Postgres/Mongo never leak into the API; models lose their `json` tags.
   - Status and headers in output structs (`Status int`, `CacheControl string \`header:"Cache-Control"\``) for 201/204 and `no-store`.
   - Remove `web.Wrap`, `web.Bind`, `web.BindQuery`; keep `web.NewRouter` and `web.Health`.
   - Behaviour changes to accept and document: validation failures return **422** (malformed JSON stays 400); unknown JSON fields are rejected; error bodies become problem+json.
4. **Spec generation**
   - `cmd/openapi` builds the Identity and Catalog Huma APIs without starting servers (registration needs no live dependencies), merges paths and components (fails on schema-name clashes), adds gateway-level parts (`/ping`, servers, security, gateway 401/502 responses), and writes `api/openapi.yaml`.
   - `task api:gen` regenerates; `task api:check` = gen + `git diff --exit-code api/openapi.yaml` + Redocly lint. The hand-written spec is replaced by the generated file; the gateway keeps serving it at `/docs`.
5. **Gateway**: same routing; its own errors (401, 502, unknown route) switch to `problem.Write`.
6. **Docs and checks**: update design.md (§2.3, §7, decision O1), AGENTS.md (handler/DTO/error conventions), README (error format); update the smoke script for 422 and problem bodies.

**Done when:** all 10 operations run through Huma, `api/openapi.yaml` is generated and `task api:check` passes, every error response (services and gateway) is problem+json, the 45-check smoke script passes against `task up`, and `task check` is clean.

**Outcome (deviations from the steps above):** no problem constructors were needed (handlers return domain sentinels and Huma's `ErrorModel` is already the shared shape); `huma.NewError` is overridden (`problem.NewError`, installed by `httpapi.Config`) only to stop echoing submitted values, added after 1.9 in `675cbd1`; `instance` is omitted because Huma's schema declares it an absolute URI; one Huma API registering both services replaces the merge step (Huma panics on schema-name clashes); domain `Invalid` errors are 422 like Huma's validation errors. Smoke script extended to 52 checks (400 malformed JSON, unknown field, bad UUID, 405, 404, 502 verified by hand).

## 1.10 Test plan

Goal: prove the rules that matter (access control, token security, session rotation, validation) at the cheapest layer that can prove them, and keep the OpenAPI contract honest.

| Layer                 | Scope                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Deps                                       | Build tag / task                             |
| --------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------ | -------------------------------------------- |
| Unit                  | `policy.go` (both services); `auth.Tokens` (valid, expired, wrong secret, `alg` none/HS512, wrong issuer, no `exp`, bad `sub`/role, short secret); password/email rules; `problem.From` status mapping; Huma operations through `humatest` (validation → 422, actor middleware, problem bodies); gateway `RequestID`, `StripIdentityHeaders`, `Authenticate` via `httptest`; Catalog `Service` and Identity `Service`/`AuthService` with in-memory fakes (role checks, 404 vs 403, list filters per role, limit clamping, login/refresh/logout paths) | none                                       | `task test` (`go test -race ./...`)          |
| Integration           | Identity Postgres repository (unique email → `ErrEmailTaken`), Catalog Mongo repository (UUID codec round-trip, visibility filter, pagination order), Redis `TokenStore` (rotate, reuse revokes family, TTLs, N concurrent rotations → exactly one wins)                                                                                                                                                                                                                                                                                              | real Postgres/Mongo/Redis (testcontainers) | `-tags integration`, `task test:integration` |
| End-to-end + contract | Go port of the 45-check smoke script: register → login → courses → refresh/reuse → logout → admin, every response validated against `api/openapi.yaml` (`kin-openapi`)                                                                                                                                                                                                                                                                                                                                                                                | `task up`                                  | `-tags e2e`, `task test:e2e`                 |

**Test utilities** (`internal/platform/testutil`, imported only from `_test.go` files). Decided: assertions with the standard library + `github.com/google/go-cmp` (T1: idiomatic per the Go team's test guidance, readable diffs, `cmpopts` for timestamps/IDs, no assertion-library lock-in); real databases via `testcontainers-go` (T2: self-contained, CI-ready, no dependency on a running stack).
- `testutil.Actor(role)` / fixed UUIDs; `testutil.Tokens(t)` with a fixed 32-byte secret and a controllable clock.
- HTTP: Huma's `humatest` for service operations; `testutil.Do(t, handler, method, path, body, headers)` → recorder for the gateway; `testutil.Problem(t, rec)` decodes and checks a problem+json body.
- Containers: `testutil.Postgres(t, "identity")`, `testutil.Mongo(t)`, `testutil.Redis(t)`, each applying `migrations/<svc>` and cleaning up with `t.Cleanup`.
- Contract: `testutil.OpenAPI(t)` loading the generated `api/openapi.yaml` and validating `*http.Response`s (catches gateway-level drift that Huma can't see, e.g. 401/502 bodies).
- Fakes stay in each package's `_test.go` files (in-memory `Repository` / `TokenStore`), not shared.

**Small refactors for testability:** inject a clock into `auth.Tokens` and `AuthService` (expiry tests without sleeping); expose `restrictToVisible`'s filter to assert per-role queries.

**Done when:** all three layers pass locally, `go test -race ./...` is clean, Taskfile has `test`, `test:integration`, `test:e2e`, and the contract test fails if the spec and the API drift.

**Outcome:** done as planned, with these simplifications: no clock injection was needed (`Tokens.Sign` already takes `now`, so expiry is tested by signing in the past); `restrictToVisible` is tested in-package and via the fake repository's recorded filter; migrations are applied in tests by a small reader of `migrations/<svc>` (SQL executed, Mongo JSON commands run) instead of the golang-migrate library; the e2e layer lives in `test/e2e`, also checks that the gateway serves the committed spec, and skips the Redis/Postgres internals the smoke script inspected via `docker exec` (covered by integration tests); the contract validator lives in `test/e2e` rather than `testutil.OpenAPI` (its only user), and gateway tests run against `httptest.NewServer` instead of a `testutil.Do` recorder (Gin's `CloseNotify` panics on a `ResponseRecorder` inside `ReverseProxy`). Verified that the validator rejects wrong types, extra fields, the old `{"error"}` shape and undocumented statuses. golangci-lint now lints the tagged files too.

## Progress log

| Date               | Milestone                                                                                                                       | Commit                |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------- | --------------------- |
| 2026-09-07 → 10-20 | Monolith foundation: tooling, Gin, Postgres + GORM + migrations, users and courses CRUD, shared error/web helpers               | `6d02497` … `71333ec` |
| 2026-10-20         | Switched to microservices + MongoDB; architecture and access rules approved (1.1)                                               | `79ac37b`             |
| 2026-10-20         | 1.2 + 1.3: `internal/platform`, Identity service                                                                                | `e972e47`             |
| 2026-10-20         | 1.4: Catalog on MongoDB with visibility/ownership rules                                                                         | `38821b7`             |
| 2026-10-20         | 1.5: Gateway reverse proxy, request IDs                                                                                         | `dadd26a`             |
| 2026-10-20         | 1.6: Auth (register/login/refresh/logout, JWT, admin CLI)                                                                       | `039010b`             |
| 2026-10-20         | Refresh tokens moved to Redis (atomic Lua rotation)                                                                             | `417ce64`             |
| 2026-10-20         | 1.7: Whole stack in Docker (`task up`)                                                                                          | `a132521`             |
| 2026-10-20         | 1.8: README, design doc, AGENTS.md, OpenAPI 3.1 + Swagger UI at `/docs`; PLAN trimmed; migration 000004 deleted                 | `7902868`             |
| 2026-10-20         | Migration 000003 deleted (no `refresh_tokens` table; local DB rolled back to v2); test tooling decided (go-cmp, testcontainers) | `4a86031`             |
| 2026-10-20         | Decided O1: Huma code-first API + RFC 9457 errors (planned as 1.9; tests move to 1.10)                                          | `b50a9c6`             |
| 2026-10-20         | 1.9: 10 operations on Huma, generated `api/openapi.yaml` (`task api:gen`/`api:check`), problem+json everywhere, `Invalid` → 422 | `625f603`             |
| 2026-10-20         | 1.10: unit, integration (testcontainers) and e2e + OpenAPI contract tests; `task test`, `test:integration`, `test:e2e`          | `a380f20`             |
| 2026-10-20         | Error details no longer echo submitted values (passwords in 422s, raw body in 400s)                                             | `675cbd1`             |
| 2026-10-20         | Docs brought up to date after 1.9/1.10 (test tooling decisions in design.md, AGENTS gotchas, CI gap noted)                      | *(this commit)*       |
