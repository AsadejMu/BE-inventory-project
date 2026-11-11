package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type Product struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	CategoryID   int64   `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Description  string  `json:"description"`
	SerialNumber string  `json:"serial_number"`
	Price        float64 `json:"price"`
	Stock        int     `json:"stock"`
	ImageURL     string  `json:"image_url"`
}

const productSelect = `
	SELECT p.id, p.name, p.category_id, c.name, p.description, p.serial_number,
	       p.price::float8, p.stock, p.image_url
	FROM products p JOIN categories c ON c.id = p.category_id`

func scanProduct(row scanner) (Product, error) {
	var p Product
	err := row.Scan(&p.ID, &p.Name, &p.CategoryID, &p.CategoryName,
		&p.Description, &p.SerialNumber, &p.Price, &p.Stock, &p.ImageURL)
	return p, err
}

func (s *Store) queryProducts(ctx context.Context, sql string, args ...any) ([]Product, error) {
	rows, err := s.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Product{}
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListProducts ค่าจากผู้ใช้เดินทางผ่าน $1 $2 แยกจากคำสั่งเสมอ (กัน SQL injection)
func (s *Store) ListProducts(ctx context.Context, search string, categoryID int64) ([]Product, error) {
	return s.queryProducts(ctx,
		productSelect+` WHERE (p.name ILIKE '%' || $1 || '%' OR p.serial_number ILIKE '%' || $1 || '%')
		                AND ($2::bigint = 0 OR p.category_id = $2)
		                ORDER BY p.id`, search, categoryID)
}

func (s *Store) ListLowStock(ctx context.Context, threshold int) ([]Product, error) {
	return s.queryProducts(ctx,
		productSelect+` WHERE p.stock <= $1 ORDER BY p.stock, p.id`, threshold)
}

func (s *Store) GetProduct(ctx context.Context, id int64) (Product, error) {
	p, err := scanProduct(s.Pool.QueryRow(ctx, productSelect+` WHERE p.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	return p, err
}

func (s *Store) CreateProduct(ctx context.Context, p Product) (Product, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Product{}, err
	}
	defer tx.Rollback(ctx)

	// ไม่กรอก Serial มา -> ให้ระบบสร้างให้ (ทำใน transaction เดียวกับการเพิ่มสินค้า)
	if p.SerialNumber == "" {
		sn, err := nextSerial(ctx, tx, p.CategoryID)
		if err != nil {
			return Product{}, err
		}
		p.SerialNumber = sn
	}

	var id int64
	err = tx.QueryRow(ctx,
		`INSERT INTO products (name, category_id, description, serial_number, price, stock, image_url)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		p.Name, p.CategoryID, p.Description, p.SerialNumber, p.Price, p.Stock, p.ImageURL).Scan(&id)
	if pgCode(err) == "23505" {
		return Product{}, ErrDuplicate // Serial ที่กรอกมาซ้ำ
	}
	if pgCode(err) == "23503" {
		return Product{}, ErrBadRef
	}
	if err != nil {
		return Product{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Product{}, err
	}
	return s.GetProduct(ctx, id)
}

// nextSerial สร้าง Serial 1 ตัวสำหรับสินค้าใหม่ รูปแบบ "ประเภท-เลขรัน" เช่น GPU-0004
func nextSerial(ctx context.Context, tx pgx.Tx, categoryID int64) (string, error) {
	sns, err := mintSerials(ctx, tx, categoryID, 1)
	if err != nil {
		return "", err
	}
	return sns[0], nil
}

// mintSerials ออก Serial n ตัวต่อเนื่องของหมวดหมู่ด้วยคำสั่งเดียว (ไม่ว่า n จะกี่ตัว)
// UPDATE ล็อกแถวหมวดหมู่ไว้จนจบ transaction คนที่ออก Serial หมวดเดียวกันพร้อมกันจึงต้องต่อคิว ไม่ได้เลขซ้ำ
// ถ้าชุดเลขนั้นชนกับ Serial ที่มีอยู่แล้ว (เช่นมีคนกรอกเอง) จะข้ามไปชุดถัดไป
func mintSerials(ctx context.Context, tx pgx.Tx, categoryID int64, n int) ([]string, error) {
	for attempt := 0; attempt < 50; attempt++ {
		var name string
		var last int
		err := tx.QueryRow(ctx,
			`UPDATE categories SET serial_seq = serial_seq + $2 WHERE id = $1
			 RETURNING name, serial_seq`, categoryID, n).Scan(&name, &last)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBadRef
		}
		if err != nil {
			return nil, err
		}
		prefix := serialPrefix(name)
		sns := make([]string, n)
		lower := make([]string, n)
		for i := range sns {
			sns[i] = fmt.Sprintf("%s-%04d", prefix, last-n+1+i)
			lower[i] = strings.ToLower(sns[i])
		}
		var clash bool
		err = tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM products WHERE lower(serial_number) = ANY($1))
			     OR EXISTS (SELECT 1 FROM withdrawal_serials WHERE lower(serial_number) = ANY($1))`,
			lower).Scan(&clash)
		if err != nil {
			return nil, err
		}
		if !clash {
			return sns, nil
		}
	}
	return nil, errors.New("สร้าง Serial ไม่สำเร็จ")
}

