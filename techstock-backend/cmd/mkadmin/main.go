// สร้างบัญชี admin: go run ./cmd/mkadmin <email> <password> <name>
// (ไม่มี API ไหนทำให้ใครเป็น admin ได้ ต้องสร้างจากตรงนี้หรือแก้ฐานข้อมูลเท่านั้น)
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"techstock/auth"
	"techstock/store"
)

func main() {
	if len(os.Args) != 4 {
		log.Fatal("ใช้: go run ./cmd/mkadmin <email> <password> <name>")
	}
	email, password, name := strings.ToLower(os.Args[1]), os.Args[2], os.Args[3]
	if len(password) < 6 {
		log.Fatal("รหัสผ่านต้องยาว 6 ตัวขึ้นไป")
	}
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatal(err)
	}
	s := &store.Store{Pool: pool}
	u, err := s.CreateUser(context.Background(), name, email, hash, "admin")
	if errors.Is(err, store.ErrEmailTaken) {
		log.Fatal("อีเมลนี้มีบัญชีอยู่แล้ว")
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("สร้างแอดมินแล้ว id=%d email=%s\n", u.ID, u.Email)
}
