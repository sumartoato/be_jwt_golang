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

## Master Data Klinik › Obat (drugs and stock per branch)

`/master-data-klinik/obat` lists the drug master and each drug's stock and prices **per branch**.

| Table          | Content |
|----------------|---------|
| `refobat`      | Drug master: name, jenis, units, default prices, distributor, principal, komposisi, … |
| `trxstock`     | One row per drug and branch (`id_obat`, `cabang`): `stok`, `stok_min`, `hbeli`, `hp`, `hjual`, `isactive` |
| lookups        | `refjenisobat`, `refsatuan`, `reflogoobat`, `refprodusen`, `refkodemovingobat`, `refkodemovingpo`, `refdistributorobat`, `refprincipalobat`, `refgolonganobat` |

These tables have the same columns, types and primary keys as in the `lab` database (migration `021_obat.sql`). The app adds foreign keys where the lab data is consistent. `refobat.id_prod` has none because most values point to producers that no longer exist.

**Branch authorization.** Users see and edit stock only for the branches they are mapped to under Management › User (`refbranchauth`):

- `GET /api/obat` returns every drug, but `stocks` only for the caller's branches.
- `PUT /api/obat/{id}/stock/{cabang}` returns `403` for any other branch.
- Several branches: the page shows them all, one line per branch, and the **Cabang** dropdown narrows to one. One branch: plain values. No branch: stock columns are empty and the **$** button is disabled.
- The mapping is read on every request, so changes apply immediately.

Who can open the page at all is decided by the role (the Obat menu item).

**Editing.**

- The pencil edits the drug master; `create_user`/`update_user` get the user's email cut to 20 characters (the column size).
- The **$** button edits stock and prices for one of the user's branches. Saving creates the `trxstock` row if the drug has none at that branch yet.
- Upload (import from a file) is not built yet.

### Importing drugs from lab

The drug data is not in the migrations. Copy it from the lab database (with the stack running):

```bash
LAB_PASSWORD=... scripts/import-lab-obat.sh            # only when the app has no drugs yet
LAB_PASSWORD=... scripts/import-lab-obat.sh --replace  # wipe the app's drug tables and copy again
```

`LAB_HOST` (default `host.docker.internal`, the Docker host), `LAB_PORT`, `LAB_DB` and `LAB_USER` can be overridden. Stock rows for drugs deleted from lab (about 4,200) are skipped. The import takes about 10 seconds for 2,485 drugs and 24,570 stock rows. Re-run it after `docker compose down -v`, which deletes the database.

## Master Data Klinik › Tindakan (procedures and prices per branch)

`/master-data-klinik/tindakan` works like the Obat page.

| Table               | Content |
|---------------------|---------|
| `reftindakan`       | Procedure: name, jenis, default prices (`hbeli`, `hjual`, `hjual_jamkes`), `isactive`, `isedit`, `sts_ctk_krtu` |
| `reftindakandetail` | One row per procedure and branch: `hbeli`, `hjual`, `hjual_jamkes` (price for health-insurance patients) and `isactive` |
| `refjenistindakan`  | Procedure types |

The tables have the same layout as in `lab` (migration `022_tindakan.sql`). The app adds:

- a unique `(id_tindakan, cabang)`, so there is one price row per procedure and branch;
- foreign keys to `reftindakan` and `refbranch`.

There is no foreign key on `id_jenis`, because one procedure in lab points to a type that no longer exists.

Prices follow the same branch authorization as Obat:

- `GET /api/tindakan` returns `prices` only for the caller's branches.
- `PUT /api/tindakan/{id}/price/{cabang}` returns `403` for any other branch.
- The pencil edits the procedure itself; the **$** button edits the prices at one of the user's branches.

Import the data from lab with:

```bash
LAB_PASSWORD=... scripts/import-lab-tindakan.sh            # only when the app has no procedures yet
LAB_PASSWORD=... scripts/import-lab-tindakan.sh --replace  # wipe and copy again
```

The three duplicate price rows in lab (same procedure and branch, same values) are imported once. Shared import helpers live in `scripts/lib/lab.sh`.

## Master Data Klinik › Penunjang (supporting examinations and prices per branch)

`/master-data-klinik/penunjang` works like Tindakan, for lab, radiology and other supporting examinations.

