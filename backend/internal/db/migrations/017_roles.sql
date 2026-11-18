-- Roles decide which menus (and therefore which pages) a user can open.
--
--   roles.all_access  true: every active menu, including menus added later (no role_menus rows needed)
--   role_menus        menus granted to a role. Granting a group grants everything under it,
--                     including children added later. The groups above a granted item are
--                     shown automatically so the item can be reached in the sidebar.
--   users.role_id     one role per user; NULL means no menus at all
CREATE TABLE IF NOT EXISTS roles (
    id         SERIAL PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    all_access BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS role_menus (
    role_id INT NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    menu_id INT NOT NULL REFERENCES menus (id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, menu_id)
);

-- RESTRICT: a role that still has users cannot be deleted.
ALTER TABLE users ADD COLUMN IF NOT EXISTS role_id INT REFERENCES roles (id) ON DELETE RESTRICT;
CREATE INDEX IF NOT EXISTS users_role_id_idx ON users (role_id);

INSERT INTO roles (name, all_access) VALUES
    ('Admin', true),
    ('Pendaftaran', false),
    ('Perawat', false);

INSERT INTO role_menus (role_id, menu_id)
SELECT r.id, m.id
FROM (VALUES
    ('Pendaftaran', 'Home'),
    ('Pendaftaran', 'Master Data Klinik'),
    ('Pendaftaran', 'Front Office Klinik'),
    ('Pendaftaran', 'Report Klinik'),
    ('Perawat',     'Home'),
    ('Perawat',     'Transaksi Klinik'),
    ('Perawat',     'Report Klinik')
) AS g (role_name, menu_title)
JOIN roles r ON r.name = g.role_name
JOIN menus m ON m.title = g.menu_title AND m.parent_id IS NULL;
