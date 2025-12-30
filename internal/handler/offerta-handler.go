// offerta_handler.go
package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

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
		return "client"
	}
	return r
}

func isAllowedRole(role string) bool {
	return role == "driver" || role == "client"
}

func writeOffertaJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) handleOffertaStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	tgStr := strings.TrimSpace(r.URL.Query().Get("telegram_id"))
	role := normalizeRole(r.URL.Query().Get("role"))

	if !isAllowedRole(role) {
		writeOffertaJSON(w, http.StatusBadRequest, offertaStatusResp{
			Success: false,
			Message: "Invalid role",
		})
		return
	}

	tgID, err := strconv.ParseInt(tgStr, 10, 64)
	if err != nil || tgID == 0 {
		writeOffertaJSON(w, http.StatusBadRequest, offertaStatusResp{
			Success: false,
			Message: "Telegram ID обязателен",
		})
		return
	}

	var approveInt int
	err = h.db.QueryRow(`SELECT approve FROM offerta WHERE id_user = ? AND role = ? LIMIT 1`, tgID, role).Scan(&approveInt)

	if err == sql.ErrNoRows {
		h.logger.Info("❌ Offerta not found",
			zap.Int64("telegram_id", tgID),
			zap.String("role", role))

		writeOffertaJSON(w, http.StatusOK, offertaStatusResp{
			Success:  true,
			Approved: false,
			Role:     role,
		})
		return
	}

	if err != nil {
		h.logger.Error("Database error checking offerta", zap.Error(err))
		writeOffertaJSON(w, http.StatusInternalServerError, offertaStatusResp{
			Success: false,
			Message: "Server error",
		})
		return
	}

	approved := approveInt == 1

	h.logger.Info("✅ Offerta status checked",
		zap.Int64("telegram_id", tgID),
		zap.String("role", role),
		zap.Bool("approved", approved))

	writeOffertaJSON(w, http.StatusOK, offertaStatusResp{
		Success:  true,
		Approved: approved,
		Role:     role,
	})
}

func (h *Handler) handleOffertaApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		writeOffertaJSON(w, http.StatusMethodNotAllowed, Response{Success: false, Message: "Method not allowed"})
		return
	}

	var req offertaApproveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Invalid JSON", zap.Error(err))
		writeOffertaJSON(w, http.StatusBadRequest, Response{Success: false, Message: "Invalid JSON"})
		return
	}

	req.Role = normalizeRole(req.Role)

	if req.TelegramID == 0 {
		h.logger.Warn("Missing telegram_id in approval request")
		writeOffertaJSON(w, http.StatusBadRequest, Response{Success: false, Message: "telegram_id обязателен"})
		return
	}

	if !isAllowedRole(req.Role) {
		writeOffertaJSON(w, http.StatusBadRequest, Response{Success: false, Message: "Invalid role"})
		return
	}

	now := time.Now()

	// ✅ CRITICAL: Use transaction to ensure immediate visibility
	tx, err := h.db.Begin()
	if err != nil {
		h.logger.Error("Failed to begin transaction", zap.Error(err))
		writeOffertaJSON(w, http.StatusInternalServerError, Response{Success: false, Message: "Server error"})
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO offerta (id_user, role, approve, created_at, updated_at)
		VALUES (?, ?, 1, ?, ?)
		ON CONFLICT(id_user, role) 
		DO UPDATE SET approve = 1, updated_at = ?
	`, req.TelegramID, req.Role, now, now, now)

	if err != nil {
		h.logger.Error("Failed to save offerta",
			zap.Error(err),
			zap.Int64("telegram_id", req.TelegramID),
			zap.String("role", req.Role))
		writeOffertaJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "Database error: " + err.Error(),
		})
		return
	}

	// ✅ CRITICAL: Commit transaction before responding
	if err := tx.Commit(); err != nil {
		h.logger.Error("Failed to commit transaction", zap.Error(err))
		writeOffertaJSON(w, http.StatusInternalServerError, Response{Success: false, Message: "Commit error"})
		return
	}

	h.logger.Info("✅ Offerta approved successfully",
		zap.Int64("telegram_id", req.TelegramID),
		zap.String("role", req.Role))

	writeOffertaJSON(w, http.StatusOK, Response{Success: true, Message: "Offerta approved"})
}
