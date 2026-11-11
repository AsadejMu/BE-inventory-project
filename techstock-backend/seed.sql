-- serial_seq = 1 เพราะสินค้าตัวอย่างใช้เลข 0001 ไปแล้ว สินค้าใหม่จะเริ่มที่ 0002
INSERT INTO categories (name, serial_seq) VALUES ('CPU', 1), ('GPU', 1), ('RAM', 1), ('Storage', 1);

INSERT INTO products (name, category_id, description, serial_number, price, stock) VALUES
  ('Intel Core i5-14400F',    1, '10 Cores 16 Threads',        'CPU-0001',      7290,  8),
  ('NVIDIA RTX 4060',         2, 'NVIDIA Graphics Card 8GB',   'GPU-0001',     10900,  5),
  ('Kingston Fury DDR5 16GB', 3, 'DDR5 5600MHz',               'RAM-0001',      1590, 12),
  ('Samsung 990 EVO 1TB',     4, 'NVMe SSD',                   'STORAGE-0001',  2890,  2);
