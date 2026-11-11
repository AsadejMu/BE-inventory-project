package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"techstock/auth"
	"techstock/store"
)

type Handler struct {
	Store   *store.Store
	Tickets *auth.Issuer
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// send ตอบสำเร็จตาม API Contract: { "message": "...", "data": ... }
func send(w http.ResponseWriter, status int, message string, data any) {
	body := map[string]any{"data": data}
	if message != "" {
		body["message"] = message
	}
	writeJSON(w, status, body)
}

// writeError ตอบผิดพลาดตาม API Contract: { "message": "...", "code": "..." }
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"message": message, "code": code})
}

// invalid ตอบ 400 พร้อมบอกว่าช่องไหนผิด: { "message", "errors": { "price": "..." } }
func invalid(w http.ResponseWriter, errs map[string]string) {
	writeJSON(w, http.StatusBadRequest, map[string]any{
		"message": "ข้อมูลไม่ถูกต้อง",
		"code":    "VALIDATION",
		"errors":  errs,
	})
}

// internal จดรายละเอียดไว้ใน log ฝั่งเรา แต่บอกผู้ใช้แค่ว่าระบบขัดข้อง
func internal(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "INTERNAL", "ระบบขัดข้อง ลองใหม่อีกครั้ง")
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}

// decode อ่านกล่อง JSON (จำกัดขนาด 1MB) ถ้าอ่านไม่ได้ตอบ 400 ให้เลย
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", "อ่านกล่อง JSON ไม่ออก")
		return false
	}
	return true
}
