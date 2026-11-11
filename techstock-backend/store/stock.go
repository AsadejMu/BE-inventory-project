package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type StockResult struct {
	ProductID     int64 `json:"product_id"`
	Quantity      int   `json:"quantity"`
	PreviousStock int   `json:"previous_stock"`
	CurrentStock  int   `json:"current_stock"`
}

type Movement struct {
	ID          int64     `json:"id"`
	ProductID   int64     `json:"product_id"`
	ProductName string    `json:"product_name"`
	Type        string    `json:"type"`
	Quantity    int       `json:"quantity"`
	Note        string    `json:"note"`
	CreatedAt   time.Time `json:"created_at"`
}

// MoveStock รับเข้า ("in") หรือเบิกออก ("out") พร้อมบันทึกประวัติ ใน transaction เดียว
func (s *Store) MoveStock(ctx context.Context, userID, productID int64, kind string, qty int, note string) (StockResult, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return StockResult{}, err
	}
	defer tx.Rollback(ctx) // ไม่มีผลถ้า Commit สำเร็จแล้ว

	res, err := moveStock(ctx, tx, userID, productID, kind, qty, note)
	if err != nil {
		return StockResult{}, err
	}
	return res, tx.Commit(ctx)
}

// moveStock ใช้ร่วมกับการอนุมัติคำขอเบิก (ต้องอยู่ใน tx ของมัน)
func moveStock(ctx context.Context, tx pgx.Tx, userID, productID int64, kind string, qty int, note string) (StockResult, error) {
	delta := qty
	if kind == "out" {
		delta = -qty
	}
	var current int
	// เงื่อนไข stock + delta >= 0 ทำให้สต็อกไม่มีวันติดลบ แม้มีคนเบิกพร้อมกัน
	err := tx.QueryRow(ctx,
		`UPDATE products SET stock = stock + $2
		 WHERE id = $1 AND stock + $2 >= 0 RETURNING stock`, productID, delta).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		var name string
		if e := tx.QueryRow(ctx, `SELECT name FROM products WHERE id = $1`, productID).Scan(&name); e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				return StockResult{}, ErrNotFound
			}
			return StockResult{}, e
		}
		return StockResult{}, &InsufficientStockError{Product: name}
	}
	if err != nil {
		return StockResult{}, err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO stock_movements (product_id, type, quantity, note, created_by)
		 VALUES ($1, $2, $3, $4, $5)`, productID, kind, qty, note, userID)
	if err != nil {
		return StockResult{}, err
	}
	return StockResult{ProductID: productID, Quantity: qty, PreviousStock: current - delta, CurrentStock: current}, nil
}

func (s *Store) StockHistory(ctx context.Context) ([]Movement, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT m.id, m.product_id, p.name, m.type, m.quantity, m.note, m.created_at
		 FROM stock_movements m JOIN products p ON p.id = m.product_id
		 ORDER BY m.id DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Movement{}
	for rows.Next() {
		var m Movement
		if err := rows.Scan(&m.ID, &m.ProductID, &m.ProductName, &m.Type, &m.Quantity, &m.Note, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
