package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"techstock/store"
)

type productReq struct {
	Name         string  `json:"name"`
	CategoryID   int64   `json:"category_id"`
	Description  string  `json:"description"`
	SerialNumber string  `json:"serial_number"`
	Price        float64 `json:"price"`
	Stock        int     `json:"stock"`
	ImageURL     string  `json:"image_url"`
}

func (p productReq) validate(creating bool) map[string]string {
	errs := map[string]string{}
	if strings.TrimSpace(p.Name) == "" {
		errs["name"] = "กรุณากรอกชื่อสินค้า"
	}
	if p.Price <= 0 {
		errs["price"] = "ราคาต้องมากกว่า 0"
	}
	if p.CategoryID <= 0 {
		errs["category_id"] = "กรุณาเลือกหมวดหมู่"
	}
	if len(strings.TrimSpace(p.SerialNumber)) > 64 {
		errs["serial_number"] = "Serial Number ยาวเกิน 64 ตัวอักษร"
	}
	if creating && p.Stock < 0 {
		errs["stock"] = "สต็อกห้ามติดลบ"
	}
	return errs
}

func (p productReq) toProduct() store.Product {
	return store.Product{
		Name: strings.TrimSpace(p.Name), CategoryID: p.CategoryID, Description: p.Description,
		SerialNumber: strings.TrimSpace(p.SerialNumber),
		Price:        p.Price, Stock: p.Stock, ImageURL: p.ImageURL,
	}
}

func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var categoryID int64
	if s := q.Get("category_id"); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil || id < 0 {
			writeError(w, http.StatusBadRequest, "BAD_ID", "category_id ต้องเป็นตัวเลข")
			return
		}
		categoryID = id
	}
	items, err := h.Store.ListProducts(r.Context(), strings.TrimSpace(q.Get("search")), categoryID)
	if err != nil {
		internal(w, err)
		return
	}
	send(w, http.StatusOK, "", items)
}

func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขสินค้าต้องเป็นตัวเลข")
		return
	}
	p, err := h.Store.GetProduct(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "PRODUCT_NOT_FOUND", "ไม่พบสินค้า")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	send(w, http.StatusOK, "", p)
}

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var req productReq
	if !decode(w, r, &req) {
		return
	}
	if errs := req.validate(true); len(errs) > 0 {
		invalid(w, errs)
		return
	}
	p, err := h.Store.CreateProduct(r.Context(), req.toProduct())
	if errors.Is(err, store.ErrDuplicate) {
		serialTaken(w)
		return
	}
	if errors.Is(err, store.ErrBadRef) {
		invalid(w, map[string]string{"category_id": "ไม่พบหมวดหมู่นี้"})
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	send(w, http.StatusCreated, "เพิ่มสินค้าสำเร็จ", p)
}

func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขสินค้าต้องเป็นตัวเลข")
		return
	}
	var req productReq
	if !decode(w, r, &req) {
		return
	}
	if errs := req.validate(false); len(errs) > 0 {
		invalid(w, errs)
		return
	}
	prod := req.toProduct()
	prod.ID = id
	p, err := h.Store.UpdateProduct(r.Context(), prod)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "PRODUCT_NOT_FOUND", "ไม่พบสินค้า")
	case errors.Is(err, store.ErrDuplicate):
		serialTaken(w)
	case errors.Is(err, store.ErrBadRef):
		invalid(w, map[string]string{"category_id": "ไม่พบหมวดหมู่นี้"})
	case err != nil:
		internal(w, err)
	default:
		send(w, http.StatusOK, "แก้ไขสินค้าสำเร็จ", p)
	}
}

func (h *Handler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขสินค้าต้องเป็นตัวเลข")
		return
	}
	err := h.Store.DeleteProduct(r.Context(), id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "PRODUCT_NOT_FOUND", "ไม่พบสินค้า")
	case errors.Is(err, store.ErrInUse):
		writeError(w, http.StatusConflict, "PRODUCT_IN_USE", "สินค้านี้มีประวัติสต็อกหรือคำขอเบิก ลบไม่ได้")
	case err != nil:
		internal(w, err)
	default:
		send(w, http.StatusOK, "ลบสินค้าสำเร็จ", nil)
	}
}

// ---------- categories ----------

type categoryReq struct {
	Name string `json:"name"`
}

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListCategories(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	send(w, http.StatusOK, "", items)
}

func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req categoryReq
	if !decode(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		invalid(w, map[string]string{"name": "กรุณากรอกชื่อหมวดหมู่"})
		return
	}
	c, err := h.Store.CreateCategory(r.Context(), name)
	if errors.Is(err, store.ErrDuplicate) {
		writeError(w, http.StatusConflict, "CATEGORY_EXISTS", "มีหมวดหมู่นี้อยู่แล้ว")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	send(w, http.StatusCreated, "เพิ่มหมวดหมู่สำเร็จ", c)
}

func (h *Handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขหมวดหมู่ต้องเป็นตัวเลข")
		return
	}
	var req categoryReq
	if !decode(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		invalid(w, map[string]string{"name": "กรุณากรอกชื่อหมวดหมู่"})
		return
	}
	c, err := h.Store.UpdateCategory(r.Context(), id, name)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "CATEGORY_NOT_FOUND", "ไม่พบหมวดหมู่")
	case errors.Is(err, store.ErrDuplicate):
		writeError(w, http.StatusConflict, "CATEGORY_EXISTS", "มีหมวดหมู่นี้อยู่แล้ว")
	case err != nil:
		internal(w, err)
	default:
		send(w, http.StatusOK, "แก้ไขหมวดหมู่สำเร็จ", c)
	}
}

func (h *Handler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขหมวดหมู่ต้องเป็นตัวเลข")
		return
	}
	err := h.Store.DeleteCategory(r.Context(), id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "CATEGORY_NOT_FOUND", "ไม่พบหมวดหมู่")
	case errors.Is(err, store.ErrInUse):
		writeError(w, http.StatusConflict, "CATEGORY_IN_USE", "ยังมีสินค้าในหมวดหมู่นี้ ลบไม่ได้")
	case err != nil:
		internal(w, err)
	default:
		send(w, http.StatusOK, "ลบหมวดหมู่สำเร็จ", nil)
	}
}

func serialTaken(w http.ResponseWriter) {
	writeJSON(w, http.StatusConflict, map[string]any{
		"message": "Serial Number นี้มีอยู่แล้ว",
		"code":    "SERIAL_EXISTS",
		"errors":  map[string]string{"serial_number": "Serial Number นี้มีอยู่แล้ว"},
	})
}