// serialPrefix เอาเฉพาะ A-Z 0-9 จากชื่อประเภท ตัวพิมพ์ใหญ่ ยาวไม่เกิน 8 ตัว (ชื่อไทยล้วนใช้ ITEM)
func serialPrefix(name string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(name) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
		if b.Len() == 8 {
			break
		}
	}
	if b.Len() == 0 {
		return "ITEM"
	}
	return b.String()
}

// UpdateProduct ไม่แตะ stock เพราะสต็อกต้องเปลี่ยนผ่าน Stock API เพื่อให้มีประวัติ
func (s *Store) UpdateProduct(ctx context.Context, p Product) (Product, error) {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE products SET name = $2, category_id = $3, description = $4,
		        price = $5, image_url = $6, serial_number = COALESCE(NULLIF($7, ''), serial_number) WHERE id = $1`,
		p.ID, p.Name, p.CategoryID, p.Description, p.Price, p.ImageURL, p.SerialNumber)
	if pgCode(err) == "23505" {
		return Product{}, ErrDuplicate // Serial Number ซ้ำ
	}
	if pgCode(err) == "23503" {
		return Product{}, ErrBadRef
	}
	if err != nil {
		return Product{}, err
	}
	if tag.RowsAffected() == 0 {
		return Product{}, ErrNotFound
	}
	return s.GetProduct(ctx, p.ID)
}

func (s *Store) DeleteProduct(ctx context.Context, id int64) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM products WHERE id = $1`, id)
	if pgCode(err) == "23503" {
		return ErrInUse // มีประวัติสต็อกหรือคำขอเบิกอ้างอิงอยู่
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- categories ----------

type Category struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func (s *Store) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, name FROM categories ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateCategory(ctx context.Context, name string) (Category, error) {
	c := Category{Name: name}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO categories (name) VALUES ($1) RETURNING id`, name).Scan(&c.ID)
	if pgCode(err) == "23505" {
		return Category{}, ErrDuplicate
	}
	return c, err
}

func (s *Store) UpdateCategory(ctx context.Context, id int64, name string) (Category, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE categories SET name = $2 WHERE id = $1`, id, name)
	if pgCode(err) == "23505" {
		return Category{}, ErrDuplicate
	}
	if err != nil {
		return Category{}, err
	}
	if tag.RowsAffected() == 0 {
		return Category{}, ErrNotFound
	}
	return Category{ID: id, Name: name}, nil
}

func (s *Store) DeleteCategory(ctx context.Context, id int64) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM categories WHERE id = $1`, id)
	if pgCode(err) == "23503" {
		return ErrInUse
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
