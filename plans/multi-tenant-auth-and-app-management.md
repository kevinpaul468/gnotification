# Multi-Tenant Auth, App Management, and Per-App Provider Configs

## Problem Statement

The notification service is single-tenant: all provider configs are global, there's no authentication, no app/client management, and no way to track per-app usage. Adding a new client requires shared secrets and manual DB operations. The worker initializes providers once at startup with global config, so per-app overrides (like different `from` email addresses) aren't possible without code changes.

## Solution

Introduce a multi-tenant architecture with:
- An `App` model to register clients with allowed provider permissions
- `Authorization: Bearer <key>` middleware that authenticates requests and sets `app_id`
- Per-app provider configs that merge with global defaults (merge resolution model)
- Provider permission enforcement at send time
- App management API (register, API key generation, usage tracking)
- Worker resolves per-app provider configs at notification processing time

## Commits

### Phase 1 — Models and DB Layer

1. **Add `App` model** with fields: `ID`, `Name`, `Description`, `AllowedProviders` (JSON array of provider names), `CreatedAt`, `UpdatedAt`. Add migration.
2. **Add `app_id` column to `ProviderConfig`** (nullable). When `NULL` the config is global; when set it's per-app. Add composite index on `(provider, app_id)`.
3. **Add `AppID` field to `queue.Message`** so the worker knows which app a notification belongs to.
4. **Add DB query: `GetProviderConfig(provider, appID)`** — returns the app-specific config if it exists, otherwise falls back to the global one.
5. **Add DB query: `GetMergedProviderConfig(provider, appID)`** — loads the global config JSON and app-specific config JSON (if exists), deep-merges them, returns the merged result.
6. **Add DB query: `GetNotificationsByApp(appID, limit, offset)`** — fetches paginated notifications for an app (for usage tracking).
7. **Add `AppID` to the notification create + publish path** in the handler and queue message.

### Phase 2 — Auth Middleware

8. **Implement `AuthMiddleware`** — extracts `Authorization: Bearer <key>` from request, hashes with SHA256, looks up in `api_keys` table, sets `app_id` and `allowed_providers` in Echo context. Returns 401 if invalid.
9. **Wire middleware into server** — register it before the notification routes.

### Phase 3 — App Management API

10. **Add handler: `POST /apps`** — registers a new app (name, description), creates the app row and a corresponding API key, returns the API key in the response (only time the raw key is shown).
11. **Add handler: `GET /apps`** — lists all registered apps and their metadata.
12. **Add handler: `GET /apps/:id`** — returns a single app's details.
13. **Add handler: `POST /apps/:id/api-keys`** — generates a new API key for the app (useful for rotation).
14. **Add handler: `DELETE /api-keys/:id`** — revokes an API key.
15. **Add handler: `GET /apps/:id/usage`** — returns paginated notification history for the app.

### Phase 4 — Permission Validation

16. **Add permission check in `SendNotification` handler** — after auth middleware sets `allowed_providers`, validate that the requested provider is in the app's allowed list. Return 403 if not.

### Phase 5 — Worker Provider Resolution

17. **Refactor worker `InitializeProviders`** — load global configs into a separate cache. Add lazy per-app provider resolution: when a notification arrives, look up the app's merged config from DB, create a provider instance with the merged config, cache it by `(appID, provider)`.
18. **Update `processMessage`** — use the app-scoped provider instance instead of the global one. Pass `AppID` from the queue message through to the resolution logic.
19. **Add cache invalidation stub** — log a warning if per-app config is not found (graceful fallback to global).

### Phase 6 — Tests

20. **Set up test infrastructure** — create `db_test.go` helper with test database (or use an in-memory SQLite with GORM compatible queries), create `handler_test.go` with Echo test helpers.
21. **Write tests for DB queries** — `GetProviderConfig` fallback logic, `GetMergedProviderConfig` merge behavior, `GetNotificationsByApp` pagination.
22. **Write tests for auth middleware** — valid key, invalid key, missing header, expired/revoked key.
23. **Write tests for app management handlers** — create app, list apps, generate API key, delete API key, duplicate name handling.
24. **Write tests for permission validation** — app with `smtp` permission denied for `sms`, wildcard/all-providers permission.
25. **Write tests for provider config merge logic** — global-only, per-app only, global + per-app merge, per-app overrides specific fields.

## Decision Document

- **One key per app**. API keys are app-level credentials.
- **Merge resolution model**: per-app config values are deep-merged over the global config. A per-app config with `{"from": "support@acme.com"}` merges with global `{"host": "...", "port": 587, ...}` to produce the full config used for initialization.
- **Provider permissions are app-level**, stored as a JSON array on the App model (e.g., `["smtp", "sms"]`).
- **Worker resolves providers lazily**: global configs loaded at startup, per-app configs loaded on first notification and cached by `(appID, provider)`.
- **Auth uses `Authorization: Bearer`** header with SHA256-hashed keys (same as existing `hashAPIKey` function).
- **App management endpoints are unprotected** in v1 (no admin auth). Can be locked behind an admin key later.
- **Raw API key is shown only once** at creation time (standard security practice).
- **`provider_configs` table** gets a nullable `app_id` column. `app_id = NULL` means global. A valid app-specific config does not need a corresponding global config to exist (standalone per-app providers like Firebase are valid).
- **Usage endpoint returns paginated notifications** ordered by `created_at DESC`.

## Testing Decisions

- **Good test**: tests external behavior (HTTP response codes, DB query results, auth pass/fail) without testing implementation details like which GORM method was called.
- **Modules to test**: `pkg/database` (DB queries), `internal/handlers` (HTTP handlers + middleware), `pkg/providers` (config merge utility).
- **Prior art**: No existing tests in the repo. Use Go's standard `testing` package with `httptest` for HTTP tests and a test PostgreSQL database.
- **Worker tests**: Mock the `Provider` interface and verify that the correct provider instance (per-app) is invoked with the right config.

## Out of Scope

- Admin-level auth for management endpoints (all treated as unprotected for now).
- Dynamic hot-reload of provider configs without worker restart (cached at startup, stale until restart).
- Template storage/render engine (content is still caller-provided).
- Rate limiting per app.
- Webhook delivery status callbacks.
- Separate SMS/Firebase provider implementations (still stubs).
