package handler

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"techstock/auth"
	"techstock/mw"
	"techstock/store"
)

// dummyHash คือลายนิ้วมือหลอก ใช้เทียบเมื่อไม่พบอีเมล เพื่อให้เสียเวลาเท่ากับกรณีรหัสผิด
// คนนอกจะจับเวลาเดาไม่ได้ว่าอีเมลไหนมีบัญชี
var dummyHash, _ = auth.HashPassword("no-such-user-password")

type registerReq struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if !decode(w, r, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	name := strings.TrimSpace(req.Name)

	errs := map[string]string{}
	if name == "" || utf8.RuneCountInString(name) > 50 {
		errs["name"] = "กรุณากรอกชื่อ (ไม่เกิน 50 ตัวอักษร)"
	}
	if !strings.Contains(email, "@") || len(email) > 255 {
		errs["email"] = "รูปแบบอีเมลไม่ถูกต้อง"
	}
	if utf8.RuneCountInString(req.Password) < 6 || len(req.Password) > 72 {
		errs["password"] = "รหัสผ่านต้องยาว 6 ตัวอักษรขึ้นไป และไม่เกิน 72 ไบต์"
	}
	if len(errs) > 0 {
		invalid(w, errs)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		internal(w, err)
		return
	}
	// role ถูกกำหนดเป็น member ที่นี่เท่านั้น ผู้สมัครกำหนดเองไม่ได้
	u, err := h.Store.CreateUser(r.Context(), name, email, hash, "member")
	if errors.Is(err, store.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "EMAIL_TAKEN", "อีเมลนี้มีบัญชีอยู่แล้ว")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	send(w, http.StatusCreated, "สมัครสมาชิกสำเร็จ", u)
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if !decode(w, r, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	u, err := h.Store.UserByEmail(r.Context(), email)
	if errors.Is(err, store.ErrNotFound) {
		auth.CheckPassword(dummyHash, req.Password) // เสียเวลาเท่ากับกรณีรหัสผิด
		writeError(w, http.StatusUnauthorized, "BAD_CREDENTIALS", "อีเมลหรือรหัสผ่านไม่ถูกต้อง")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "BAD_CREDENTIALS", "อีเมลหรือรหัสผ่านไม่ถูกต้อง")
		return
	}
	token, err := h.Tickets.Issue(u.ID, u.Role)
	if err != nil {
		internal(w, err)
		return
	}
	send(w, http.StatusOK, "เข้าสู่ระบบสำเร็จ", map[string]any{"token": token, "user": u})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	who := mw.WhoFrom(r.Context())
	u, err := h.Store.UserByID(r.Context(), who.ID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "LOGIN_REQUIRED", "กรุณาเข้าสู่ระบบก่อน")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	send(w, http.StatusOK, "", u)
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > 50 {
		invalid(w, map[string]string{"name": "กรุณากรอกชื่อ (ไม่เกิน 50 ตัวอักษร)"})
		return
	}
	u, err := h.Store.UpdateUserName(r.Context(), mw.WhoFrom(r.Context()).ID, name)
	if err != nil {
		internal(w, err)
		return
	}
	send(w, http.StatusOK, "แก้ไขโปรไฟล์สำเร็จ", u)
}
