package store

import "context"

// LowStockThreshold สินค้าที่เหลือไม่เกินค่านี้ (แต่ยังไม่หมด) นับเป็น "ใกล้หมด"
const LowStockThreshold = 3

type Dashboard struct {
	TotalProducts      int `json:"total_products"`
	TotalCategories    int `json:"total_categories"`
	LowStockProducts   int `json:"low_stock_products"`
	OutOfStockProducts int `json:"out_of_stock_products"`
	PendingWithdrawals int `json:"pending_withdrawals"`
}

// GetDashboard ตัวเลขทุกตัวคำนวณจากฐานข้อมูลจริง ไม่ใช่ค่าที่ FE กำหนดเอง
func (s *Store) GetDashboard(ctx context.Context) (Dashboard, error) {
	var d Dashboard
	err := s.Pool.QueryRow(ctx,
		`SELECT
		   (SELECT count(*) FROM products),
		   (SELECT count(*) FROM categories),
		   (SELECT count(*) FROM products WHERE stock > 0 AND stock <= $1),
		   (SELECT count(*) FROM products WHERE stock = 0),
		   (SELECT count(*) FROM withdrawal_requests WHERE status = 'pending')`,
		LowStockThreshold).
		Scan(&d.TotalProducts, &d.TotalCategories, &d.LowStockProducts,
			&d.OutOfStockProducts, &d.PendingWithdrawals)
	return d, err
}
