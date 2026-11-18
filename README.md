# JWT Auth: Go + React + PostgreSQL

Register and log in with JWT. A protected page shows `Hello [email], welcome back`. You are logged out after 15 minutes of inactivity.

| Layer    | Stack                                                        |
|----------|--------------------------------------------------------------|
| Backend  | Go 1.26, `net/http`, `pgx`, `golang-jwt/jwt/v5`, bcrypt      |
| Frontend | React 18 + TypeScript (Vite), React Router                   |
| Database | PostgreSQL 16                                                |
| Runtime  | Docker Compose (db, backend, frontend served by nginx)       |

## Quick start (Docker)

Requirements: Docker with Docker Compose v2.

```bash
cp .env.example .env          # then set JWT_SECRET to a random string of 32+ characters
docker compose up --build
```

- Frontend: http://localhost:3000
- Backend API: http://localhost:8080 (the frontend also proxies `/api` to it)
- PostgreSQL: localhost:5432

On startup the backend applies the SQL files in `backend/internal/db/migrations` that have not run yet (tracked in the `schema_migrations` table), creating the `users`, `menus`, `roles` and `role_menus` tables. To stop and remove the data: `docker compose down -v`.

## Environment variables

| Variable            | Required | Default   | Description                                                     |
|---------------------|----------|-----------|-----------------------------------------------------------------|
| `JWT_SECRET`        | yes      | none      | HS256 signing secret, at least 32 characters (`openssl rand -hex 32`) |
| `JWT_TTL`           | no       | `15m`     | Token lifetime and inactivity timeout (Go duration: `15m`, `1m`) |
| `POSTGRES_USER`     | no       | `app`     | Database user                                                    |
| `POSTGRES_PASSWORD` | no       | `app`     | Database password                                                |
| `POSTGRES_DB`       | no       | `authdb`  | Database name                                                    |
| `FRONTEND_PORT`     | no       | `3000`    | Host port for the web app                                        |
| `BACKEND_PORT`      | no       | `8080`    | Host port for the API                                            |
| `DB_PORT`           | no       | `5432`    | Host port for PostgreSQL                                         |
| `SWAGGER_ENABLED`   | no       | `false`   | Serve Swagger UI at `/swagger/`. Keep `false` in production      |
| `SEED_DEMO_USERS`   | no       | `false`   | Create the [demo users](#demo-users) on startup. Keep `false` in production |

When running the backend outside Docker it reads `DATABASE_URL` (e.g. `postgres://app:app@localhost:5432/authdb?sslmode=disable`), `JWT_SECRET`, `JWT_TTL` and `PORT` (default `8080`). Compose builds `DATABASE_URL` from the `POSTGRES_*` values.

## API

| Method | Path                 | Auth   | Body                                   | Response |
|--------|----------------------|--------|----------------------------------------|----------|
| POST   | `/api/auth/register` | none   | `{email, password, confirmPassword}`   | `201 {id, email}`; `400 {errors}`; `409` if the email is taken |
| POST   | `/api/auth/login`    | none   | `{email, password}`                    | `200 {token, expiresAt}`; `401` |
| POST   | `/api/auth/refresh`  | Bearer | none                                   | `200 {token, expiresAt}` (new token) |
| GET    | `/api/me`            | Bearer | none                                   | `200 {id, email, message: "Hello <email>, welcome back"}` |
| GET    | `/api/menus`         | Bearer | none                                   | `200 {menus: [{id, title, icon, path, children}], role}`: the sidebar menu for the caller's role |
| GET    | `/api/roles`         | Bearer + Role menu | none                       | `200 {roles: [{id, name, allAccess, menuIds, userCount}], menus}` |
| POST   | `/api/roles`         | Bearer + Role menu | `{name, allAccess, menuIds}` | `201` role; `400`; `409` if the name exists |
| PUT    | `/api/roles/{id}`    | Bearer + Role menu | `{name, allAccess, menuIds}` | `200` role; `400` if it would remove your own access to the Role page |
| DELETE | `/api/roles/{id}`    | Bearer + Role menu | none                       | `204`; `409` while users still have the role |
| GET    | `/api/users`         | Bearer + User menu | none                       | `200 {users: [{id, email, roleId}], roles: [{id, name}]}` |
| PUT    | `/api/users/{id}/role` | Bearer + User menu | `{roleId}` (null = no role) | `200`; `400` for your own account |
| PUT    | `/api/users/{id}/branches` | Bearer + User menu | `{branches: ["MED", "PRM"]}` | `200`; `400` for an unknown branch |
| GET    | `/api/users/{id}/menus` | Bearer + User menu | none                    | `200 {role, allAccess, menus}`: the menu tree that user's sidebar shows |
| GET    | `/api/branches`      | Bearer + Cabang menu | none                     | `200 {branches: [...]}` (secret key masked) |
| POST   | `/api/branches`      | Bearer + Cabang menu | branch fields            | `201` branch; `400`; `409` if the code is used |
| PUT    | `/api/branches/{kode}` | Bearer + Cabang menu | branch fields (kode ignored; empty `secretKey` keeps it) | `200` branch; `404` |
| GET    | `/healthz`           | none   | none                                   | `ok` |

Validation rules: the email must be valid and is stored lowercase; the password must be 8–72 characters; `confirmPassword` must equal `password`. Validation errors come back per field:

```json
{ "error": "validation failed", "errors": { "confirmPassword": "Passwords do not match" } }
```

JWT claims: `sub` (user id, UUID), `email`, `iat`, `exp`.

### Swagger / OpenAPI

With `SWAGGER_ENABLED=true` (set in `.env.example`), the backend serves:

- Swagger UI: http://localhost:8080/swagger/
- OpenAPI 3 spec: http://localhost:8080/swagger/openapi.yaml

To try a protected endpoint, call `POST /api/auth/login`, click **Authorize** and paste the `token`.

The variable defaults to `false`, so in production the docs are hidden (both URLs return `404`) unless you turn them on. The spec is written by hand in [backend/internal/docs/openapi.yaml](backend/internal/docs/openapi.yaml) and embedded in the binary, so update it whenever an endpoint changes. A test fails if a route is missing from it. The UI page loads Swagger UI's assets from cdn.jsdelivr.net.

Example:

```bash
curl -X POST localhost:8080/api/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"password1","confirmPassword":"password1"}'

TOKEN=$(curl -s -X POST localhost:8080/api/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"password1"}' | sed -E 's/.*"token":"([^"]+)".*/\1/')

curl localhost:8080/api/me -H "Authorization: Bearer $TOKEN"
# {"email":"alice@example.com","id":"...","message":"Hello alice@example.com, welcome back"}
```

## Sidebar menu (configured in the database)

After login every page shares a layout with a sidebar. The sidebar is built from the `menus` table, so you change it with SQL (or DBeaver) and users see the change after reloading the page. No restart or redeploy is needed.

| Column       | Meaning |
|--------------|---------|
| `parent_id`  | `NULL` for a top-level item, otherwise the `id` of its group. Any depth works |
| `title`      | Text shown in the menu |
| `icon`       | Icon name: `home`, `database`, `circle`, `users`, `user`, `settings`, `heart`, `chart`, `bar-chart`, `monitor`, `boxes`, `upload`, `barcode`, `dollar`, `file`, `user-circle`, `copy`, `building`. Unknown names show a dot. Add names in [frontend/src/layout/icons.tsx](frontend/src/layout/icons.tsx) |
| `path`       | Frontend route the item opens. Leave `NULL` on a group: clicking a group opens or closes its children |
| `sort_order` | Lower comes first among siblings |
| `is_active`  | `false` hides the item and everything under it |

```sql
-- Add a child under "Setting" (it becomes a group)
INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT id, 'User Management', 'users', '/setting/users', 10 FROM menus WHERE title = 'Setting' AND parent_id IS NULL;
UPDATE menus SET path = NULL WHERE title = 'Setting' AND parent_id IS NULL;

-- Hide an item
UPDATE menus SET is_active = false WHERE title = 'Upload';
```

The first menu is seeded by `002_create_menus.sql` only once, so later edits and deletes are kept across restarts. A route without its own React page shows a placeholder with the menu title. To build a real page, add a `<Route>` for its path inside the layout route in [frontend/src/main.tsx](frontend/src/main.tsx), wrapped in `<MenuGuard>` so only roles that have the menu can open it.

## Roles and menu access

Each user has one role, and the role decides which menus they see. Admins manage this in the app, with no SQL needed:

- **Management › Role**: create, rename or delete roles, and tick the menus each role can open. Ticking a group grants every item in it, including items added later. **All menus** grants everything, including menus created later.
- **Management › User**: pick a role for each user, and with the building icon the branches (cabang) they work at. A user can belong to several branches. The **Action** icon shows, as a tree, exactly which menus that user gets from their role. A user without a role (for example someone who just registered) sees an empty sidebar until an admin assigns one.

Changes apply on the user's next page load, without a restart or a new login, because access is read from the database on every request.

| Table / column       | Meaning |
|----------------------|---------|
| `roles`              | `name` (unique) and `all_access` (every menu, including future ones) |
| `role_menus`         | `(role_id, menu_id)` pairs. A group id grants everything under it, and the groups above a granted item are shown so it can be reached |
| `users.role_id`      | The user's role, or `NULL` for no menus. A role that still has users cannot be deleted |
| `refbranchauth`      | User-to-branch mapping, one row per pair: `branch` (refbranch.kode) and `userid` (users.id). Same columns as `refbranchauth` in `lab`, plus a unique `(branch, userid)` and a foreign key to `refbranch`. Branches do not grant page access; they are there for pages that filter data by branch |

Access is also enforced on the server. The role and user APIs check that the caller's role can open `/management/role` or `/management/user`. The rule is "whoever has the menu can use its page", so giving a role the Role menu effectively makes it an admin role. To stop admins from locking themselves out, you cannot remove the Role page from your own role or change your own role.

Seeded by `017_roles.sql`:

| Role          | Menus |
|---------------|-------|
| `Admin`       | All menus |
| `Pendaftaran` | Home, Master Data Klinik, Front Office Klinik, Report Klinik |
| `Perawat`     | Home, Transaksi Klinik, Report Klinik |

### Demo users

With `SEED_DEMO_USERS=true` (set in `.env.example`), the backend creates these users on startup if they do not exist yet. It never overwrites an existing user.

| Email                | Password    | Role          |
|----------------------|-------------|---------------|
| `admin@medika.com`   | `Medika123` | `Admin`       |
| `cs@medika.com`      | `Medika123` | `Pendaftaran` |
| `perawat@medika.com` | `Medika123` | `Perawat`     |

The password is public, so keep `SEED_DEMO_USERS=false` in production. If the demo users were already created somewhere they should not exist, delete them.

## Management › Cabang (branches)

`/management/cabang` lists clinic branches with their Satu Sehat settings. **+ Add** and the edit icon open a form. Use Status `Nonaktif` to retire a branch; there is no delete.

The data lives in the `refbranch` table, which has the same columns, types and primary key (`pk_refbranch` on `kode`) as `refbranch` in the existing `lab` database. Migration `018_branches.sql` creates the table and `019_refbranch.sql` reshapes it.

- `kode` is the primary key, so it is set on create and cannot be changed afterwards (`PUT /api/branches/{kode}`).
- `isactive` and `isinternal` are `1`/`0` in the database and booleans in the API. Empty optional fields are stored as `NULL`, like the lab data.
- Field limits in the form and API match the `varchar` sizes (for example `nama` 50, `kode_kwt` 10).
- The table shows the columns of the original screen. Kode KWT, Alamat, Telp, NPWP, Nama NPWP and Internal start hidden; turn them on under **Column visibility**.
- **Secret Key is write-only.** The API never returns it, only whether it is set and its last 4 characters. When editing, leave it blank to keep the current key. The key is stored as plain text in the database because the server needs it to call Satu Sehat, so limit who can read the database.
- The 11 seeded branches match lab for the non-secret columns. Alamat, Telp, NPWP and the keys are not seeded; enter them in the form.

The table is the reusable [DataTable](frontend/src/components/DataTable.tsx) component, ready for other list pages. It provides:

- records per page, search, column sorting, column visibility and paging;
- **Copy** (tab-separated, for pasting into a spreadsheet), **Excel** (a real `.xlsx`), **CSV**, **PDF** and **Print**.

Exports contain every row that matches the search, in the current sort order, with the visible columns only. The PDF library (`jspdf`) loads only when PDF is clicked.

## How the 15-minute inactivity logout works

The token lifetime (`JWT_TTL`) is also the inactivity window, and the rule is enforced on both the server and the client:

1. **Server:** every token expires `JWT_TTL` after it is issued. An expired token gets `401` on protected routes.
2. **Client:** the app records the last user activity (mouse, keyboard, scroll, touch). While you are active it calls `/api/auth/refresh` (at most every ~30 s), so the token's expiry stays about 15 minutes after your latest activity.
3. If there is no activity for `JWT_TTL`, the client logs you out and shows a message. By then the token has also expired on the server, so it cannot be reused.

Activity and logout are shared across browser tabs through `localStorage`. To test quickly, set `JWT_TTL=1m` in `.env`, run `docker compose up --build`, log in, and leave the page alone for a minute.

## Local development without Docker

```bash
docker compose up -d db                                # just the database

cd backend
export DATABASE_URL=postgres://app:app@localhost:5432/authdb?sslmode=disable
export JWT_SECRET=$(openssl rand -hex 32)
go run ./cmd/server

cd frontend
npm install
npm run dev                                            # http://localhost:5173, proxies /api to :8080
```

## Tests

```bash
cd backend && go test ./...
# or without a local Go install:
docker run --rm -v "$PWD/backend:/src" -w /src golang:1.26-alpine go test ./...
```

The tests cover JWT generation and parsing (expired tokens, wrong secret, `alg=none`), register and login validation, and the full HTTP flow (register, duplicate email, wrong password, login, `/api/me`, refresh, missing or expired token) against an in-memory repository. They also cover menu filtering by role and the role and user APIs (403 without the menu, validation, duplicate names, deleting a role in use, and the self-lockout guards).

## Project layout

```
backend/
  cmd/server/main.go          wiring, HTTP server, graceful shutdown
  internal/config             environment variables
  internal/db                 connection with retry and embedded SQL migrations (each runs once)
  internal/user               users repository (PostgreSQL)
  internal/auth               JWT, validation, handlers, middleware and tests
  internal/menu               menus repository, tree building, role filtering, GET /api/menus, RequireMenu
  internal/role               roles CRUD and user role assignment (/api/roles, /api/users)
  internal/branch             clinic branches in refbranch (/api/branches)
  internal/httpx              shared JSON response helpers
  internal/docs               OpenAPI spec and Swagger UI (SWAGGER_ENABLED)
frontend/
  src/api.ts                  fetch wrapper
  src/auth/AuthContext.tsx    session, inactivity timer, token refresh
  src/layout/                 app shell: sidebar from /api/menus, top bar, icons
  src/components/             DataTable (paging, search, sort, column visibility) and its exporters
  src/pages/                  Register, Login, Dashboard, RolePage, UserPage, BranchPage, MenuPage (placeholder)
  nginx.conf                  serves the SPA and proxies /api to the backend
docker-compose.yml
.env.example
```

---

<details>
<summary>Original assignment</summary>

### Take-home Assignment: Auth with JWT

Build a small application in **TypeScript/Go/C#** that supports user **registration** and **login** using **JWT**.

- **Register**: fields `email`, `password`, `confirmPassword`
- **Login**: input `email`, `password`; returns a **JWT** containing at least `email` and a user id
- **Authenticated view / endpoint**: shows `Hello [email], welcome back`
- The user should be logged out after 15 minutes of inactivity

Deliver a runnable project and a README explaining how to build and run it (Docker Compose) and the required environment variables.

Acceptance criteria: registration works with validation; login returns a usable JWT; a protected route or screen shows the welcome message using JWT auth.

Optional bonus: Docker, a Go backend, a React/Vue frontend, tests.

</details>
