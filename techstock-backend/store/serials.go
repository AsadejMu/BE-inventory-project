package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// SerialInfo ผลการตรวจ Serial: type = "product" (Serial ของสินค้า) หรือ "issued" (ชิ้นที่ออกให้ตอนอนุมัติ)
type SerialInfo struct {
	Type         string     `json:"type"`
	Serial       string     `json:"serial_number"`
	ProductID    int64      `json:"product_id"`
	ProductName  string     `json:"product_name"`
	CategoryName string     `json:"category_name"`
	Price        float64    `json:"price"`
	Stock        int        `json:"stock"`
	Status       string     `json:"status,omitempty"`
	RequestID    *int64     `json:"request_id,omitempty"`
	RequestedBy  string     `json:"requested_by,omitempty"`
	DecidedBy    string     `json:"decided_by_name,omitempty"`
	DecidedAt    *time.Time `json:"decided_at,omitempty"`
	RequestOwner int64      `json:"-"` // ไว้ให้ handler ตัดสินว่าใครดูรายละเอียดผู้เบิกได้
}

func (s *Store) LookupSerial(ctx context.Context, serial string) (SerialInfo, error) {
	// 1) Serial ของสินค้า
	info := SerialInfo{Type: "product"}
	err := s.Pool.QueryRow(ctx,
		`SELECT p.serial_number, p.id, p.name, c.name, p.price::float8, p.stock
		 FROM products p JOIN categories c ON c.id = p.category_id
		 WHERE lower(p.serial_number) = lower($1)`, serial).
		Scan(&info.Serial, &info.ProductID, &info.ProductName, &info.CategoryName, &info.Price, &info.Stock)
	if err == nil {
		return info, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return SerialInfo{}, err
	}

	// 2) Serial ของชิ้นที่ออกให้ตอนอนุมัติคำขอเบิก
	info = SerialInfo{Type: "issued"}
	var reqID int64
	var by *string
	err = s.Pool.QueryRow(ctx,
		`SELECT s.serial_number, p.id, p.name, c.name, p.price::float8, p.stock,
		        w.id, w.user_id, u.name, w.status, d.name, w.decided_at
		 FROM withdrawal_serials s
		 JOIN withdrawal_items i ON i.id = s.item_id
		 JOIN withdrawal_requests w ON w.id = i.request_id
		 JOIN users u ON u.id = w.user_id
		 LEFT JOIN users d ON d.id = w.decided_by
		 JOIN products p ON p.id = i.product_id
		 JOIN categories c ON c.id = p.category_id
		 WHERE lower(s.serial_number) = lower($1)`, serial).
		Scan(&info.Serial, &info.ProductID, &info.ProductName, &info.CategoryName, &info.Price, &info.Stock,
			&reqID, &info.RequestOwner, &info.RequestedBy, &info.Status, &by, &info.DecidedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SerialInfo{}, ErrNotFound
	}
	if err != nil {
		return SerialInfo{}, err
	}
	info.RequestID = &reqID
	if by != nil {
		info.DecidedBy = *by
	}
	return info, nil
}
