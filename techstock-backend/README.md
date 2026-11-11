# TechStock Backend (Go + PostgreSQL/Supabase)

โครง `store / handler / mw / auth` ตามแล็บ Server Side Web Programming
ใช้ `net/http` ล้วน + `pgx` + `bcrypt` + `JWT (HS256)` ตอบตาม API Contract ของโปรเจกต์ (`{ message, data }`)

## รันในเครื่อง
    docker compose up -d                 # Postgres 16 + โหลด schema.sql / seed.sql ให้เอง
    go mod tidy                          # ดึง dependency + สร้าง go.sum
    export DATABASE_URL="postgres://postgres:postgres@localhost:5432/techstock"
    openssl rand -hex 32 > .jwt_secret   # ครั้งเดียว อย่าสุ่มใหม่ ไม่งั้นตั๋วเก่าใช้ไม่ได้
    export JWT_SECRET=$(cat .jwt_secret)
    export ALLOWED_ORIGINS=http://localhost:5173
    go run .

    go run ./cmd/mkadmin admin@techstock.com admin123 Admin   # สร้างแอดมิน
    go test ./auth -v                                         # ทดสอบตั๋วปลอม 6 แบบ
    go test -race ./...

## ต่อกับ Frontend
ใน frontend ตั้ง `.env` เป็น `VITE_API_URL=http://localhost:8080` แล้วรัน `npm run dev`

## ลองด้วย curl
    curl -s localhost:8080/api/products
    curl -s -X POST localhost:8080/api/auth/login -H 'Content-Type: application/json' \
         -d '{"email":"admin@techstock.com","password":"admin123"}'
    export TOKEN=<ค่า data.token>
    curl -s localhost:8080/api/dashboard -H "Authorization: Bearer $TOKEN"

## Supabase + Render
- Supabase: เปิด SQL Editor วาง `schema.sql` แล้ว `seed.sql`
- DATABASE_URL: ใช้ connection string แบบ **Session pooler** (Render ใช้ IPv4 ส่วน direct connection ของ Supabase เป็น IPv6)
  ถ้าใช้ Transaction pooler (พอร์ต 6543) ให้ต่อท้าย `?default_query_exec_mode=simple_protocol`
- Render: Build `go build -o app .` / Start `./app` / ตั้ง env `DATABASE_URL`, `JWT_SECRET`, `ALLOWED_ORIGINS` (URL ของ Vercel), health check `/healthz`

## สิ่งที่ควรรู้
- ตั๋วเก็บ role ไว้ในตัว ถ้าเปลี่ยน role ในฐานข้อมูล ต้องรอตั๋วเก่าหมดอายุ (8 ชม.) หรือให้ login ใหม่
- สต็อกไม่มีวันติดลบ: ตัดด้วย `UPDATE ... WHERE stock + delta >= 0` ใน transaction เดียวกับการบันทึกประวัติ
- ยังไม่มี pagination (contract ระบุเป็นตัวเลือก) และตาราง `suppliers` ยังไม่มี API

## เชื่อมกับ Frontend
ดู `docs/FE_BE_MAP.md` (หน้าไหนเรียก API ไหน) และรัน `./smoke.sh` เพื่อทดสอบ flow ที่ FE ใช้ทั้งหมด
