#!/usr/bin/env bash
# ตัวเช็คความพร้อมก่อน/หลัง deploy (อ่านอย่างเดียว ไม่สร้างหรือแก้ข้อมูลใดๆ)
#   ./check.sh                                   ตรวจก่อน deploy (ในเครื่อง รันในโฟลเดอร์ backend)
#   ./check.sh https://xxx.onrender.com          ตรวจ BE หลัง deploy
#   ./check.sh https://xxx.onrender.com https://xxx.vercel.app   ตรวจ CORS ของ FE ด้วย
#   ใส่ ADMIN_EMAIL / ADMIN_PASS นำหน้าคำสั่ง เพื่อตรวจส่วนที่ต้อง login
pass=0; fail=0; warn=0
ok()    { pass=$((pass+1)); echo "PASS  $1"; }
bad()   { fail=$((fail+1)); echo "FAIL  $1"; [ -n "$2" ] && echo "      -> $2"; }
wn()    { warn=$((warn+1)); echo "WARN  $1"; [ -n "$2" ] && echo "      -> $2"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1" "คาด $2 ได้ $3"; fi; }
psqlq() { psql "$DATABASE_URL" -tAc "$1" 2>/dev/null; }

local_checks() {
  echo "== ตรวจโค้ด =="
  if command -v go >/dev/null; then
    go build ./... >/tmp/chk_build.log 2>&1 && ok "go build" || bad "go build" "cat /tmp/chk_build.log"
    go vet ./...   >/tmp/chk_vet.log 2>&1   && ok "go vet"   || bad "go vet"   "cat /tmp/chk_vet.log"
    [ -z "$(gofmt -l .)" ] && ok "gofmt" || wn "มีไฟล์ที่ยังไม่จัดรูปแบบ" "gofmt -w ."
    go test ./auth >/tmp/chk_test.log 2>&1 && ok "go test ./auth" || bad "go test ./auth" "cat /tmp/chk_test.log"
  else
    bad "ไม่พบคำสั่ง go" "ติดตั้ง Go ก่อน"
  fi

  echo "== ตรวจ Git =="
  if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git ls-files --error-unmatch go.sum >/dev/null 2>&1 && ok "go.sum ถูก commit แล้ว" || bad "go.sum ยังไม่ถูก commit (Render จะ build ไม่ผ่าน)" "go mod tidy && git add go.sum"
    if git ls-files | grep -Eq '(^|/)(\.env|\.jwt_secret)$'; then
      bad "ไฟล์ความลับถูก commit (.env หรือ .jwt_secret)" "git rm --cached ไฟล์นั้น แล้วเปลี่ยน JWT_SECRET กับรหัสผ่านฐานข้อมูลใหม่"
    else ok "ไม่มี .env/.jwt_secret ใน Git"; fi
    [ -z "$(git status --porcelain)" ] && ok "ไม่มีไฟล์ค้างที่ยังไม่ commit" || wn "มีไฟล์ที่ยังไม่ commit" "git status"
    if git rev-parse '@{u}' >/dev/null 2>&1; then
      [ "$(git rev-list '@{u}..HEAD' --count)" = "0" ] && ok "push ครบแล้ว" || wn "ยังมี commit ที่ไม่ได้ push" "git push"
    else wn "branch นี้ยังไม่ผูกกับ remote"; fi
  else wn "ไม่ใช่ Git repo"; fi

  echo "== ตรวจตัวแปร env (ในเทอร์มินัลนี้) =="
  [ -n "$DATABASE_URL" ] && ok "DATABASE_URL ถูกตั้งแล้ว" || bad "DATABASE_URL ว่าง" "set -a; source .env; set +a"
  case "$DATABASE_URL" in
    *@db.*.supabase.co*) bad "DATABASE_URL เป็นแบบ Direct (IPv6 ต่อไม่ได้จาก Render/Codespaces)" "ใช้ Session pooler ที่ host ลงท้าย pooler.supabase.com" ;;
    *pooler.supabase.com:6543*) wn "ใช้ Transaction pooler (พอร์ต 6543)" "ต่อท้าย ?default_query_exec_mode=simple_protocol หรือใช้ Session pooler (พอร์ต 5432)" ;;
    *pooler.supabase.com*) ok "DATABASE_URL เป็น Session pooler" ;;
    *localhost*|*127.0.0.1*) wn "DATABASE_URL ชี้เครื่องตัวเอง" "บน Render ต้องใช้ค่าของ Supabase" ;;
  esac
  [ "${#JWT_SECRET}" -ge 32 ] && ok "JWT_SECRET ยาว ${#JWT_SECRET} ตัว" || bad "JWT_SECRET ว่างหรือสั้นกว่า 32 ตัว" "openssl rand -hex 32"
  [ -n "$ALLOWED_ORIGINS" ] && ok "ALLOWED_ORIGINS=$ALLOWED_ORIGINS" || wn "ALLOWED_ORIGINS ว่าง (จะใช้ค่าเริ่มต้น http://localhost:5173)"

  echo "== ตรวจฐานข้อมูล =="
  if [ -z "$DATABASE_URL" ]; then wn "ข้ามการตรวจฐานข้อมูล (ไม่มี DATABASE_URL)"
  elif ! command -v psql >/dev/null; then wn "ข้ามการตรวจฐานข้อมูล (ไม่มี psql)" "ตรวจใน Supabase SQL Editor แทน"
  else
    check "ตารางครบ 8 ตาราง" 8 "$(psqlq "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('users','categories','suppliers','products','stock_movements','withdrawal_requests','withdrawal_items','withdrawal_serials')")"
    check "คอลัมน์/ตารางของ Serial ครบ (3 รายการ)" 3 "$(psqlq "SELECT (SELECT count(*) FROM information_schema.columns WHERE table_name='products' AND column_name='serial_number') + (SELECT count(*) FROM information_schema.columns WHERE table_name='categories' AND column_name='serial_seq') + (SELECT count(*) FROM information_schema.tables WHERE table_name='withdrawal_serials')")"
    a=$(psqlq "SELECT count(*) FROM users WHERE role='admin'")
    [ "${a:-0}" -ge 1 ] 2>/dev/null && ok "มีแอดมิน $a คน" || bad "ยังไม่มีแอดมิน" "go run ./cmd/mkadmin อีเมล รหัสผ่าน ชื่อ"
    r=$(psqlq "SELECT count(*) FROM pg_tables WHERE schemaname='public' AND NOT rowsecurity")
    [ "$r" = "0" ] && ok "เปิด RLS ครบทุกตาราง" || wn "มีตารางที่ยังไม่เปิด RLS (${r:-?} ตาราง)" "ดูส่วน ENABLE ROW LEVEL SECURITY ใน schema.sql"
  fi
}

