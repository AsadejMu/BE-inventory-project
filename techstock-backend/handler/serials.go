package handler

import (
	"errors"
	"net/http"
	"strings"

	"techstock/mw"
	"techstock/store"
)

// LookupSerial GET /api/serials?serial=GPU-0005 (ต้อง login)
func (h *Handler) LookupSerial(w http.ResponseWriter, r *http.Request) {
	serial := strings.TrimSpace(r.URL.Query().Get("serial"))
	if serial == "" || len(serial) > 64 {
		invalid(w, map[string]string{"serial": "กรุณากรอก Serial Number (ไม่เกิน 64 ตัวอักษร)"})
		return
	}
	info, err := h.Store.LookupSerial(r.Context(), serial)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "SERIAL_NOT_FOUND", "ไม่พบ Serial Number นี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	// ใครเบิกและใครอนุมัติ เป็นข้อมูลส่วนบุคคล ให้เฉพาะแอดมินกับเจ้าของคำขอ
	who := mw.WhoFrom(r.Context())
	if info.Type == "issued" && who.Role != "admin" && info.RequestOwner != who.ID {
		info.RequestID, info.RequestedBy, info.DecidedBy, info.DecidedAt = nil, "", "", nil
	}
	send(w, http.StatusOK, "", info)
}
