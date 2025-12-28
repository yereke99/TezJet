// offerta-handler.go
package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

type offertaStatusResp struct {
	Success  bool   `json:"success"`
	Approved bool   `json:"approved"`
	Role     string `json:"role,omitempty"`
	Message  string `json:"message,omitempty"`
}

type offertaApproveReq struct {
	TelegramID int64  `json:"telegram_id"`
	Role       string `json:"role"`
}

func normalizeRole(role string) string {
	r := strings.ToLower(strings.TrimSpace(role))
	if r == "" {
		return "driver"
	}
	return r
}

func isAllowedRole(role string) bool {
	switch role {
	case "driver", "client":
		return true
	default:
		return false
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) handleOffertaStatus(w http.ResponseWriter, r *http.Request) {
	// GET /api/offerta/status?telegram_id=...&role=driver|client
	tgStr := strings.TrimSpace(r.URL.Query().Get("telegram_id"))
	role := normalizeRole(r.URL.Query().Get("role"))

	if !isAllowedRole(role) {
		writeJSON(w, http.StatusBadRequest, offertaStatusResp{
			Success: false, Approved: false, Message: "Invalid role. Use driver|client",
		})
		return
	}

	tgID, _ := strconv.ParseInt(tgStr, 10, 64)
	if tgID == 0 {
		writeJSON(w, http.StatusBadRequest, offertaStatusResp{
			Success: false, Approved: false, Message: "Telegram ID обязателен",
		})
		return
	}

	// approve stored as INTEGER 0/1
	var approveInt int
	err := h.db.QueryRow(
		`SELECT approve FROM offerta WHERE id_user = ? AND role = ? LIMIT 1`,
		tgID, role,
	).Scan(&approveInt)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, offertaStatusResp{Success: true, Approved: false, Role: role})
		return
	}
	if err != nil {
		h.logger.Error("offerta status query failed", zap.Error(err))
		writeJSON(w, http.StatusInternalServerError, offertaStatusResp{
			Success: false, Approved: false, Message: "Server error",
		})
		return
	}

	writeJSON(w, http.StatusOK, offertaStatusResp{
		Success:  true,
		Approved: approveInt == 1,
		Role:     role,
	})
}

func (h *Handler) handleOffertaApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, Response{Success: false, Message: "Method not allowed"})
		return
	}

	var req offertaApproveReq
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Success: false, Message: "Invalid JSON"})
		return
	}

	req.Role = normalizeRole(req.Role)

	if req.TelegramID == 0 {
		writeJSON(w, http.StatusBadRequest, Response{Success: false, Message: "telegram_id обязателен"})
		return
	}
	if !isAllowedRole(req.Role) {
		writeJSON(w, http.StatusBadRequest, Response{Success: false, Message: "Invalid role. Use driver|client"})
		return
	}

	// ✅ правильный UPSERT по UNIQUE(id_user, role)
	_, err := h.db.Exec(`
		INSERT INTO offerta (id_user, role, approve, created_at, updated_at)
		VALUES (?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(id_user, role) DO UPDATE SET
			approve = 1,
			updated_at = CURRENT_TIMESTAMP
	`, req.TelegramID, req.Role)

	if err != nil {
		h.logger.Error("offerta approve failed", zap.Error(err))
		writeJSON(w, http.StatusInternalServerError, Response{Success: false, Message: "Ошибка сохранения"})
		return
	}

	writeJSON(w, http.StatusOK, Response{Success: true, Message: "OK"})
}
