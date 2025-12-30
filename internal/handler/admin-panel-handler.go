package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// DriverShort represents minimal driver info for admin operations
type DriverShort struct {
	ID         string
	TelegramID int64
	FirstName  string
	LastName   string
	Status     string
	IsVerified bool
	Contact    string
}

// AdminDriver represents full driver details for admin panel
type AdminDriver struct {
	ID            string  `json:"id"`
	TelegramID    int64   `json:"telegram_id"`
	FirstName     string  `json:"first_name"`
	LastName      string  `json:"last_name"`
	Birthday      string  `json:"birthday"`
	ContactNumber string  `json:"contact_number"`
	StartCity     string  `json:"start_city"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	ProfilePhoto  string  `json:"profile_photo"`
	LicenseFront  string  `json:"license_front"`
	LicenseBack   string  `json:"license_back"`
	TruckType     string  `json:"truck_type"`
	IsVerified    bool    `json:"is_verified"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	ApprovedAt    *string `json:"approved_at,omitempty"`
	ApprovedBy    *string `json:"approved_by,omitempty"`
}

// AdminOrder represents full order details for admin panel
type AdminOrder struct {
	ID          string  `json:"id"`
	TelegramID  int64   `json:"telegram_id"`
	FromAddress string  `json:"from_address"`
	FromLat     float64 `json:"from_lat"`
	FromLon     float64 `json:"from_lon"`
	ToAddress   string  `json:"to_address"`
	ToLat       float64 `json:"to_lat"`
	ToLon       float64 `json:"to_lon"`
	DistanceKm  float64 `json:"distance_km"`
	EtaMin      int     `json:"eta_min"`
	Price       int     `json:"price"`
	TruckType   string  `json:"truck_type"`
	Contact     string  `json:"contact"`
	TimeStart   string  `json:"time_start"`
	Comment     string  `json:"comment"`
	ItemPhoto   string  `json:"item_photo_path"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// DayStat represents statistics for a single day
type DayStat struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// ==================== ADMIN MESSAGE SENDING ====================

// SendDriverMessage sends a custom message from admin to a driver
// POST /api/admin/drivers/{id}/message?telegram_id=...
func (h *Handler) SendDriverMessage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	driverID := vars["id"]

	// Get admin telegram ID from query
	adminTidStr := r.URL.Query().Get("telegram_id")
	if adminTidStr == "" {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "telegram_id қажет",
		})
		return
	}

	adminTid, err := strconv.ParseInt(adminTidStr, 10, 64)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "telegram_id дұрыс емес",
		})
		return
	}

	// Verify admin permissions
	if !h.isAdmin(adminTid) {
		h.writeJSON(w, http.StatusForbidden, Response{
			Success: false,
			Message: "Тек әкімшіге рұқсат",
		})
		return
	}

	// Parse request body
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "JSON қате",
		})
		return
	}

	// Validate message content
	message := strings.TrimSpace(req.Message)
	if message == "" {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "Хабарлама бос болмауы керек",
		})
		return
	}

	if len(message) < 10 {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "Хабарлама тым қысқа (кемінде 10 таңба)",
		})
		return
	}

	if len(message) > 4000 {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "Хабарлама тым ұзын (максимум 4000 таңба)",
		})
		return
	}

	// Get driver data from database
	var driver DriverShort
	err = h.db.QueryRowContext(ctx,
		`SELECT id, telegram_id, first_name, last_name, status, is_verified, contact_number
         FROM drivers
         WHERE id = ?`,
		driverID,
	).Scan(&driver.ID, &driver.TelegramID, &driver.FirstName, &driver.LastName,
		&driver.Status, &driver.IsVerified, &driver.Contact)

	if err == sql.ErrNoRows {
		h.writeJSON(w, http.StatusNotFound, Response{
			Success: false,
			Message: "Жүргізуші табылмады",
		})
		return
	}
	if err != nil {
		h.logErr("select driver for message", err)
		h.writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "Деректер базасының қатесі",
		})
		return
	}

	// Send message to driver synchronously
	err = h.sendAdminMessageSync(ctx, driver, message)
	if err != nil {
		h.logErr("send admin message", err)
		h.writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "Хабарлама жіберу сәтсіз: " + err.Error(),
		})
		return
	}

	// Success response
	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Хабарлама сәтті жіберілді",
		Data: map[string]interface{}{
			"driver_id":   driverID,
			"driver_name": driver.FirstName + " " + driver.LastName,
			"sent_at":     time.Now().Format(time.RFC3339),
		},
	})
}

// sendAdminMessageSync sends admin message synchronously with proper HTML escaping
func (h *Handler) sendAdminMessageSync(ctx context.Context, d DriverShort, message string) error {
	if d.TelegramID == 0 {
		return fmt.Errorf("жүргізушінің telegram ID жоқ")
	}

	if h.bot == nil {
		return fmt.Errorf("telegram bot инициализацияланбаған")
	}

	// CRITICAL: Escape ALL user input for HTML
	escapedFirstName := html.EscapeString(d.FirstName)
	escapedMessage := html.EscapeString(message)

	// Format message with HTML tags
	text := fmt.Sprintf(
		"📢 <b>ALASH-GO ӘКІМШІ ХАБАРЛАМАСЫ</b>\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n\n"+
			"Құрметті <b>%s</b>!\n\n"+
			"%s\n\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n\n"+
			"Сұрақтарыңыз болса, бізге хабарласыңыз:\n"+
			"WhatsApp: +7 (776) 900-10-29\n\n"+
			"<i>Рахмет, Alash-Go командасы</i>",
		escapedFirstName,
		escapedMessage,
	)

	// WhatsApp contact button
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{
					Text: "💬 WhatsApp хабарласу",
					URL:  "https://wa.me/87769001029",
				},
			},
		},
	}

	// Send message with HTML parse mode
	_, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      d.TelegramID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		return fmt.Errorf("Telegram API қатесі: %v", err)
	}

	// Log successful send
	h.logger.Info("Admin message sent successfully",
		zap.Int64("telegram_id", d.TelegramID),
		zap.String("driver_id", d.ID),
		zap.String("message_preview", truncateString(message, 50)))

	return nil
}

// ==================== DRIVER BLOCKING ====================

// RejectDriver blocks a driver with a specific reason
// POST /api/admin/drivers/{id}/reject?telegram_id=...
func (h *Handler) RejectDriver(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	driverID := vars["id"]

	// Get admin telegram ID
	adminTidStr := r.URL.Query().Get("telegram_id")
	if adminTidStr == "" {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "telegram_id қажет",
		})
		return
	}

	adminTid, err := strconv.ParseInt(adminTidStr, 10, 64)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "telegram_id дұрыс емес",
		})
		return
	}

	// Verify admin permissions
	if !h.isAdmin(adminTid) {
		h.writeJSON(w, http.StatusForbidden, Response{
			Success: false,
			Message: "Тек әкімшіге рұқсат",
		})
		return
	}

	// Parse request body
	var req struct {
		ReasonType   string `json:"reason_type"`
		CustomReason string `json:"custom_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "JSON қате",
		})
		return
	}

	// Validate reason type
	if req.ReasonType == "" {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "Блоктау себебі міндетті",
		})
		return
	}

	// Validate custom reason if selected
	if req.ReasonType == "custom" {
		customReason := strings.TrimSpace(req.CustomReason)
		if customReason == "" {
			h.writeJSON(w, http.StatusBadRequest, Response{
				Success: false,
				Message: "Өз себебіңізді жазыңыз",
			})
			return
		}
		req.CustomReason = customReason
	}

	// Start transaction
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		h.logErr("begin transaction", err)
		h.writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "Транзакция қатесі",
		})
		return
	}
	defer tx.Rollback()

	// Get driver data
	var driver DriverShort
	err = tx.QueryRowContext(ctx,
		`SELECT id, telegram_id, first_name, last_name, status, is_verified, contact_number
         FROM drivers
         WHERE id = ?`,
		driverID,
	).Scan(&driver.ID, &driver.TelegramID, &driver.FirstName, &driver.LastName,
		&driver.Status, &driver.IsVerified, &driver.Contact)

	if err == sql.ErrNoRows {
		h.writeJSON(w, http.StatusNotFound, Response{
			Success: false,
			Message: "Жүргізуші табылмады",
		})
		return
	}
	if err != nil {
		h.logErr("select driver for rejection", err)
		h.writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "Деректер базасының қатесі",
		})
		return
	}

	// Update driver status to rejected
	_, err = tx.ExecContext(ctx,
		`UPDATE drivers
         SET status = 'rejected',
             is_verified = 0,
             updated_at = CURRENT_TIMESTAMP,
             approved_by = ?
         WHERE id = ?`,
		adminTidStr, driverID,
	)
	if err != nil {
		h.logErr("update driver status to rejected", err)
		h.writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "Статусты жаңарту қатесі",
		})
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		h.logErr("commit rejection transaction", err)
		h.writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "Транзакцияны сақтау қатесі",
		})
		return
	}

	// Send notification to driver asynchronously
	go h.notifyDriverBlocked(context.Background(), driver, req.ReasonType, req.CustomReason)

	// Success response
	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Жүргізуші сәтті блокталды",
		Data: map[string]interface{}{
			"driver_id":   driverID,
			"driver_name": driver.FirstName + " " + driver.LastName,
			"status":      "rejected",
			"reason_type": req.ReasonType,
			"blocked_at":  time.Now().Format(time.RFC3339),
		},
	})
}

