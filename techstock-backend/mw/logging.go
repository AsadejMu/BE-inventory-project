package mw

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Logging จดแค่ method path status เวลา ไม่จด header ใดๆ เพราะมีตั๋วอยู่ในนั้น
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &recorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %v", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Microsecond))
	})
}

// Recovery ตาข่ายนิรภัย handler พังกลางทางก็ยังตอบ 500 เป็น JSON ได้
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %v", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]any{
					"message": "ระบบขัดข้อง ลองใหม่อีกครั้ง",
					"code":    "INTERNAL",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
