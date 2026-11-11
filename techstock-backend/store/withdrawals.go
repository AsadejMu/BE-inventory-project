package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type WItem struct {
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name,omitempty"`
	Quantity    int    `json:"quantity"`
	ID          int64  `json:"-"`
	// Serial ของแต่ละชิ้นที่ได้รับ (ออกให้ตอนอนุมัติ ก่อนอนุมัติจะเป็นรายการว่าง)
	Serials []string `json:"serials"`
}

type Withdrawal struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	UserName  string    `json:"user_name,omitempty"`
	Status    string    `json:"status"`
	Note      string    `json:"note"`
	Items     []WItem   `json:"items"`
	CreatedAt time.Time `json:"created_at"`

	// log การตัดสินใจ: ใครอนุมัติ/ปฏิเสธ และเมื่อไหร่ (ว่างเมื่อยัง pending)
	DecidedBy string     `json:"decided_by_name,omitempty"`
	DecidedAt *time.Time `json:"decided_at,omitempty"`
}

func (s *Store) CreateWithdrawal(ctx context.Context, userID int64, note string, items []WItem) (Withdrawal, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Withdrawal{}, err
	}
	defer tx.Rollback(ctx)

	var id int64
	err = tx.QueryRow(ctx,
		`INSERT INTO withdrawal_requests (user_id, note) VALUES ($1, $2) RETURNING id`,
		userID, note).Scan(&id)
	if err != nil {
		return Withdrawal{}, err
	}
	for _, it := range items {
		_, err = tx.Exec(ctx,
			`INSERT INTO withdrawal_items (request_id, product_id, quantity) VALUES ($1, $2, $3)`,
			id, it.ProductID, it.Quantity)
		if pgCode(err) == "23503" {
			return Withdrawal{}, ErrBadRef // มีสินค้าที่ไม่มีอยู่จริง
		}
		if err != nil {
			return Withdrawal{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Withdrawal{}, err
	}
	return s.GetWithdrawal(ctx, id)
}

const withdrawalSelect = `
	SELECT w.id, w.user_id, u.name, w.status, w.note, w.created_at, d.name, w.decided_at
	FROM withdrawal_requests w
	JOIN users u ON u.id = w.user_id
	LEFT JOIN users d ON d.id = w.decided_by`

func scanWithdrawal(row scanner) (Withdrawal, error) {
	var w Withdrawal
	var by *string // ผู้ตัดสินใจเป็น NULL ได้ (ยัง pending)
	err := row.Scan(&w.ID, &w.UserID, &w.UserName, &w.Status, &w.Note, &w.CreatedAt, &by, &w.DecidedAt)
	if by != nil {
		w.DecidedBy = *by
	}
	w.Items = []WItem{}
	return w, err
}

// ListWithdrawals all=true เห็นทุกคำขอ (admin) ไม่งั้นเห็นเฉพาะของ userID
func (s *Store) ListWithdrawals(ctx context.Context, userID int64, all bool) ([]Withdrawal, error) {
	rows, err := s.Pool.Query(ctx,
		withdrawalSelect+` WHERE ($2 OR w.user_id = $1) ORDER BY w.id DESC`, userID, all)
	if err != nil {
		return nil, err
	}
	list := []Withdrawal{}
	for rows.Next() {
		w, err := scanWithdrawal(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		list = append(list, w)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return s.attachItems(ctx, list)
}

func (s *Store) GetWithdrawal(ctx context.Context, id int64) (Withdrawal, error) {
	w, err := scanWithdrawal(s.Pool.QueryRow(ctx, withdrawalSelect+` WHERE w.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Withdrawal{}, ErrNotFound
	}
	if err != nil {
		return Withdrawal{}, err
	}
	list, err := s.attachItems(ctx, []Withdrawal{w})
	if err != nil {
		return Withdrawal{}, err
	}
	return list[0], nil
}

// attachItems ดึงรายการสินค้าของทุกคำขอด้วย query เดียว (กัน N+1)
func (s *Store) attachItems(ctx context.Context, list []Withdrawal) ([]Withdrawal, error) {
	if len(list) == 0 {
		return list, nil
	}
	ids := make([]int64, len(list))
	pos := make(map[int64]int, len(list))
	for i := range list {
		ids[i] = list[i].ID
		pos[list[i].ID] = i
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT i.request_id, i.product_id, p.name, i.quantity,
		        COALESCE(array_agg(s.serial_number ORDER BY s.id) FILTER (WHERE s.id IS NOT NULL), '{}')
		 FROM withdrawal_items i
		 JOIN products p ON p.id = i.product_id
		 LEFT JOIN withdrawal_serials s ON s.item_id = i.id
		 WHERE i.request_id = ANY($1)
		 GROUP BY i.id, p.name ORDER BY i.id`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var rid int64
		var it WItem
		if err := rows.Scan(&rid, &it.ProductID, &it.ProductName, &it.Quantity, &it.Serials); err != nil {
			return nil, err
		}
		if it.Serials == nil {
			it.Serials = []string{}
		}
		i := pos[rid]
		list[i].Items = append(list[i].Items, it)
	}
	return list, rows.Err()
}

// ApproveWithdrawal ตรวจสต็อก ตัดสต็อก บันทึกประวัติ และเปลี่ยนสถานะ ใน transaction เดียว
// ถ้าชิ้นไหนไม่พอ ทุกอย่างถูกยกเลิกทั้งหมด (ไม่มีการตัดครึ่งๆ กลางๆ)
func (s *Store) ApproveWithdrawal(ctx context.Context, id, adminID int64) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var status string
	// FOR UPDATE ล็อกแถวคำขอ กันแอดมินสองคนกดอนุมัติใบเดียวกันพร้อมกัน
	err = tx.QueryRow(ctx,
		`SELECT status FROM withdrawal_requests WHERE id = $1 FOR UPDATE`, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "pending" {
		return ErrNotPending
	}

	// เรียงตาม product_id ให้ทุกคำขอล็อกสินค้าลำดับเดียวกัน ลดโอกาส deadlock
	rows, err := tx.Query(ctx,
		`SELECT id, product_id, quantity FROM withdrawal_items WHERE request_id = $1 ORDER BY product_id, id`, id)
	if err != nil {
		return err
	}
	var items []WItem
	for rows.Next() {
		var it WItem
		if err := rows.Scan(&it.ID, &it.ProductID, &it.Quantity); err != nil {
			rows.Close()
			return err
		}
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, it := range items {
		if _, err := moveStock(ctx, tx, adminID, it.ProductID, "out", it.Quantity, fmt.Sprintf("เบิก #%d", id)); err != nil {
			return err
		}
		// ออก Serial ให้ครบทุกชิ้นที่เบิก (ประเภท-เลขรัน) ใน transaction เดียวกัน
		var categoryID int64
		if err := tx.QueryRow(ctx, `SELECT category_id FROM products WHERE id = $1`, it.ProductID).Scan(&categoryID); err != nil {
			return err
		}
		sns, err := mintSerials(ctx, tx, categoryID, it.Quantity)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO withdrawal_serials (item_id, serial_number) SELECT $1, unnest($2::text[])`,
			it.ID, sns); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx,
		`UPDATE withdrawal_requests SET status = 'approved', decided_by = $2, decided_at = now()
		 WHERE id = $1`, id, adminID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) RejectWithdrawal(ctx context.Context, id, adminID int64) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE withdrawal_requests SET status = 'rejected', decided_by = $2, decided_at = now()
		 WHERE id = $1 AND status = 'pending'`, id, adminID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var status string
	err = s.Pool.QueryRow(ctx, `SELECT status FROM withdrawal_requests WHERE id = $1`, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return ErrNotPending
}