// notifyDriverBlocked sends block notification to driver
func (h *Handler) notifyDriverBlocked(ctx context.Context, d DriverShort, reasonType, customReason string) {
	if d.TelegramID == 0 || h.bot == nil {
		return
	}

	// Build reason message based on type (bilingual - Kazakh/Russian)
	var reasonMessage string
	switch reasonType {
	case "incorrect_documents":
		reasonMessage = "📄 <b>Себебі / Причина:</b> Құжаттарыңыз платформа талаптарына сәйкес келмейді / " +
			"Ваши документы не соответствуют требованиям платформы.\n\n" +
			"Құжаттарды қайта тексеріп, дұрыс және анық фотосуреттерді жүктеңіз / " +
			"Проверьте документы и загрузите четкие фотографии."

	case "name_mismatch":
		reasonMessage = "👤 <b>Себебі / Причина:</b> Профильдегі аты-жөніңіз құжаттармен сәйкес келмейді / " +
			"Ваше имя в профиле не совпадает с документами.\n\n" +
			"Профильде және құжаттарда бірдей аты-жөні болуы керек / " +
			"Имя в профиле и документах должно совпадать."

	case "photo_mismatch":
		reasonMessage = "📸 <b>Себебі / Причина:</b> Профиль фотосуретіңіз құжаттардағы суретпен сәйкес келмейді / " +
			"Ваше фото профиля не совпадает с фото в документах.\n\n" +
			"Құжаттардағы және профильдегі фото бір адам болуы керек / " +
			"Фото в документах и профиле должны быть одного человека."

	case "payment_required":
		reasonMessage = "💳 <b>Себебі / Причина:</b> Платформаға қол жеткізу үшін төлем төленбеген / " +
			"Оплата за доступ к платформе не произведена.\n\n" +
			"Қызметті жалғастыру үшін төлемді орындаңыз / " +
			"Произведите оплату для продолжения работы."

	case "custom":
		if customReason != "" {
			// CRITICAL: Escape custom reason for HTML
			escapedReason := html.EscapeString(strings.TrimSpace(customReason))
			reasonMessage = fmt.Sprintf("✍️ <b>Себебі / Причина:</b>\n%s", escapedReason)
		} else {
			reasonMessage = "✍️ <b>Себебі / Причина:</b> Әкімші көрсеткен себептер бойынша / " +
				"По причинам, указанным администратором."
		}

	default:
		reasonMessage = "Блоктау себебі көрсетілмеген / Причина блокировки не указана."
	}

	// CRITICAL: Escape driver name for HTML
	escapedFirstName := html.EscapeString(d.FirstName)

	// Format notification message
	text := fmt.Sprintf(
		"🚫 <b>Alash-Go Әкімші Хабарламасы / Сообщение Администрации</b>\n\n"+
			"Сәлеметсіз бе / Здравствуйте, <b>%s</b>!\n\n"+
			"Өкінішке орай, сіздің Alash-Go жүргізуші аккаунтыңыз уақытша блокталды / "+
			"К сожалению, ваш аккаунт водителя Alash-Go временно заблокирован.\n\n"+
			"%s\n\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n\n"+
			"📞 <b>Аккаунтты қалпына келтіру үшін / Для восстановления аккаунта:</b>\n\n"+
			"WhatsApp арқылы бізге хабарласыңыз / Свяжитесь с нами через WhatsApp:\n"+
			"+7 (776) 900-10-29\n\n"+
			"Біз сізге көмектесуге әрқашан дайынбыз! / Мы всегда готовы помочь! 🤝\n\n"+
			"<i>Рахмет / Спасибо, Alash-Go командасы 🚀</i>",
		escapedFirstName,
		reasonMessage,
	)

	// WhatsApp contact button
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{
					Text: "💬 WhatsApp хабарласу / Связаться",
					URL:  "https://wa.me/87769001029",
				},
			},
		},
	}

	// Send notification
	_, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      d.TelegramID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		h.logErr("send block notification", err)
	} else {
		h.logger.Info("Block notification sent",
			zap.Int64("telegram_id", d.TelegramID),
			zap.String("driver_id", d.ID),
			zap.String("reason_type", reasonType))
	}
}

