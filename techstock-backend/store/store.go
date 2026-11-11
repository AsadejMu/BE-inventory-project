package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ห้อง store ไม่รู้จัก HTTP เลย รู้แค่ฐานข้อมูล แล้วแปลรหัสของ Postgres เป็นความผิดพลาดที่มีชื่อ
var (
	ErrNotFound   = errors.New("not found")
	ErrEmailTaken = errors.New("email taken")
	ErrDuplicate  = errors.New("duplicate")
	ErrBadRef     = errors.New("referenced row not found")
	ErrInUse      = errors.New("still in use")
	ErrNotPending = errors.New("request already decided")
)

// InsufficientStockError บอกว่าสินค้าชิ้นไหนไม่พอ
type InsufficientStockError struct{ Product string }

func (e *InsufficientStockError) Error() string { return "insufficient stock: " + e.Product }

type Store struct {
	Pool *pgxpool.Pool
}

type scanner interface{ Scan(dest ...any) error }

// pgCode คืนรหัสความผิดพลาดของ PostgreSQL เช่น 23505 (ซ้ำ) 23503 (คีย์นอก)
func pgCode(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}
