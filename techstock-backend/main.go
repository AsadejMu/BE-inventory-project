package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"techstock/auth"
	"techstock/handler"
	"techstock/mw"
	"techstock/store"
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		log.Fatal("JWT_SECRET ต้องยาวอย่างน้อย 32 ตัวอักษร (สร้างด้วย: openssl rand -hex 32)")
	}
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatal("ต่อฐานข้อมูลไม่ติด ", err)
	}

	tickets := &auth.Issuer{Secret: []byte(secret), TTL: 8 * time.Hour}
	h := &handler.Handler{Store: &store.Store{Pool: pool}, Tickets: tickets}

	// ประตูตรวจตั๋ว และประตูตรวจบทบาท
	login := func(f http.HandlerFunc) http.HandlerFunc { return mw.RequireLogin(tickets, f) }
	only := func(role string, f http.HandlerFunc) http.HandlerFunc { return login(mw.RequireRole(role, f)) }

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })

	// Auth
	mux.HandleFunc("POST /api/auth/register", h.Register)
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.HandleFunc("GET /api/auth/me", login(h.Me))
	mux.HandleFunc("PUT /api/auth/profile", login(h.UpdateProfile))

	// Products (ทุกคนดูได้ / admin จัดการ)
	mux.HandleFunc("GET /api/products", h.ListProducts)
	mux.HandleFunc("GET /api/products/{id}", h.GetProduct)
	mux.HandleFunc("POST /api/products", only("admin", h.CreateProduct))
	mux.HandleFunc("PUT /api/products/{id}", only("admin", h.UpdateProduct))
	mux.HandleFunc("DELETE /api/products/{id}", only("admin", h.DeleteProduct))

	// Categories
	mux.HandleFunc("GET /api/categories", h.ListCategories)
	mux.HandleFunc("POST /api/categories", only("admin", h.CreateCategory))
	mux.HandleFunc("PUT /api/categories/{id}", only("admin", h.UpdateCategory))
	mux.HandleFunc("DELETE /api/categories/{id}", only("admin", h.DeleteCategory))

	// Stock (admin)
	mux.HandleFunc("GET /api/stock", only("admin", h.StockList))
	mux.HandleFunc("POST /api/stock/in", only("admin", h.StockIn))
	mux.HandleFunc("POST /api/stock/out", only("admin", h.StockOut))
	mux.HandleFunc("GET /api/stock/history", only("admin", h.StockHistory))

	// Withdrawals
	mux.HandleFunc("POST /api/withdrawals", only("member", h.CreateWithdrawal))
	mux.HandleFunc("GET /api/withdrawals", login(h.ListWithdrawals))
	mux.HandleFunc("GET /api/withdrawals/{id}", login(h.GetWithdrawal))
	mux.HandleFunc("PUT /api/withdrawals/{id}/approve", only("admin", h.ApproveWithdrawal))
	mux.HandleFunc("PUT /api/withdrawals/{id}/reject", only("admin", h.RejectWithdrawal))

	// ตรวจ Serial (ต้อง login)
	mux.HandleFunc("GET /api/serials", login(h.LookupSerial))

	// Members (admin)
	mux.HandleFunc("GET /api/members", only("admin", h.ListMembers))
	mux.HandleFunc("PUT /api/members/{id}", only("admin", h.UpdateMember))
	mux.HandleFunc("DELETE /api/members/{id}", only("admin", h.DeleteMember))

	// Dashboard (admin)
	mux.HandleFunc("GET /api/dashboard", only("admin", h.Dashboard))
	mux.HandleFunc("GET /api/dashboard/low-stock", only("admin", h.LowStock))

	// ทางเดินหน้าครัว: Logging > Recovery > CORS > Timeout > mux
	origins := strings.Split(getenv("ALLOWED_ORIGINS", "http://localhost:5173"), ",")
	var app http.Handler = http.TimeoutHandler(mux, 10*time.Second, "ครัวใช้เวลานานเกินไป")
	app = mw.Logging(mw.Recovery(mw.CORS(origins, app)))

	srv := &http.Server{
		Addr:              ":" + getenv("PORT", "8080"),
		Handler:           app,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	log.Println("TechStock API ฟังอยู่ที่", srv.Addr, "ต้อนรับ", origins)

	// ปิดร้านอย่างสุภาพ: ไม่รับใบสั่งใหม่ แต่รอใบที่ค้างอยู่ให้เสร็จก่อน
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("ได้รับสัญญาณปิดร้าน")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Println("ปิดไม่ทัน", err)
	}
	log.Println("ปิดร้านเรียบร้อย")
}