remote_checks() {
  local R=${1%/} FE=${2%/} H='Content-Type: application/json'
  code() { curl -s --max-time 30 -o /tmp/chk.out -w '%{http_code}' "$@"; }
  echo "== ตรวจ $R (ครั้งแรกอาจรอได้ถึง 1 นาทีถ้าเซิร์ฟเวอร์เพิ่งตื่น) =="
  check "GET /healthz" 200 "$(curl -s --max-time 90 -o /dev/null -w '%{http_code}' "$R/healthz")"
  check "GET /api/products" 200 "$(code "$R/api/products")"
  grep -q '"data":\[' /tmp/chk.out && ok "products ตอบเป็นรายการ (data เป็น array)" || bad "products ตอบผิดรูปแบบ" "$(head -c 150 /tmp/chk.out)"
  check "GET /api/categories" 200 "$(code "$R/api/categories")"
  check "login ผิดต้องได้ 401" 401 "$(code -X POST "$R/api/auth/login" -H "$H" -d '{"email":"nobody@example.com","password":"wrong-password"}')"
  check "members ไม่มี token ต้องได้ 401" 401 "$(code "$R/api/members")"
  check "serials ไม่มี token ต้องได้ 401" 401 "$(code "$R/api/serials?serial=X")"

  if [ -n "$FE" ]; then
    pre() { curl -si --max-time 30 -X OPTIONS "$R/api/products" -H "Origin: $1" -H 'Access-Control-Request-Method: GET' | tr -d '\r'; }
    pre "$FE" | grep -i '^access-control-allow-origin:' | grep -qiF "$FE" && ok "CORS อนุญาต $FE" \
      || bad "CORS ไม่อนุญาต $FE" "แก้ ALLOWED_ORIGINS บน Render ให้ตรงเป๊ะ (มี https:// ไม่มี / ท้าย)"
    check "CORS ปฏิเสธ origin แปลกปลอม" 0 "$(pre https://evil.example | grep -ic '^access-control-allow-origin:')"
  else wn "ข้ามการตรวจ CORS" "ใส่ลิงก์ FE เป็นอาร์กิวเมนต์ที่ 2"; fi

  if [ -n "$ADMIN_EMAIL" ] && [ -n "$ADMIN_PASS" ]; then
    tok=$(curl -s --max-time 30 -X POST "$R/api/auth/login" -H "$H" \
      -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASS\"}" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
    if [ -n "$tok" ]; then
      ok "admin login ได้"
      check "GET /api/dashboard (admin)" 200 "$(code "$R/api/dashboard" -H "Authorization: Bearer $tok")"
      check "GET /api/members (admin)" 200 "$(code "$R/api/members" -H "Authorization: Bearer $tok")"
      check "GET /api/withdrawals (admin)" 200 "$(code "$R/api/withdrawals" -H "Authorization: Bearer $tok")"
      check "ตรวจ Serial ที่ไม่มีอยู่ต้องได้ 404" 404 "$(code "$R/api/serials?serial=NOPE-0000" -H "Authorization: Bearer $tok")"
    else bad "admin login ไม่ได้" "ยังไม่ได้สร้างแอดมินในฐานข้อมูลนี้ หรืออีเมล/รหัสผ่านไม่ถูก"; fi
  else wn "ข้ามส่วนที่ต้อง login" "ADMIN_EMAIL=... ADMIN_PASS=... ./check.sh $R"; fi
}

if [ -n "$1" ]; then remote_checks "$@"; else local_checks; fi
echo; echo "ผ่าน $pass   เตือน $warn   ไม่ผ่าน $fail"
[ "$fail" -eq 0 ]