// ==================== DRIVER UNBLOCKING ====================

// UnblockDriver unblocks a previously blocked driver
// POST /api/admin/drivers/{id}/unblock?telegram_id=...
func (h *Handler) UnblockDriver(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	driverID := vars["id"]

	// Get admin telegram ID
	adminTidStr := r.URL.Query().Get("telegram_id")
	if adminTidStr == "" {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "telegram_id қажет",
		})
		return
	}

	adminTid, err := strconv.ParseInt(adminTidStr, 10, 64)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "telegram_id дұрыс емес",
		})
		return
	}

	// Verify admin permissions
	if !h.isAdmin(adminTid) {
		h.writeJSON(w, http.StatusForbidden, Response{
			Success: false,
			Message: "Тек әкімшіге рұқсат",
		})
		return
	}

	// Start transaction
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		h.logErr("begin transaction", err)
		h.writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "Транзакция қатесі",
		})
		return
	}
	defer tx.Rollback()

	// Get driver data
	var driver DriverShort
	err = tx.QueryRowContext(ctx,
		`SELECT id, telegram_id, first_name, last_name, status, is_verified, contact_number
         FROM drivers
         WHERE id = ?`,
		driverID,
	).Scan(&driver.ID, &driver.TelegramID, &driver.FirstName, &driver.LastName,
		&driver.Status, &driver.IsVerified, &driver.Contact)

	if err == sql.ErrNoRows {
		h.writeJSON(w, http.StatusNotFound, Response{
			Success: false,
			Message: "Жүргізуші табылмады",
		})
		return
	}
	if err != nil {
		h.logErr("select driver for unblocking", err)
		h.writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "Деректер базасының қатесі",
		})
		return
	}

	// Check if driver is actually blocked
	if driver.Status != "rejected" && driver.Status != "suspended" {
		h.writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "Жүргізуші блокталмаған",
		})
		return
	}

	// Update driver status to pending (requires re-verification)
	_, err = tx.ExecContext(ctx,
		`UPDATE drivers
         SET status = 'pending',
             is_verified = 0,
             updated_at = CURRENT_TIMESTAMP,
             approved_by = NULL,
             approved_at = NULL
         WHERE id = ?`,
		driverID,
	)
	if err != nil {
		h.logErr("update driver status to pending", err)
		h.writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "Статусты жаңарту қатесі",
		})
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		h.logErr("commit unblock transaction", err)
		h.writeJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "Транзакцияны сақтау қатесі",
		})
		return
	}

	// Send notification to driver asynchronously
	go h.notifyDriverUnblocked(context.Background(), driver)

	// Success response
	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Жүргізуші блоктан шығарылды",
		Data: map[string]interface{}{
			"driver_id":    driverID,
			"driver_name":  driver.FirstName + " " + driver.LastName,
			"status":       "pending",
			"unblocked_at": time.Now().Format(time.RFC3339),
		},
	})
}

