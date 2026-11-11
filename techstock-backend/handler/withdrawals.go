package handler

import (
	"errors"
	"net/http"
	"strings"

	"techstock/mw"
	"techstock/store"
)

type withdrawalReq struct {
	Note  string `json:"note"`
	Items []struct {
		ProductID int64 `json:"product_id"`
		Quantity  int   `json:"quantity"`
	} `json:"items"`
}

func (h *Handler) CreateWithdrawal(w http.ResponseWriter, r *http.Request) {
	var req withdrawalReq
	if !decode(w, r, &req) {
		return
	}
	if len(req.Items) == 0 || len(req.Items) > 50 {
		invalid(w, map[string]string{"items": "เลือกสินค้า 1 ถึง 50 รายการ"})
		return
	}
	items := make([]store.WItem, 0, len(req.Items))
	for _, it := range req.Items {
		if it.ProductID <= 0 || it.Quantity <= 0 || it.Quantity > 1000 {
			invalid(w, map[string]string{"items": "ทุกรายการต้องมีสินค้า และจำนวน 1 ถึง 1,000 ชิ้น"})
			return
		}
		items = append(items, store.WItem{ProductID: it.ProductID, Quantity: it.Quantity})
	}
	// เลขผู้ขอมาจากตั๋วเท่านั้น
	wd, err := h.Store.CreateWithdrawal(r.Context(), mw.WhoFrom(r.Context()).ID, strings.TrimSpace(req.Note), items)
	if errors.Is(err, store.ErrBadRef) {
		invalid(w, map[string]string{"items": "มีสินค้าที่ไม่พบในระบบ"})
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	send(w, http.StatusCreated, "ส่งคำขอเบิกสำเร็จ", wd)
}

func (h *Handler) ListWithdrawals(w http.ResponseWriter, r *http.Request) {
	who := mw.WhoFrom(r.Context())
	items, err := h.Store.ListWithdrawals(r.Context(), who.ID, who.Role == "admin")
	if err != nil {
		internal(w, err)
		return
	}
	send(w, http.StatusOK, "", items)
}

func (h *Handler) GetWithdrawal(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขคำขอต้องเป็นตัวเลข")
		return
	}
	who := mw.WhoFrom(r.Context())
	wd, err := h.Store.GetWithdrawal(r.Context(), id)
	// สมาชิกดูได้เฉพาะของตัวเอง ของคนอื่นตอบ 404 เหมือนไม่มีอยู่ (ไม่บอกว่ามีคำขอนี้)
	if errors.Is(err, store.ErrNotFound) || (err == nil && who.Role != "admin" && wd.UserID != who.ID) {
		writeError(w, http.StatusNotFound, "WITHDRAWAL_NOT_FOUND", "ไม่พบคำขอเบิก")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	send(w, http.StatusOK, "", wd)
}

func (h *Handler) ApproveWithdrawal(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขคำขอต้องเป็นตัวเลข")
		return
	}
	err := h.Store.ApproveWithdrawal(r.Context(), id, mw.WhoFrom(r.Context()).ID)
	var short *store.InsufficientStockError
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "WITHDRAWAL_NOT_FOUND", "ไม่พบคำขอเบิก")
	case errors.Is(err, store.ErrNotPending):
		writeError(w, http.StatusConflict, "ALREADY_DECIDED", "คำขอนี้ถูกดำเนินการแล้ว")
	case errors.As(err, &short):
		writeError(w, http.StatusConflict, "INSUFFICIENT_STOCK", "สินค้าไม่พอ: "+short.Product)
	case err != nil:
		internal(w, err)
	default:
		send(w, http.StatusOK, "อนุมัติคำขอเบิกสำเร็จ", map[string]any{"id": id, "status": "approved"})
	}
}

func (h *Handler) RejectWithdrawal(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขคำขอต้องเป็นตัวเลข")
		return
	}
	err := h.Store.RejectWithdrawal(r.Context(), id, mw.WhoFrom(r.Context()).ID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "WITHDRAWAL_NOT_FOUND", "ไม่พบคำขอเบิก")
	case errors.Is(err, store.ErrNotPending):
		writeError(w, http.StatusConflict, "ALREADY_DECIDED", "คำขอนี้ถูกดำเนินการแล้ว")
	case err != nil:
		internal(w, err)
	default:
		send(w, http.StatusOK, "ปฏิเสธคำขอเบิกแล้ว", map[string]any{"id": id, "status": "rejected"})
	}
}
