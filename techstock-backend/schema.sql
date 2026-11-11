-- TechStock schema (PostgreSQL / Supabase) : 7 ตาราง
-- รันซ้ำได้ แต่จะลบข้อมูลเดิมทั้งหมด
BEGIN;

DROP TABLE IF EXISTS withdrawal_serials, withdrawal_items, withdrawal_requests, stock_movements,
                     products, suppliers, categories, users CASCADE;

CREATE TABLE users (
  id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name          TEXT NOT NULL,
  email         TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  role          TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'admin')),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_key ON users (lower(email));

CREATE TABLE categories (
  id   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  serial_seq INTEGER NOT NULL DEFAULT 0 -- ตัวนับสำหรับสร้าง Serial อัตโนมัติ
);

CREATE TABLE suppliers (
  id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name  TEXT NOT NULL,
  phone TEXT NOT NULL DEFAULT ''
);

CREATE TABLE products (
  id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name        TEXT NOT NULL,
  category_id BIGINT NOT NULL REFERENCES categories (id),
  supplier_id BIGINT REFERENCES suppliers (id),
  description TEXT NOT NULL DEFAULT '',
  serial_number TEXT NOT NULL DEFAULT '',
  price       NUMERIC(12, 2) NOT NULL CHECK (price > 0),
  stock       INTEGER NOT NULL DEFAULT 0 CHECK (stock >= 0),
  image_url   TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE stock_movements (
  id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  product_id BIGINT NOT NULL REFERENCES products (id),
  type       TEXT NOT NULL CHECK (type IN ('in', 'out')),
  quantity   INTEGER NOT NULL CHECK (quantity > 0),
  note       TEXT NOT NULL DEFAULT '',
  created_by BIGINT REFERENCES users (id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE withdrawal_requests (
  id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id    BIGINT NOT NULL REFERENCES users (id),
  status     TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
  note       TEXT NOT NULL DEFAULT '',
  decided_by BIGINT REFERENCES users (id),
  decided_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE withdrawal_items (
  id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  request_id BIGINT NOT NULL REFERENCES withdrawal_requests (id) ON DELETE CASCADE,
  product_id BIGINT NOT NULL REFERENCES products (id),
  quantity   INTEGER NOT NULL CHECK (quantity > 0)
);

-- 8) withdrawal_serials : Serial ของแต่ละชิ้นที่ออกให้ตอนอนุมัติคำขอเบิก
CREATE TABLE withdrawal_serials (
  id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  item_id       BIGINT NOT NULL REFERENCES withdrawal_items (id) ON DELETE CASCADE,
  serial_number TEXT NOT NULL
);
CREATE UNIQUE INDEX withdrawal_serials_key ON withdrawal_serials (lower(serial_number));
CREATE INDEX withdrawal_serials_item_idx ON withdrawal_serials (item_id);

CREATE INDEX products_category_idx ON products (category_id);
-- Serial Number ห้ามซ้ำ (ไม่นับค่าว่าง = ไม่ระบุ)
CREATE UNIQUE INDEX products_serial_key ON products (lower(serial_number)) WHERE serial_number <> '';
CREATE INDEX stock_movements_product_idx ON stock_movements (product_id);
CREATE INDEX withdrawal_requests_user_idx ON withdrawal_requests (user_id);
CREATE INDEX withdrawal_items_request_idx ON withdrawal_items (request_id);

-- ปิดช่อง Data API ของ Supabase: เปิด RLS แต่ไม่สร้าง policy (Go ใช้ role postgres ซึ่งข้าม RLS)
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE suppliers ENABLE ROW LEVEL SECURITY;
ALTER TABLE products ENABLE ROW LEVEL SECURITY;
ALTER TABLE stock_movements ENABLE ROW LEVEL SECURITY;
ALTER TABLE withdrawal_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE withdrawal_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE withdrawal_serials ENABLE ROW LEVEL SECURITY;

COMMIT;
