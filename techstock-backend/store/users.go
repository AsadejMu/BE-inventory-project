package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type User struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	PasswordHash string `json:"-"` // ไม่มีวันออกไปกับ JSON
}

func (s *Store) CreateUser(ctx context.Context, name, email, hash, role string) (User, error) {
	u := User{Name: name, Email: email, Role: role}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO users (name, email, password_hash, role)
		 VALUES ($1, $2, $3, $4) RETURNING id`, name, email, hash, role).Scan(&u.ID)
	if pgCode(err) == "23505" {
		return User{}, ErrEmailTaken
	}
	return u, err
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.Pool.QueryRow(ctx,
		`SELECT id, name, email, role, password_hash FROM users WHERE lower(email) = $1`, email).
		Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	var u User
	err := s.Pool.QueryRow(ctx,
		`SELECT id, name, email, role FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Name, &u.Email, &u.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Store) UpdateUserName(ctx context.Context, id int64, name string) (User, error) {
	var u User
	err := s.Pool.QueryRow(ctx,
		`UPDATE users SET name = $2 WHERE id = $1 RETURNING id, name, email, role`, id, name).
		Scan(&u.ID, &u.Name, &u.Email, &u.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// MemberRow แถวในหน้ารายชื่อสมาชิกของแอดมิน (ไม่มี password_hash)
type MemberRow struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	RequestCount int       `json:"request_count"`
}

func (s *Store) ListMembers(ctx context.Context) ([]MemberRow, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT u.id, u.name, u.email, u.role, u.created_at,
		        (SELECT count(*) FROM withdrawal_requests w WHERE w.user_id = u.id)
		 FROM users u ORDER BY u.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MemberRow{}
	for rows.Next() {
		var m MemberRow
		if err := rows.Scan(&m.ID, &m.Name, &m.Email, &m.Role, &m.CreatedAt, &m.RequestCount); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpdateMember แก้ชื่อ อีเมล และบทบาทของสมาชิก (ใช้โดยแอดมิน)
func (s *Store) UpdateMember(ctx context.Context, id int64, name, email, role string) (User, error) {
	var u User
	err := s.Pool.QueryRow(ctx,
		`UPDATE users SET name = $2, email = $3, role = $4 WHERE id = $1
		 RETURNING id, name, email, role`, id, name, email, role).
		Scan(&u.ID, &u.Name, &u.Email, &u.Role)
	if pgCode(err) == "23505" {
		return User{}, ErrEmailTaken
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// DeleteMember ลบไม่ได้ถ้ามีประวัติคำขอเบิกหรือประวัติสต็อกอ้างอิงอยู่ (foreign key กันไว้)
func (s *Store) DeleteMember(ctx context.Context, id int64) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
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
