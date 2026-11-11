package mw

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"techstock/auth"
)

type Who struct {
	ID   int64
	Role string
}

type ctxKey struct{}

// WhoFrom อ่านว่าใครส่งคำขอนี้ (มาจากตั๋วที่ตรวจแล้วเท่านั้น ไม่ใช่จากเนื้อคำขอ)
func WhoFrom(ctx context.Context) Who {
	w, _ := ctx.Value(ctxKey{}).(Who)
	return w
}

func deny(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"message": msg, "code": code})
}

// RequireLogin ด่านตรวจตั๋ว ไม่มีตั๋วหรือตั๋วใช้ไม่ได้ ตอบ 401
func RequireLogin(iss *auth.Issuer, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !found || token == "" {
			deny(w, http.StatusUnauthorized, "LOGIN_REQUIRED", "กรุณาเข้าสู่ระบบก่อน")
			return
		}
		id, role, err := iss.Parse(token)
		if err != nil {
			deny(w, http.StatusUnauthorized, "TOKEN_INVALID", "ตั๋วใช้ไม่ได้หรือหมดอายุ กรุณาเข้าสู่ระบบใหม่")
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, Who{ID: id, Role: role})
		next(w, r.WithContext(ctx))
	}
}

// RequireRole ใช้ต่อจาก RequireLogin บทบาทไม่ตรง ตอบ 403
func RequireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if WhoFrom(r.Context()).Role != role {
			deny(w, http.StatusForbidden, "FORBIDDEN", "คุณไม่มีสิทธิ์เข้าถึงข้อมูลนี้")
			return
		}
		next(w, r)
	}
}
