package mw

import (
	"net/http"
	"strings"
)

// CORS อนุญาตเฉพาะ origin ที่อยู่ในรายชื่อ (เช่น หน้าเว็บบน Vercel)
func CORS(allowed []string, next http.Handler) http.Handler {
	ok := map[string]bool{}
	for _, o := range allowed {
		ok[strings.TrimSpace(o)] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if ok[origin] {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			h.Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return // ใบสั่งถามทาง จบตรงนี้ ไม่ต้องเข้า handler
		}
		next.ServeHTTP(w, r)
	})
}