| Table                | Content |
|----------------------|---------|
| `refpenunjang`       | Examination: name, type, default `hjual` and `hjual_jamkes`, `isactive` |
| `refpenunjangdetail` | One row per examination and branch: `hjual`, `hjual_jamkes`, `isactive` |
| `refjenispanjang`    | Examination types (LABORATORIUM, RADIOLOGI, LAB MOHC, …). The table is named like this in `lab`, and the name is kept |

- **No purchase price:** these tables in `lab` have no `hbeli` column, so there is none here either.
- **Same layout as `lab`** (migration `023_penunjang.sql`), plus a unique `(id_penunjang, cabang)` and foreign keys to the type, examination and branch. The lab data satisfies all of them.
- **Branch authorization** is the same as Obat and Tindakan (`PUT /api/penunjang/{id}/price/{cabang}` returns `403` outside the caller's branches).

```bash
LAB_PASSWORD=... scripts/import-lab-penunjang.sh            # only when the app has no examinations yet
LAB_PASSWORD=... scripts/import-lab-penunjang.sh --replace  # wipe and copy again
```

## Master Data Klinik › Pasien (patients, paged on the server)

`/master-data-klinik/pasien` lists the ~430,000 patients of `refmpasien`. The table runs in server-side mode (`DataTable`'s `remote` option): paging, search (name, No RM, NIK, address, or Kode) and sorting happen in PostgreSQL, so each page loads in well under a second. Exports fetch at most the first 5,000 matching rows.

| Table | Content |
|-------|---------|
| `refmpasien` | Patients |
| `refgender`, `refgoldarah`, `refagama`, `refstatuskawin` | Gender, blood type, religion, marital status |
| `refjaminankesehatan` | Health insurers |
| `refkelurahan` › `refkecamatan` › `refkabupaten` › `refprovinsi` | Region hierarchy. Patients point to `refkelurahan`; the form fills kecamatan, kab/kota and provinsi when a kelurahan is picked |

The tables have the same columns as in `lab`. The app adds the following:

- **Migration `024_pasien.sql`**: search and sort indexes, including trigram (`pg_trgm`) indexes.
- **Migration `025_pasien_keys.sql`**:
  - primary keys on the lookups that lack one in `lab` (gender, agama, status kawin, kelurahan);
  - foreign keys for every reference, which the lab data satisfies.

**Branch authorization.**

- Users see, open and edit only patients whose `cabang` is one of their branches (`refbranchauth`). A patient of another branch answers `404`.
- New patients and moved patients must go to one of the user's branches.
- The 19 lab patients without a branch are not listed for anyone.

**No RM** is generated on create and never changes:

- It is the branch's newest number + 1 in the `NN-NN-NN` format (e.g. `89-01-17` → `89-01-18`), skipping numbers already used in that branch.
- Generation is serialized per branch with an advisory lock.
- No RM is not unique in `lab`; numbers repeat across branches, and 96 even repeat within a branch. So it has no unique constraint.

**Print card** opens a printable card with the name, a Code 128 barcode of the No RM ([barcode.ts](frontend/src/components/barcode.ts)) and the No RM.

**Data notes (lab):**

- 175 birth dates are impossible (years 1000 to 9999). They show no age, sort last, and must be corrected before the patient can be saved.
- Empty branch codes were imported as `NULL`.

### Importing patients from lab

```bash
LAB_PASSWORD=... scripts/import-lab-pasien.sh            # only when the app has no patients yet
LAB_PASSWORD=... scripts/import-lab-pasien.sh --replace  # wipe and copy again (about 40 seconds)
```

This copies real personal and medical data (names, NIK, addresses, allergies). Keep it only on machines allowed to hold it, never commit a dump, and consider anonymizing it for shared development databases.

## Master Data Klinik › Dokter (doctors, their procedures and practice branches)

`/master-data-klinik/dokter` has three tabs:

- **Data Dokter**: the 689 doctors of `refmdokter`, with type, gender, status (`status` 1 = Active, 2 = Non Active) and practice branches. **+ Add** and the pencil open the form.
- **Tindakan Dokter**: the procedures a doctor may perform (`trxtindakan` → `reftindakan`). The left list holds the doctor's procedures; remove them with the trash icon, or tick several and press **Delete**. The right list holds the other procedures; tick them and press **Move**.
- **Cabang Praktek**: the branches where the doctor practises (`refmdokterauth`), with the same two lists. Users can only add or remove **their own branches** (`refbranchauth`); other branches show a lock, and the API answers `403` for them.

Open the second and third tab for a doctor with the row buttons in Data Dokter.

| Table | Content |
|-------|---------|
| `refmdokter` | Doctors. Legacy fields `id_internal`, `cabang`, `oldid` and `ihs_*` are kept but not edited |
| `refjenisdokter` | Doctor types (DOKTER UMUM, DOKTER GIGI, …) |
| `refmdokterauth` | Doctor ↔ practice branch |
| `trxtindakan` | Doctor ↔ procedure |

The tables have the same layout as in `lab`. Migration `026_dokter.sql` creates them; `027_dokter_keys.sql` adds:

- foreign keys to the lookups (shared with the patient tables) and to `reftindakan` and `refbranch`;
- unique `(dokterid, cabang)` and `(id_dokter, id_tindakan)`.

Lab has 1,512 duplicate doctor/procedure pairs; the import loads each pair once.

```bash
LAB_PASSWORD=... scripts/import-lab-dokter.sh            # needs the Pasien and Tindakan imports first
LAB_PASSWORD=... scripts/import-lab-dokter.sh --replace
```

## Master Data Klinik › Jaminan Kesehatan (health insurers)

`/master-data-klinik/jaminan-kesehatan` lists the insurers of `refjaminankesehatan`. The columns are type, status (`isactive` 1 = Active, 2 = Non Active) and limit (`status_limit` 1 = Limit with the `limit` amount, 2 = No Limit). Each row has three actions:

- **Pencil**: edit the insurer.
- **Ketentuan PKS**: the agreement terms of `refjaminankesehatan_detail`, one text per term of `refjaminankesehatan_pksmaster`: Masa berlaku PKS, Tarif, Biaya Adm, Biaya Konsul, Batas Tagihan Kadaluarsa and Batas Pembayaran kadaluarsa. Leave a term empty to remove it.
- **Detail item (i)**: the insurer's numbered list in `refjaminankesehatan_detail_item` (in lab, exclusions such as "Penyakit akibat alkohol dan narkoba"). Items can be added, reordered and removed; they are renumbered 1, 2, 3, … on save.

Migration `028_jamkes.sql`:

- creates the type table (`refjaminankesehatantype`: PT, CV, Non PT CV, Payor Admedika, Non Payor Admedika, PT & CV), the PKS term list and the two detail tables, with the same columns as `lab`;
- seeds the 6 types and 6 terms;
- adds a primary key on the term list, foreign keys, and a unique `(jamkesid, detailid)` for the terms.

The insurers come with the patient import. The details have their own import:

```bash
LAB_PASSWORD=... scripts/import-lab-jamkes.sh            # PKS terms and item lists
LAB_PASSWORD=... scripts/import-lab-jamkes.sh --replace
```

Re-running `import-lab-pasien.sh --replace` empties the insurer details (they reference the insurers), so run `import-lab-jamkes.sh` again afterwards.

## Master Data Klinik › Distributor

`/master-data-klinik/distributor` lists the drug distributors of `refdistributorobat`, with these columns: Kode, Nama, Alamat, Telp, Fax, Kontak Person (`cp`) and Telp KP (`telp_cp`). **+ Add** and the pencil open the form. Field limits follow the `varchar` sizes (Nama 300, Alamat 200, Telp 15, Fax 25, Kontak Person 40, Telp KP 25).

The table and its data come with the Obat migration and import (`021_obat.sql`, `scripts/import-lab-obat.sh`), so this page needs no migration or import of its own. There is no delete, because every drug refers to its distributor.

## Master Data Klinik › Principal

`/master-data-klinik/principal` lists the drug principals of `refprincipalobat` (Kode, Nama). **+ Add** and the pencil open a one-field form (Nama, at most 300 characters).

The table and its data come with the Obat migration and import (`021_obat.sql`, `scripts/import-lab-obat.sh`), so this page needs no migration or import of its own. There is no delete, because drugs refer to their principal.

## Master Data Klinik › ICD

`/master-data-klinik/icd` lists the 18,505 ICD-X diagnosis codes of `reficd` (Kode, Kode ICD-X, Nama Penyakit, Jenis ICD-X).

- **Server-side mode.** Like Pasien, paging, search (code, disease name or type) and sorting run in the database, and exports fetch at most 5,000 matching rows.
- **Form.** **+ Add** and the pencil open the form; Kode ICD-X is stored uppercase and must be unique (`409` otherwise).

Migration `029_icd.sql` creates `refjenisicd` (GIGI, THT, SARAF, …) and `reficd` with the lab layout. It adds a unique `kode_icd`, a foreign key to the type, and trigram indexes for search; the lab data satisfies all of them.

```bash
LAB_PASSWORD=... scripts/import-lab-icd.sh            # only when the app has no ICD codes yet
LAB_PASSWORD=... scripts/import-lab-icd.sh --replace
```

## Master Data Klinik › Penjual OTC (Perujuk OTC)

`/master-data-klinik/penjual-otc` lists the people who refer over-the-counter sales (`refperujukotc`: ID, Nama), titled "Perujuk OTC" like the original screen. **+ Add** and the pencil open a one-field form (Nama, at most 50 characters). There is no delete, because sales transactions (`trxjual.id_perujuk_otc` in lab) refer to these rows. Names are not unique in lab (one appears twice), so there is no unique constraint.

```bash
LAB_PASSWORD=... scripts/import-lab-perujuk-otc.sh            # migration 030_perujuk_otc.sql creates the table
LAB_PASSWORD=... scripts/import-lab-perujuk-otc.sh --replace
```

## Front Office Klinik › Pendaftaran Pasien (patient registrations)

`/front-office-klinik/pendaftaran-pasien` lists registrations (`trxpendaftaran`, 344k rows in lab) with server-side paging, search and sorting, like Pasien. The columns are No Pendaftaran, No Antrian, No Antrian Poli, Tanggal Waktu, Pelayanan, Status Kunjungan, No RM, Nama, Tanggal Lahir, Umur, Alamat, Cabang, Pakai Kartu, Total Point and Action.

- **Filters.** A date range (both ends default to today), a branch selector, and a **Filter** popover with Pelayanan and Status. Exports fetch at most 5,000 matching rows.
- **Pelayanan** comes from `refjenisrawat` (POLI UMUM, POLI GIGI, LABORATORIUM, OBAT BEBAS, …). `refjenispanjang` holds the supporting examination types used by Penunjang, not the registration services.
- **Status Kunjungan** comes from `refstatustransaksi`: NON KUN, CLOSE, DAFTAR, PROSES, PAYMENT, CANCEL. A new registration starts as DAFTAR. It can be edited (Pelayanan, Pakai Kartu) or cancelled only while it is still DAFTAR. Cancelling records who cancelled it, when, and the reason, which shows as a tooltip on the status.
- **Total Point** is the patient's point balance in `refpasienpoint`.
- **+ Add** opens a dialog with Tanggal (today or earlier), Pelayanan, Cabang and Pakai Kartu, above a patient search table where you tick one patient. Cabang is filled from the patient's branch and can be changed.
- **Queue numbers** follow lab:
  - No Antrian counts per branch per day.
  - No Antrian Poli counts per branch, day and service. It gets a printed form only for services with `neednoantrian = 1`: the service's `firstcode` plus three digits, for example `U009` or `G001`.
  - Numbering is serialized per branch and day with a PostgreSQL advisory lock. Changing the service of a registration gives it a new poli number.
- **Time zone.** Dates and times are the clinic's (`Asia/Jakarta`), whatever the server's zone is. The time zone database is embedded in the binary, because the Alpine image has none.
- **Branch authorization.** Users only see, add and edit registrations of their own branches (`refbranchauth`). Asking for another branch answers `403`, and another branch's registration answers `404`.

Migration `031_pendaftaran.sql` creates `refjenisrawat`, `refstatustransaksi`, `trxpendaftaran` and `refpasienpoint` with the lab layout. `032_pendaftaran_keys.sql` adds foreign keys to the service, the status and the branch. There is no foreign key to the patient, because 38,174 lab registrations refer to patients that no longer exist; the list shows "(pasien tidak ditemukan)" for them. The import maps 194 empty branch codes to NULL, and copies point balances only for patients that exist in the app. The lab data ends on 2026-11-08, so "today" is empty until new registrations are made.

```bash
LAB_PASSWORD=... scripts/import-lab-pendaftaran.sh            # needs the Pasien import first
LAB_PASSWORD=... scripts/import-lab-pendaftaran.sh --replace
```

The data is personal (who visited which clinic and when): keep it on machines allowed to hold it, and never commit it.

### Master Data Satu Sehat › Organisasi & Lokasi

`/master-data-satu-sehat/organisasi-lokasi` lists, per branch and poli, the SATUSEHAT Organization and Location the clinic uses (`reforganizationlocation`): Kode, Cabang, Poli (`refjenisrawat`), Organisasi, Lokasi, Organisasi ID and Lokasi ID. Like the original screen there is no add or delete: rows are fixed per branch and poli, and the pencil edits the names and the two SATUSEHAT IDs (empty, or a UUID). Users only see and edit the rows of their own branches (`refbranchauth`). `jenisrawatid` has no foreign key, because `import-lab-pendaftaran.sh` truncates `refjenisrawat` with CASCADE.

```bash
LAB_PASSWORD=... scripts/import-lab-organisasi-lokasi.sh            # migration 033_organization_location.sql creates the table
LAB_PASSWORD=... scripts/import-lab-organisasi-lokasi.sh --replace
```

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
  internal/obat               drug master and stock per branch (/api/obat)
  internal/tindakan           procedures and prices per branch (/api/tindakan)
  internal/penunjang          supporting examinations and prices per branch (/api/penunjang)
  internal/pasien             patients with server-side paging, No RM generation (/api/pasien)
  internal/dokter             doctors, their procedures and practice branches (/api/dokter)
  internal/jamkes             health insurers, PKS terms and item lists (/api/jamkes)
  internal/distributor        drug distributors (/api/distributor)
  internal/principal          drug principals (/api/principal)
  internal/icd                ICD-X codes with server-side paging (/api/icd)
  internal/perujukotc         OTC referrers, "Penjual OTC" (/api/perujuk-otc)
  internal/pendaftaran        patient registrations, queue numbers, cancel (/api/pendaftaran)
  internal/branchauth         which branches a user is mapped to (refbranchauth)
  internal/httpx              shared JSON response helpers
  internal/docs               OpenAPI spec and Swagger UI (SWAGGER_ENABLED)
frontend/
  src/api.ts                  fetch wrapper
  src/auth/AuthContext.tsx    session, inactivity timer, token refresh
  src/layout/                 app shell: sidebar from /api/menus, top bar, icons
  src/components/             DataTable (paging, search, sort, column visibility) and its exporters
  src/components/branchView   per-branch columns, branch filter (Obat, Tindakan, Penunjang)
  src/components/FormKit      modal form, fields and amount inputs for master-data dialogs
  src/components/barcode.ts   Code 128 barcode (SVG) for patient cards
  src/components/dates.ts     today, DD/MM/YYYY formatting and age
  src/pages/                  Register, Login, Dashboard, RolePage, UserPage, BranchPage, ObatPage, TindakanPage, PenunjangPage, PasienPage, DokterPage, JamkesPage, DistributorPage, PrincipalPage, IcdPage, PerujukOtcPage, PendaftaranPage, MenuPage (placeholder)
scripts/import-lab-obat.sh    copies drugs and stock from the legacy lab database
scripts/import-lab-tindakan.sh copies procedures and prices per branch from lab
scripts/import-lab-penunjang.sh copies supporting examinations and prices per branch from lab
scripts/import-lab-pasien.sh  copies patients and their lookups from lab (personal data)
scripts/import-lab-dokter.sh  copies doctors, their procedures and practice branches from lab
scripts/import-lab-jamkes.sh  copies the insurers' PKS terms and item lists from lab
scripts/import-lab-icd.sh     copies ICD-X codes and their types from lab
scripts/import-lab-perujuk-otc.sh copies the OTC referrers from lab
scripts/import-lab-pendaftaran.sh copies registrations, services, statuses and point balances from lab (personal data)
scripts/import-lab-organisasi-lokasi.sh copies the SATUSEHAT organizations & locations from lab
scripts/lib/lab.sh            shared helpers for the import scripts
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