// notifyDriverUnblocked sends unblock notification to driver
func (h *Handler) notifyDriverUnblocked(ctx context.Context, d DriverShort) {
	if d.TelegramID == 0 || h.bot == nil {
		return
	}

	// CRITICAL: Escape driver name for HTML
	escapedFirstName := html.EscapeString(d.FirstName)

	// Format notification message (bilingual)
	text := fmt.Sprintf(
		"✅ <b>Alash-Go Хабарламасы / Сообщение</b>\n\n"+
			"Құрметті / Уважаемый <b>%s</b>!\n\n"+
			"🎉 <b>Сіздің аккаунтыңыз қалпына келтірілді! / Ваш аккаунт восстановлен!</b>\n\n"+
			"Сіз қазір Alash-Go платформасында жүргізуші ретінде жұмыс жасай аласыз / "+
			"Теперь вы можете работать водителем на платформе Alash-Go.\n\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n\n"+
			"📋 <b>Келесі қадам / Следующий шаг:</b>\n\n"+
			"1️⃣ Платформада тіркелуіңізді аяқтаңыз / Завершите регистрацию\n"+
			"2️⃣ Жүк тасымалдау өтінімдерін алуды бастаңыз / Начните получать заказы\n"+
			"3️⃣ Табыс табыңыз! 💰 / Зарабатывайте! 💰\n\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n\n"+
			"📞 <b>Көмек керек пе? / Нужна помощь?</b>\n\n"+
			"Кез келген сұрақтар бойынша бізге хабарласыңыз / "+
			"По любым вопросам свяжитесь с нами:\n\n"+
			"💬 WhatsApp: +7 (776) 900-10-29\n\n"+
			"Сәттілік тілейміз! / Желаем успехов! 🚀\n"+
			"<i>Alash-Go командасы / команда</i>",
		escapedFirstName,
	)

	// WhatsApp contact button
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{
					Text: "💬 WhatsApp хабарласу / Связаться",
					URL:  "https://wa.me/87769001029",
				},
			},
		},
	}

	// Send notification
	_, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      d.TelegramID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		h.logErr("send unblock notification", err)
	} else {
		h.logger.Info("Unblock notification sent",
			zap.Int64("telegram_id", d.TelegramID),
			zap.String("driver_id", d.ID))
	}
}

