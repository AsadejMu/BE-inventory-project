package handler

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"techstock/mw"
	"techstock/store"
)

// ListMembers รายชื่อสมาชิกทั้งหมด (เฉพาะ admin ผ่านประตู RequireRole ใน main.go)
func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListMembers(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	send(w, http.StatusOK, "", items)
}

type memberReq struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (h *Handler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขสมาชิกต้องเป็นตัวเลข")
		return
	}
	var req memberReq
	if !decode(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	email := strings.ToLower(strings.TrimSpace(req.Email))

	errs := map[string]string{}
	if name == "" || utf8.RuneCountInString(name) > 50 {
		errs["name"] = "กรุณากรอกชื่อ (ไม่เกิน 50 ตัวอักษร)"
	}
	if !strings.Contains(email, "@") || len(email) > 255 {
		errs["email"] = "รูปแบบอีเมลไม่ถูกต้อง"
	}
	if req.Role != "member" && req.Role != "admin" {
		errs["role"] = "บทบาทต้องเป็น member หรือ admin"
	}
	if len(errs) > 0 {
		invalid(w, errs)
		return
	}
	// กันแอดมินลดสิทธิ์ตัวเอง เพื่อให้ระบบมีแอดมินเหลืออย่างน้อยหนึ่งคนเสมอ
	if id == mw.WhoFrom(r.Context()).ID && req.Role != "admin" {
		writeError(w, http.StatusConflict, "SELF_DEMOTE", "ไม่สามารถเปลี่ยนบทบาทของบัญชีตัวเองได้")
		return
	}
	u, err := h.Store.UpdateMember(r.Context(), id, name, email, req.Role)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "MEMBER_NOT_FOUND", "ไม่พบสมาชิก")
	case errors.Is(err, store.ErrEmailTaken):
		writeError(w, http.StatusConflict, "EMAIL_TAKEN", "อีเมลนี้มีบัญชีอยู่แล้ว")
	case err != nil:
		internal(w, err)
	default:
		send(w, http.StatusOK, "แก้ไขสมาชิกสำเร็จ", u)
	}
}

func (h *Handler) DeleteMember(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขสมาชิกต้องเป็นตัวเลข")
		return
	}
	if id == mw.WhoFrom(r.Context()).ID {
		writeError(w, http.StatusConflict, "SELF_DELETE", "ไม่สามารถลบบัญชีของตัวเองได้")
		return
	}
	err := h.Store.DeleteMember(r.Context(), id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "MEMBER_NOT_FOUND", "ไม่พบสมาชิก")
	case errors.Is(err, store.ErrInUse):
		writeError(w, http.StatusConflict, "MEMBER_IN_USE", "สมาชิกนี้มีประวัติคำขอเบิก ลบไม่ได้")
	case err != nil:
		internal(w, err)
	default:
		send(w, http.StatusOK, "ลบสมาชิกสำเร็จ", nil)
	}
}
