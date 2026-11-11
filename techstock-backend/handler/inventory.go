package handler

import (
	"errors"
	"net/http"
	"strings"

	"techstock/mw"
	"techstock/store"
)

type stockReq struct {
	ProductID int64  `json:"product_id"`
	Quantity  int    `json:"quantity"`
	Note      string `json:"note"`
}

func (h *Handler) StockList(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListProducts(r.Context(), "", 0)
	if err != nil {
		internal(w, err)
		return
	}
	send(w, http.StatusOK, "", items)
}

func (h *Handler) StockIn(w http.ResponseWriter, r *http.Request) {
	h.stockMove(w, r, "in", "รับสินค้าเข้าคลังสำเร็จ")
}

func (h *Handler) StockOut(w http.ResponseWriter, r *http.Request) {
	h.stockMove(w, r, "out", "บันทึกการเบิกสินค้าออกสำเร็จ")
}

func (h *Handler) stockMove(w http.ResponseWriter, r *http.Request, kind, okMsg string) {
	var req stockReq
	if !decode(w, r, &req) {
		return
	}
	errs := map[string]string{}
	if req.ProductID <= 0 {
		errs["product_id"] = "กรุณาเลือกสินค้า"
	}
	if req.Quantity <= 0 {
		errs["quantity"] = "จำนวนต้องมากกว่า 0"
	}
	if len(errs) > 0 {
		invalid(w, errs)
		return
	}
	res, err := h.Store.MoveStock(r.Context(), mw.WhoFrom(r.Context()).ID,
		req.ProductID, kind, req.Quantity, strings.TrimSpace(req.Note))
	var short *store.InsufficientStockError
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "PRODUCT_NOT_FOUND", "ไม่พบสินค้า")
	case errors.As(err, &short):
		writeError(w, http.StatusConflict, "INSUFFICIENT_STOCK", "สินค้าไม่พอ: "+short.Product)
	case err != nil:
		internal(w, err)
	default:
		send(w, http.StatusCreated, okMsg, res)
	}
}

func (h *Handler) StockHistory(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.StockHistory(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	send(w, http.StatusOK, "", items)
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	d, err := h.Store.GetDashboard(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	send(w, http.StatusOK, "", d)
}

func (h *Handler) LowStock(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListLowStock(r.Context(), store.LowStockThreshold)
	if err != nil {
		internal(w, err)
		return
	}
	send(w, http.StatusOK, "", items)
}