// ==================== ADMIN DASHBOARD API ====================

// handleAdminSummary returns admin dashboard statistics
// GET /api/admin/summary?telegram_id=...
func (h *Handler) handleAdminSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get telegram ID from query parameter
	var telegramID int64
	if v := r.URL.Query().Get("telegram_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			telegramID = id
		}
	}

	// Verify admin permissions
	if !h.isAdmin(telegramID) {
		h.sendErrorResponse(w, "Рұқсат жоқ / Нет доступа", http.StatusForbidden)
		return
	}

	// Get driver statistics
	var totalDrivers, pendingDrivers, approvedDrivers, rejectedDrivers int
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM drivers`).Scan(&totalDrivers)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM drivers WHERE status='pending'`).Scan(&pendingDrivers)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM drivers WHERE status='approved'`).Scan(&approvedDrivers)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM drivers WHERE status='rejected'`).Scan(&rejectedDrivers)

	// Get order statistics
	var totalOrders, pendingOrders, matchedOrders, inProgressOrders, completedOrders, cancelledOrders int
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM delivery_requests`).Scan(&totalOrders)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM delivery_requests WHERE status='pending'`).Scan(&pendingOrders)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM delivery_requests WHERE status='matched'`).Scan(&matchedOrders)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM delivery_requests WHERE status='in_progress'`).Scan(&inProgressOrders)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM delivery_requests WHERE status='completed'`).Scan(&completedOrders)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM delivery_requests WHERE status='cancelled'`).Scan(&cancelledOrders)

	// Get orders by day (last 7 days)
	rows, err := h.db.Query(`
		SELECT strftime('%Y-%m-%d', created_at) as d, COUNT(*) 
		FROM delivery_requests
		WHERE created_at >= datetime('now', '-7 days')
		GROUP BY d
		ORDER BY d ASC`)

	var ordersByDay []DayStat
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var s DayStat
			if err := rows.Scan(&s.Date, &s.Count); err == nil {
				ordersByDay = append(ordersByDay, s)
			}
		}
	} else {
		h.logger.Error("Failed to query orders by day", zap.Error(err))
	}

	// Get drivers by day (last 7 days)
	rows2, err2 := h.db.Query(`
		SELECT strftime('%Y-%m-%d', created_at) as d, COUNT(*) 
		FROM drivers
		WHERE created_at >= datetime('now', '-7 days')
		GROUP BY d
		ORDER BY d ASC`)

	var driversByDay []DayStat
	if err2 == nil {
		defer rows2.Close()
		for rows2.Next() {
			var s DayStat
			if err := rows2.Scan(&s.Date, &s.Count); err == nil {
				driversByDay = append(driversByDay, s)
			}
		}
	} else {
		h.logger.Error("Failed to query drivers by day", zap.Error(err2))
	}

	// Build response
	resp := map[string]interface{}{
		"drivers": map[string]interface{}{
			"total":    totalDrivers,
			"pending":  pendingDrivers,
			"approved": approvedDrivers,
			"rejected": rejectedDrivers,
		},
		"orders": map[string]interface{}{
			"total":       totalOrders,
			"pending":     pendingOrders,
			"matched":     matchedOrders,
			"in_progress": inProgressOrders,
			"completed":   completedOrders,
			"cancelled":   cancelledOrders,
		},
		"charts": map[string]interface{}{
			"orders_by_day":  ordersByDay,
			"drivers_by_day": driversByDay,
		},
	}

	h.sendSuccessResponse(w, "Admin summary", resp)
}

// handleAdminDrivers returns list of all drivers for admin panel
// GET /api/admin/drivers?telegram_id=...
func (h *Handler) handleAdminDrivers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get telegram ID
	var telegramID int64
	if v := r.URL.Query().Get("telegram_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			telegramID = id
		}
	}

	// Verify admin permissions
	if !h.isAdmin(telegramID) {
		h.sendErrorResponse(w, "Рұқсат жоқ / Нет доступа", http.StatusForbidden)
		return
	}

	// Query drivers
	rows, err := h.db.Query(`
		SELECT 
			id, telegram_id, first_name, last_name, birthday,
			contact_number, start_city, latitude, longitude,
			profile_photo, license_front, license_back,
			truck_type, is_verified, status, created_at, updated_at
		FROM drivers
		ORDER BY created_at DESC
		LIMIT 200`)

	if err != nil {
		h.logger.Error("Failed to query drivers", zap.Error(err))
		h.sendErrorResponse(w, "Қате / Ошибка", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Parse results
	var drivers []AdminDriver
	for rows.Next() {
		var d AdminDriver
		var createdAt, updatedAt time.Time
		if err := rows.Scan(
			&d.ID, &d.TelegramID, &d.FirstName, &d.LastName, &d.Birthday,
			&d.ContactNumber, &d.StartCity, &d.Latitude, &d.Longitude,
			&d.ProfilePhoto, &d.LicenseFront, &d.LicenseBack,
			&d.TruckType, &d.IsVerified, &d.Status, &createdAt, &updatedAt,
		); err != nil {
			h.logger.Warn("Failed to scan driver row", zap.Error(err))
			continue
		}
		d.CreatedAt = createdAt.Format(time.RFC3339)
		d.UpdatedAt = updatedAt.Format(time.RFC3339)
		drivers = append(drivers, d)
	}

	h.sendSuccessResponse(w, "Admin drivers", map[string]interface{}{
		"count":   len(drivers),
		"drivers": drivers,
	})
}

// handleAdminDriverDetail returns detailed information about a specific driver
// GET /api/admin/drivers/{id}?telegram_id=...
func (h *Handler) handleAdminDriverDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get telegram ID
	var telegramID int64
	if v := r.URL.Query().Get("telegram_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			telegramID = id
		}
	}

	// Verify admin permissions
	if !h.isAdmin(telegramID) {
		h.sendErrorResponse(w, "Рұқсат жоқ / Нет доступа", http.StatusForbidden)
		return
	}

	// Get driver ID from URL
	vars := mux.Vars(r)
	driverID := vars["id"]
	if driverID == "" {
		h.sendErrorResponse(w, "Driver ID қажет / нужен", http.StatusBadRequest)
		return
	}

	// Query driver details
	query := `
		SELECT 
			id, telegram_id, first_name, last_name, birthday,
			contact_number, start_city, latitude, longitude,
			profile_photo, license_front, license_back,
			truck_type, is_verified, status, created_at, updated_at, approved_at, approved_by
		FROM drivers
		WHERE id = ?`

	var d AdminDriver
	var createdAt, updatedAt time.Time
	var approvedAt sql.NullTime
	var approvedBy sql.NullString

	err := h.db.QueryRow(query, driverID).Scan(
		&d.ID, &d.TelegramID, &d.FirstName, &d.LastName, &d.Birthday,
		&d.ContactNumber, &d.StartCity, &d.Latitude, &d.Longitude,
		&d.ProfilePhoto, &d.LicenseFront, &d.LicenseBack,
		&d.TruckType, &d.IsVerified, &d.Status, &createdAt, &updatedAt, &approvedAt, &approvedBy,
	)

	if err == sql.ErrNoRows {
		h.sendErrorResponse(w, "Driver табылмады / не найден", http.StatusNotFound)
		return
	}
	if err != nil {
		h.logger.Error("Failed to query driver detail", zap.Error(err))
		h.sendErrorResponse(w, "Қате / Ошибка", http.StatusInternalServerError)
		return
	}

	// Format timestamps
	d.CreatedAt = createdAt.Format(time.RFC3339)
	d.UpdatedAt = updatedAt.Format(time.RFC3339)

	if approvedAt.Valid {
		approvedAtStr := approvedAt.Time.Format(time.RFC3339)
		d.ApprovedAt = &approvedAtStr
	}
	if approvedBy.Valid {
		d.ApprovedBy = &approvedBy.String
	}

	// Build response
	resp := map[string]interface{}{
		"id":             d.ID,
		"telegram_id":    d.TelegramID,
		"first_name":     d.FirstName,
		"last_name":      d.LastName,
		"birthday":       d.Birthday,
		"contact_number": d.ContactNumber,
		"start_city":     d.StartCity,
		"latitude":       d.Latitude,
		"longitude":      d.Longitude,
		"profile_photo":  d.ProfilePhoto,
		"license_front":  d.LicenseFront,
		"license_back":   d.LicenseBack,
		"truck_type":     d.TruckType,
		"is_verified":    d.IsVerified,
		"status":         d.Status,
		"created_at":     d.CreatedAt,
		"updated_at":     d.UpdatedAt,
	}

	if d.ApprovedAt != nil {
		resp["approved_at"] = *d.ApprovedAt
	}
	if d.ApprovedBy != nil {
		resp["approved_by"] = *d.ApprovedBy
	}

	h.sendSuccessResponse(w, "Driver detail", resp)
}

// handleAdminOrders returns list of all orders for admin panel
// GET /api/admin/orders?telegram_id=...
func (h *Handler) handleAdminOrders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get telegram ID
	var telegramID int64
	if v := r.URL.Query().Get("telegram_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			telegramID = id
		}
	}

	// Verify admin permissions
	if !h.isAdmin(telegramID) {
		h.sendErrorResponse(w, "Рұқсат жоқ / Нет доступа", http.StatusForbidden)
		return
	}

	// Query orders
	rows, err := h.db.Query(`
		SELECT 
			id, telegram_id, from_address, from_lat, from_lon,
			to_address, to_lat, to_lon, distance_km, eta_min,
			price, truck_type, contact, time_start, comment,
			item_photo_path, status, created_at, updated_at
		FROM delivery_requests
		ORDER BY created_at DESC
		LIMIT 200`)

	if err != nil {
		h.logger.Error("Failed to query orders", zap.Error(err))
		h.sendErrorResponse(w, "Қате / Ошибка", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Parse results
	var orders []AdminOrder
	for rows.Next() {
		var o AdminOrder
		var createdAt, updatedAt time.Time
		if err := rows.Scan(
			&o.ID, &o.TelegramID, &o.FromAddress, &o.FromLat, &o.FromLon,
			&o.ToAddress, &o.ToLat, &o.ToLon, &o.DistanceKm, &o.EtaMin,
			&o.Price, &o.TruckType, &o.Contact, &o.TimeStart, &o.Comment,
			&o.ItemPhoto, &o.Status, &createdAt, &updatedAt,
		); err != nil {
			h.logger.Warn("Failed to scan order row", zap.Error(err))
			continue
		}
		o.CreatedAt = createdAt.Format(time.RFC3339)
		o.UpdatedAt = updatedAt.Format(time.RFC3339)
		orders = append(orders, o)
	}

	h.sendSuccessResponse(w, "Admin orders", map[string]interface{}{
		"count":  len(orders),
		"orders": orders,
	})
}

// ==================== HELPER FUNCTIONS ====================

// isAdmin checks if the given telegram ID belongs to an admin
func (h *Handler) isAdmin(telegramID int64) bool {
	return h.cfg.AdminTelegramID != 0 && h.cfg.AdminTelegramID == telegramID
}

// writeJSON writes a JSON response with proper headers
func (h *Handler) writeJSON(w http.ResponseWriter, status int, resp Response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

// logErr logs an error with context
func (h *Handler) logErr(msg string, err error) {
	if h.logger != nil {
		h.logger.Error(msg, zap.Error(err))
	}
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// min returns the minimum of two integers (Go 1.20 compatibility)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
