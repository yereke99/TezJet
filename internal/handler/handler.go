package handler

// handler.go
import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"tezjet/config"
	"tezjet/internal/domain"
	"tezjet/internal/repository"
)

// Response represents the API response
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type MatchedDriver struct {
	ID                  string  `json:"id"` // FIXED: Changed to string
	TelegramID          string  `json:"telegram_id"`
	FirstName           string  `json:"first_name"`
	LastName            string  `json:"last_name"`
	ContactNumber       string  `json:"contact_number"`
	TruckType           string  `json:"truck_type"`
	ProfilePhoto        string  `json:"profile_photo"`
	IsVerified          bool    `json:"is_verified"`
	RouteID             string  `json:"route_id"` // FIXED: Changed to string
	FromAddress         string  `json:"from_address"`
	ToAddress           string  `json:"to_address"`
	FromLat             float64 `json:"from_lat"`
	FromLon             float64 `json:"from_lon"`
	ToLat               float64 `json:"to_lat"`
	ToLon               float64 `json:"to_lon"`
	Price               int     `json:"price"`
	DepartureTime       string  `json:"departure_time"`
	Comment             string  `json:"comment"`
	TruckPhoto          string  `json:"truck_photo"`
	DistanceToPickupKm  float64 `json:"distance_to_pickup_km"`
	DistanceToDropoffKm float64 `json:"distance_to_dropoff_km"`
	RouteMatchScore     int     `json:"route_match_score"`
	MatchQuality        string  `json:"match_quality"`
	ETAMin              int     `json:"eta_min"`
	IsOnline            bool    `json:"is_online"`
	LastSeenMin         int     `json:"last_seen_min"`
	HasWhatsApp         bool    `json:"has_whatsapp"`
	HasTelegram         bool    `json:"has_telegram"`
	ResponseTimeMin     int     `json:"response_time_min"`
}

type Handler struct {
	logger     *zap.Logger
	cfg        *config.Config
	db         *sql.DB
	bot        *bot.Bot
	userRepo   *repository.UserRepository
	driverRepo *repository.DriverRepository
	redisRepo  *repository.RedisRepository

	chatHub *Hub
}

func NewHandler(cfg *config.Config, logger *zap.Logger, db *sql.DB, userRepo *repository.UserRepository, driverRepo *repository.DriverRepository, redisClient *redis.Client) *Handler {
	// Create directories for file uploads
	os.MkdirAll("./ava", 0755)
	os.MkdirAll("./documents", 0755)
	os.MkdirAll("./delivery-photo", 0755)

	return &Handler{
		cfg:        cfg,
		logger:     logger,
		db:         db,
		userRepo:   userRepo,
		driverRepo: driverRepo,
		redisRepo:  repository.NewRedisRepository(redisClient),
		chatHub:    NewHub(),
	}
}

// NEW: handleDriverRegister - Full implementation for driver registration
func (h *Handler) handleDriverRegister(b *bot.Bot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		h.logger.Info("Received driver registration request",
			zap.String("method", r.Method),
			zap.String("content_type", r.Header.Get("Content-Type")),
			zap.String("content_length", r.Header.Get("Content-Length")))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		r = r.WithContext(ctx)

		maxMemory := int64(10 << 20) // 10 MB in memory, rest on disk
		err := r.ParseMultipartForm(maxMemory)
		if err != nil {
			h.logger.Error("Failed to parse multipart form",
				zap.Error(err),
				zap.String("content_length", r.Header.Get("Content-Length")))

			if ctx.Err() == context.DeadlineExceeded {
				h.sendErrorResponse(w, "Время загрузки файлов истекло. Попробуйте уменьшить размер файлов.", http.StatusRequestTimeout)
			} else {
				h.sendErrorResponse(w, "Ошибка обработки файлов. Проверьте размер и формат файлов.", http.StatusBadRequest)
			}
			return
		}

		driver, err := h.parseDriverRegistration(r)
		if err != nil {
			h.logger.Error("Failed to parse driver registration", zap.Error(err))
			h.sendErrorResponse(w, err.Error(), http.StatusBadRequest)
			return
		}
		taken, err := h.isTruckNumberTaken(driver.TruckNumber)
		if err != nil {
			h.logger.Error("Failed to check truck number uniqueness", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка проверки номера машины", http.StatusInternalServerError)
			return
		}
		if taken {
			h.sendErrorResponse(w, "Этот номер машины уже зарегистрирован", http.StatusBadRequest)
			return
		}
		profilePhotoPath, err := h.saveFile(r, "profilePhoto", "./ava", driver.TelegramID, "profile")
		if err != nil {
			h.logger.Error("Failed to save profile photo", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка сохранения фото профиля", http.StatusInternalServerError)
			return
		}
		driver.ProfilePhoto = profilePhotoPath
		licenseFrontPath, err := h.saveFile(r, "licenseFront", "./documents", driver.TelegramID, "license_front")
		if err != nil {
			h.logger.Error("Failed to save license front", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка сохранения водительского удостоверения", http.StatusInternalServerError)
			return
		}
		driver.LicenseFront = licenseFrontPath

		licenseBackPath, err := h.saveFile(r, "licenseBack", "./documents", driver.TelegramID, "license_back")
		if err != nil {
			h.logger.Error("Failed to save license back", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка сохранения водительского удостоверения", http.StatusInternalServerError)
			return
		}
		driver.LicenseBack = licenseBackPath

		h.logger.Info("Parsed driver registration",
			zap.String("first_name", driver.FirstName),
			zap.String("last_name", driver.LastName),
			zap.String("contact", driver.ContactNumber),
			zap.String("city", driver.StartCity),
			zap.Float64("lat", driver.Latitude),
			zap.Float64("lon", driver.Longitude),
			zap.Int64("telegram_id", driver.TelegramID))

		// Save driver registration to database
		driverID, err := h.saveDriverRegistration(driver)
		if err != nil {
			h.logger.Error("Failed to save driver registration", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка сохранения регистрации", http.StatusInternalServerError)
			return
		}

		driver.ID = driverID
		driver.Status = "pending"
		driver.CreatedAt = time.Now()

		h.logger.Info("Driver registration saved successfully", zap.String("driver_id", driverID))

		go h.sendDriverConfirmationMessage(b, driver, driverID)

		// Send success response
		h.sendSuccessResponse(w, "Регистрация успешно отправлена", map[string]interface{}{
			"driver_id": driverID,
			"status":    "pending",
		})
	}
}

func (h *Handler) handleCheckTruckNumber() http.HandlerFunc {
	type req struct {
		TruckNumber string `json:"truck_number"`
	}
	type resp struct {
		Success   bool   `json:"success"`
		Available bool   `json:"available"`
		Message   string `json:"message,omitempty"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(resp{Success: false, Message: "Method not allowed"})
			return
		}

		var body req
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(resp{Success: false, Message: "Invalid JSON"})
			return
		}

		tn := strings.TrimSpace(body.TruckNumber)
		tn = strings.ToUpper(strings.ReplaceAll(tn, " ", ""))

		if len(tn) < 5 || len(tn) > 15 {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp{Success: true, Available: false, Message: "Неверный формат номера"})
			return
		}

		taken, err := h.isTruckNumberTaken(tn)
		if err != nil {
			h.logger.Error("truck number check failed", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(resp{Success: false, Message: "Server error"})
			return
		}

		_ = json.NewEncoder(w).Encode(resp{
			Success:   true,
			Available: !taken,
		})
	}
}

// driverHandler serves the driver trip page
func (h *Handler) driverHandler(w http.ResponseWriter, r *http.Request) {
	path := "./static/driver.html"
	w.Header().Set("Content-Type", "text/html")

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		h.logger.Error("Driver page not found", zap.String("path", path))
		http.Error(w, "Driver page not found", http.StatusNotFound)
		return
	}

	h.logger.Info("Serving driver page", zap.String("user_agent", r.Header.Get("User-Agent")))
	http.ServeFile(w, r, path)
}

// handleDriverStart handles driver trip creation
func (h *Handler) handleDriverStart(b *bot.Bot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ✅ Правильный парсинг form/multipart
		var err error
		if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
			err = r.ParseMultipartForm(32 << 20) // 32MB
		} else {
			err = r.ParseForm()
		}
		if err != nil {
			h.logger.Error("Failed to parse form", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка обработки данных", http.StatusBadRequest)
			return
		}

		// (необязательно) полезно залогировать, что реально пришло
		if r.MultipartForm != nil {
			h.logger.Info("Multipart fields", zap.Any("values", r.MultipartForm.Value))
		}
		h.logger.Info("Form fields", zap.Any("form", r.Form))

		// Parse driver trip request
		trip, err := h.parseDriverTripRequest(r)
		if err != nil {
			h.logger.Error("Failed to parse driver trip request", zap.Error(err))
			h.sendErrorResponse(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Verify driver exists and is approved
		driver, err := h.CheckDriverExist(trip.TelegramID)
		if err != nil {
			h.logger.Error("Failed to check driver existence", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка проверки водителя", http.StatusInternalServerError)
			return
		}

		if driver == nil {
			h.sendErrorResponse(w, "Водитель не найден", http.StatusNotFound)
			return
		}

		if driver.Status != "approved" {
			h.sendErrorResponse(w, "Водитель не одобрен", http.StatusForbidden)
			return
		}

		trip.DriverID = driver.ID

		// Calculate route distance and time if not provided
		if trip.DistanceKm == 0 || trip.EtaMin == 0 {
			distance, duration := h.calculateRoute(trip.FromLat, trip.FromLon, trip.ToLat, trip.ToLon)
			if trip.DistanceKm == 0 {
				trip.DistanceKm = distance
			}
			if trip.EtaMin == 0 {
				trip.EtaMin = duration
			}
		}

		h.logger.Info("Parsed driver trip request",
			zap.String("from", trip.FromAddress),
			zap.String("to", trip.ToAddress),
			zap.Int("price", trip.Price),
			zap.Float64("distance", trip.DistanceKm),
			zap.Int("eta", trip.EtaMin),
			zap.String("start_time", trip.StartTime),
			zap.Int64("telegram_id", trip.TelegramID),
			zap.String("driver_id", trip.DriverID))

		// Save driver trip to database
		tripID, err := h.saveDriverTrip(trip)
		if err != nil {
			h.logger.Error("Failed to save driver trip", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка сохранения поездки", http.StatusInternalServerError)
			return
		}

		trip.ID = tripID
		trip.Status = "active"
		trip.CreatedAt = time.Now()

		h.logger.Info("Driver trip saved successfully", zap.String("trip_id", tripID))

		// Send confirmation message to driver
		go h.sendDriverTripConfirmation(b, trip, driver)

		// Send success response
		h.sendSuccessResponse(w, "Поездка успешно создана", map[string]interface{}{
			"trip_id":  tripID,
			"status":   "active",
			"distance": trip.DistanceKm,
			"eta":      trip.EtaMin,
		})
	}
}

// parseDriverTripRequest parses the driver trip form data
func (h *Handler) parseDriverTripRequest(r *http.Request) (*DriverTrip, error) {
	trip := &DriverTrip{}

	// Helper function to get form value
	getValue := func(key string) string {
		return strings.TrimSpace(r.FormValue(key))
	}

	// Required fields
	trip.FromAddress = getValue("from_address")
	trip.ToAddress = getValue("to_address")
	trip.StartTime = getValue("start_time")

	if trip.FromAddress == "" {
		return nil, fmt.Errorf("адрес отправления обязателен")
	}
	if trip.ToAddress == "" {
		return nil, fmt.Errorf("адрес назначения обязателен")
	}
	if trip.StartTime == "" {
		return nil, fmt.Errorf("время выезда обязательно")
	}

	// Parse coordinates
	var err error
	if latStr := getValue("from_lat"); latStr != "" {
		trip.FromLat, err = strconv.ParseFloat(latStr, 64)
		if err != nil {
			return nil, fmt.Errorf("неверная широта отправления: %v", err)
		}
	}

	if lonStr := getValue("from_lon"); lonStr != "" {
		trip.FromLon, err = strconv.ParseFloat(lonStr, 64)
		if err != nil {
			return nil, fmt.Errorf("неверная долгота отправления: %v", err)
		}
	}

	if latStr := getValue("to_lat"); latStr != "" {
		trip.ToLat, err = strconv.ParseFloat(latStr, 64)
		if err != nil {
			return nil, fmt.Errorf("неверная широта назначения: %v", err)
		}
	}

	if lonStr := getValue("to_lon"); lonStr != "" {
		trip.ToLon, err = strconv.ParseFloat(lonStr, 64)
		if err != nil {
			return nil, fmt.Errorf("неверная долгота назначения: %v", err)
		}
	}

	// Parse price
	if priceStr := getValue("price"); priceStr != "" {
		trip.Price, err = strconv.Atoi(priceStr)
		if err != nil || trip.Price < 2000 {
			return nil, fmt.Errorf("цена должна быть не менее 2000 тенге")
		}
	} else {
		return nil, fmt.Errorf("цена обязательна")
	}

	// Parse Telegram ID (should be passed from frontend)
	if telegramIDStr := getValue("telegram_id"); telegramIDStr != "" {
		trip.TelegramID, _ = strconv.ParseInt(telegramIDStr, 10, 64)
	}

	if trip.TelegramID == 0 {
		return nil, fmt.Errorf("telegram ID обязателен")
	}

	// Optional fields
	trip.Comment = getValue("comment")

	// Parse distance and ETA from frontend if provided
	if distStr := getValue("distance"); distStr != "" {
		trip.DistanceKm, _ = strconv.ParseFloat(distStr, 64)
	}

	if etaStr := getValue("duration"); etaStr != "" {
		trip.EtaMin, _ = strconv.Atoi(etaStr)
	}

	return trip, nil
}

func (h *Handler) saveDriverTrip(trip *DriverTrip) (string, error) {
	tripID := uuid.New().String()

	// Ensure default values for optional fields
	truckType := trip.TruckType
	if truckType == "" {
		truckType = "any"
	}

	comment := trip.Comment
	if comment == "" {
		comment = ""
	}

	startTime := trip.StartTime
	if startTime == "" {
		startTime = time.Now().Format("2006-01-02 15:04:05")
	}

	// Ensure we have valid coordinates
	if trip.FromLat == 0 || trip.FromLon == 0 {
		h.logger.Warn("Missing FROM coordinates for driver trip",
			zap.String("from_address", trip.FromAddress))
	}

	if trip.ToLat == 0 || trip.ToLon == 0 {
		h.logger.Warn("Missing TO coordinates for driver trip",
			zap.String("to_address", trip.ToAddress))
	}

	// FIXED: Insert with ALL required columns including truck_type and status
	query := `
		INSERT INTO driver_trips (
			id, driver_id, telegram_id, 
			from_address, from_lat, from_lon, 
			to_address, to_lat, to_lon, 
			distance_km, eta_min, price, 
			truck_type, start_time, comment, 
			departure_time, status, created_at
		) VALUES (
			?, ?, ?, 
			?, ?, ?, 
			?, ?, ?, 
			?, ?, ?, 
			?, ?, ?, 
			CURRENT_TIMESTAMP, 'active', CURRENT_TIMESTAMP
		)`

	_, err := h.db.Exec(
		query,
		tripID, trip.DriverID, trip.TelegramID,
		trip.FromAddress, trip.FromLat, trip.FromLon,
		trip.ToAddress, trip.ToLat, trip.ToLon,
		trip.DistanceKm, trip.EtaMin, trip.Price,
		truckType, startTime, comment,
	)

	if err != nil {
		h.logger.Error("Failed to insert driver trip",
			zap.Error(err),
			zap.String("trip_id", tripID),
			zap.String("driver_id", trip.DriverID),
			zap.String("truck_type", truckType),
			zap.String("from_address", trip.FromAddress),
			zap.String("to_address", trip.ToAddress))
		return "", fmt.Errorf("failed to save driver trip: %w", err)
	}

	h.logger.Info("✅ Driver trip saved to database successfully",
		zap.String("trip_id", tripID),
		zap.String("driver_id", trip.DriverID),
		zap.String("truck_type", truckType),
		zap.String("from", trip.FromAddress),
		zap.String("to", trip.ToAddress),
		zap.Int("price", trip.Price),
		zap.Float64("distance_km", trip.DistanceKm))

	return tripID, nil
}

// sendDriverTripConfirmation sends confirmation message to driver
func (h *Handler) sendDriverTripConfirmation(b *bot.Bot, trip *DriverTrip, driver *DriverRegistration) {
	if trip.TelegramID == 0 {
		h.logger.Warn("No Telegram ID provided for driver trip, skipping confirmation message")
		return
	}

	// Format start time
	startTimeText := h.formatTripStartTime(trip.StartTime)

	// FIXED: Using %s for string UUID
	message := fmt.Sprintf(`🚚 <b>Жаңа сапар басталды!</b>

📋 <b>Сапар нөмірі:</b> #%s

👤 <b>Жүргізуші:</b> %s %s
📱 <b>Байланыс:</b> %s

📍 <b>Қайдан:</b> %s
🎯 <b>Қайда:</b> %s

💰 <b>Бағасы:</b> %d ₸
🛣️ <b>Қашықтық:</b> %.1f км
⏱️ <b>Болжамды уақыт:</b> %d мин
%s`,
		trip.ID, // FIXED: Using %s for string UUID
		driver.FirstName,
		driver.LastName,
		driver.ContactNumber,
		trip.FromAddress,
		trip.ToAddress,
		trip.Price,
		trip.DistanceKm,
		trip.EtaMin,
		startTimeText,
	)

	// Add comment if provided
	if trip.Comment != "" {
		message += fmt.Sprintf("\n💬 <b>Түсініктеме:</b> %s", trip.Comment)
	}

	message += "\n\n✅ Сіздің сапарыңыз белсенді режимде!"

	// Send message with trip management keyboard
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🚚 Менің сапарларым", CallbackData: "my_trips"},
				{Text: "✅ Аяқтау", CallbackData: fmt.Sprintf("complete_trip_%s", trip.ID)}, // FIXED: %s for string
			},
		},
	}

	ctx := context.Background()

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      trip.TelegramID,
		Text:        message,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		h.logger.Error("Failed to send driver trip confirmation message",
			zap.Error(err),
			zap.Int64("telegram_id", trip.TelegramID),
			zap.String("trip_id", trip.ID))
	} else {
		h.logger.Info("Driver trip confirmation message sent",
			zap.Int64("telegram_id", trip.TelegramID),
			zap.String("trip_id", trip.ID))
	}
}

// formatTripStartTime formats the trip start time
func (h *Handler) formatTripStartTime(startTime string) string {
	if startTime == "" {
		return "🕐 <b>Кету уақыты:</b> Қазір | Сейчас"
	}

	// Parse ISO format time (2006-01-02T15:04)
	t, err := time.Parse("2006-01-02T15:04", startTime)
	if err != nil {
		// Try alternative formats
		if t2, err2 := time.Parse("2006-01-02 15:04", startTime); err2 == nil {
			t = t2
		} else {
			return fmt.Sprintf("🕐 <b>Кету уақыты:</b> %s", startTime)
		}
	}

	// Format in local time
	now := time.Now()
	if t.Format("2006-01-02") == now.Format("2006-01-02") {
		return fmt.Sprintf("🕐 <b>Кету уақыты:</b> Бүгін %s | Сегодня %s",
			t.Format("15:04"), t.Format("15:04"))
	} else if t.Before(now.AddDate(0, 0, 1)) && t.After(now.AddDate(0, 0, -1)) {
		if t.After(now) {
			return fmt.Sprintf("🕐 <b>Кету уақыты:</b> Ертең %s | Завтра %s",
				t.Format("15:04"), t.Format("15:04"))
		}
	}

	return fmt.Sprintf("🕐 <b>Кету уақыты:</b> %s %s",
		t.Format("02.01.2006"), t.Format("15:04"))
}

// handleDriverTrips handles getting driver's trips
func (h *Handler) handleDriverTrips(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	h.logger.Info("Received driver trips request",
		zap.String("method", r.Method))

	// Get Telegram ID from query params or request body
	var telegramID int64

	if r.Method == "GET" {
		telegramIDStr := r.URL.Query().Get("telegram_id")
		if telegramIDStr != "" {
			if id, err := strconv.ParseInt(telegramIDStr, 10, 64); err == nil {
				telegramID = id
			}
		}
	} else if r.Method == "POST" {
		var reqData struct {
			TelegramID int64 `json:"telegram_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqData); err == nil {
			telegramID = reqData.TelegramID
		}
	}

	if telegramID == 0 {
		h.sendErrorResponse(w, "Telegram ID обязателен", http.StatusBadRequest)
		return
	}

	// Get driver trips from database
	trips, err := h.getDriverTrips(telegramID)
	if err != nil {
		h.logger.Error("Failed to get driver trips", zap.Error(err))
		h.sendErrorResponse(w, "Ошибка получения поездок", http.StatusInternalServerError)
		return
	}

	h.sendSuccessResponse(w, "Поездки получены", map[string]interface{}{
		"trips": trips,
		"count": len(trips),
	})
}

// getDriverTrips retrieves driver trips from database
func (h *Handler) getDriverTrips(telegramID int64) ([]DriverTrip, error) {
	query := `
		SELECT id, driver_id, telegram_id, from_address, from_lat, from_lon,
			   to_address, to_lat, to_lon, distance_km, eta_min, price,
			   start_time, comment, status, created_at
		FROM driver_trips 
		WHERE telegram_id = ?
		ORDER BY created_at DESC
		LIMIT 50`

	rows, err := h.db.Query(query, telegramID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trips []DriverTrip
	for rows.Next() {
		var trip DriverTrip
		err := rows.Scan(
			&trip.ID, &trip.DriverID, &trip.TelegramID,
			&trip.FromAddress, &trip.FromLat, &trip.FromLon,
			&trip.ToAddress, &trip.ToLat, &trip.ToLon,
			&trip.DistanceKm, &trip.EtaMin, &trip.Price,
			&trip.StartTime, &trip.Comment, &trip.Status, &trip.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		trips = append(trips, trip)
	}

	return trips, nil
}

func (h *Handler) isTruckNumberTaken(truckNumber string) (bool, error) {
	q := `SELECT 1 FROM drivers WHERE lower(truck_number) = lower(?) AND truck_number <> '' LIMIT 1`
	var x int
	err := h.db.QueryRow(q, truckNumber).Scan(&x)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// parseDriverRegistration parses the driver registration form data
func (h *Handler) parseDriverRegistration(r *http.Request) (*DriverRegistration, error) {
	driver := &DriverRegistration{}

	// Helper function to get form value
	getValue := func(key string) string {
		return strings.TrimSpace(r.FormValue(key))
	}

	tn := strings.TrimSpace(r.FormValue("truckNumber"))
	tn = strings.ToUpper(strings.ReplaceAll(tn, " ", ""))

	// Required fields
	driver.FirstName = getValue("firstName")
	driver.LastName = getValue("lastName")
	driver.Birthday = getValue("birthday")
	driver.ContactNumber = getValue("contactNumber")
	driver.StartCity = getValue("startCity")
	driver.TruckNumber = tn

	if len(driver.TruckNumber) < 5 || len(driver.TruckNumber) > 15 {
		return nil, fmt.Errorf("Неверный номер машины")
	}

	// Validate required fields
	if driver.FirstName == "" {
		return nil, fmt.Errorf("имя обязательно")
	}
	if driver.LastName == "" {
		return nil, fmt.Errorf("фамилия обязательна")
	}
	if driver.Birthday == "" {
		return nil, fmt.Errorf("дата рождения обязательна")
	}
	if driver.ContactNumber == "" {
		return nil, fmt.Errorf("контактный номер обязателен")
	}
	if driver.StartCity == "" {
		return nil, fmt.Errorf("город работы обязателен")
	}

	// Parse coordinates
	var err error
	if latStr := getValue("latitude"); latStr != "" {
		driver.Latitude, err = strconv.ParseFloat(latStr, 64)
		if err != nil {
			return nil, fmt.Errorf("неверная широта: %v", err)
		}
	}

	if lonStr := getValue("longitude"); lonStr != "" {
		driver.Longitude, err = strconv.ParseFloat(lonStr, 64)
		if err != nil {
			return nil, fmt.Errorf("неверная долгота: %v", err)
		}
	}

	// Parse Telegram ID
	if telegramIDStr := getValue("telegramId"); telegramIDStr != "" {
		driver.TelegramID, _ = strconv.ParseInt(telegramIDStr, 10, 64)
	}

	if driver.TelegramID == 0 {
		return nil, fmt.Errorf("telegram ID обязателен")
	}

	// UPDATED: Parse truck type
	driver.TruckType = getValue("truckType")
	if driver.TruckType == "" {
		return nil, fmt.Errorf("көлік түрі міндетті / тип транспорта обязателен")
	}

	// Validate truck type
	validTruckTypes := map[string]bool{
		"intercity": true,
		"small":     true,
		"medium":    true,
		"large":     true,
		"tow":       true,
		"any":       true,
	}
	if !validTruckTypes[driver.TruckType] {
		return nil, fmt.Errorf("неверный тип транспорта")
	}

	// Validate age
	if driver.Birthday != "" {
		birthday, err := time.Parse("2006-01-02", driver.Birthday)
		if err != nil {
			return nil, fmt.Errorf("неверный формат даты рождения")
		}

		age := time.Now().Year() - birthday.Year()
		if age < 18 || age > 80 {
			return nil, fmt.Errorf("возраст должен быть от 18 до 80 лет")
		}
	}

	// Validate files
	if _, _, err := r.FormFile("profilePhoto"); err != nil {
		return nil, fmt.Errorf("фото профиля обязательно")
	}
	if _, _, err := r.FormFile("licenseFront"); err != nil {
		return nil, fmt.Errorf("лицевая сторона водительского удостоверения обязательна")
	}
	if _, _, err := r.FormFile("licenseBack"); err != nil {
		return nil, fmt.Errorf("обратная сторона водительского удостоверения обязательна")
	}

	return driver, nil
}

// saveFile saves uploaded file to specified directory with better error handling
func (h *Handler) saveFile(r *http.Request, fieldName, dir string, telegramID int64, fileType string) (string, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		h.logger.Error("Failed to get form file",
			zap.String("field", fieldName),
			zap.Error(err))
		return "", fmt.Errorf("файл %s не найден: %v", fieldName, err)
	}
	defer file.Close()

	h.logger.Info("Processing file upload",
		zap.String("field", fieldName),
		zap.String("filename", header.Filename),
		zap.Int64("size", header.Size),
		zap.String("content_type", header.Header.Get("Content-Type")))

	// Validate file size (5MB max to prevent timeouts)
	maxSize := int64(5 * 1024 * 1024) // 5MB
	if header.Size > maxSize {
		return "", fmt.Errorf("файл %s слишком большой (максимум 5МБ, получен %dMB)",
			fieldName, header.Size/(1024*1024))
	}

	// Validate file type
	contentType := header.Header.Get("Content-Type")
	if fieldName == "profilePhoto" {
		if !strings.HasPrefix(contentType, "image/") {
			return "", fmt.Errorf("профиль фото должно быть изображением (получен %s)", contentType)
		}
	} else {
		if !strings.HasPrefix(contentType, "image/") && contentType != "application/pdf" {
			return "", fmt.Errorf("документ должен быть изображением или PDF (получен %s)", contentType)
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		h.logger.Error("Failed to create directory",
			zap.String("dir", dir),
			zap.Error(err))
		return "", fmt.Errorf("ошибка создания директории: %v", err)
	}

	// Get file extension from content type or filename
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		switch contentType {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/webp":
			ext = ".webp"
		case "application/pdf":
			ext = ".pdf"
		default:
			ext = ".jpg" // default
		}
	}

	// Generate unique filename
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("%d_%s_%d%s", telegramID, fileType, timestamp, ext)
	filePath := filepath.Join(dir, filename)

	h.logger.Info("Saving file",
		zap.String("source", header.Filename),
		zap.String("destination", filePath),
		zap.Int64("size", header.Size))

	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		h.logger.Error("Failed to create destination file",
			zap.String("path", filePath),
			zap.Error(err))
		return "", fmt.Errorf("ошибка создания файла: %v", err)
	}
	defer dst.Close()

	// Copy file content with progress tracking
	written, err := io.Copy(dst, file)
	if err != nil {
		h.logger.Error("Failed to copy file content",
			zap.String("path", filePath),
			zap.Error(err))
		os.Remove(filePath)
		return "", fmt.Errorf("ошибка записи файла: %v", err)
	}

	h.logger.Info("File saved successfully",
		zap.String("field", fieldName),
		zap.String("filename", filename),
		zap.String("path", filePath),
		zap.Int64("size_expected", header.Size),
		zap.Int64("size_written", written))

	// Verify file was written completely
	if written != header.Size {
		h.logger.Warn("File size mismatch",
			zap.Int64("expected", header.Size),
			zap.Int64("written", written))
		// Don't fail, but log the warning
	}

	return filename, nil
}

// saveDriverRegistration saves driver registration to database
// UPDATED: saveDriverRegistration saves driver registration to database with truck_type
func (h *Handler) saveDriverRegistration(driver *DriverRegistration) (string, error) {
	driverID := uuid.New().String()

	query := `
	INSERT INTO drivers (
		id, telegram_id, first_name, last_name, birthday, contact_number,
		start_city, latitude, longitude, profile_photo, license_front,
		license_back, truck_number, truck_type, status, created_at
	) VALUES (
		?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', CURRENT_TIMESTAMP
	)`

	_, err := h.db.Exec(
		query,
		driverID, driver.TelegramID, driver.FirstName, driver.LastName, driver.Birthday,
		driver.ContactNumber, driver.StartCity, driver.Latitude, driver.Longitude,
		driver.ProfilePhoto, driver.LicenseFront, driver.LicenseBack,
		driver.TruckNumber, driver.TruckType,
	)

	if err != nil {
		return "", err
	}

	return driverID, nil
}

// sendDriverConfirmationMessage sends confirmation message to driver
func (h *Handler) sendDriverConfirmationMessage(b *bot.Bot, driver *DriverRegistration, driverID string) {
	if driver.TelegramID == 0 {
		h.logger.Warn("No Telegram ID provided for driver, skipping confirmation message")
		return
	}

	// Truck type names in Kazakh and Russian
	truckTypeNames := map[string]string{
		"intercity": "Қала аралық / Межгород",
		"small":     "Кіші (1.5т дейін) / Малый (до 1.5т)",
		"medium":    "Орташа (5т дейін) / Средний (до 5т)",
		"large":     "Үлкен (20т дейін) / Большой (до 20т)",
		"tow":       "Эвакуатор / Эвакуатор",
		"any":       "Кез келген / Любой",
	}

	truckTypeName := truckTypeNames[driver.TruckType]
	if truckTypeName == "" {
		truckTypeName = driver.TruckType
	}

	message := fmt.Sprintf(`🚗 <b>Жүргізуші ретінде тіркелу!</b>

📋 <b>Өтінім нөмірі:</b> #%s

👤 <b>Аты-жөні:</b> %s %s
📱 <b>Байланыс:</b> %s
🏙️ <b>Жұмыс қаласы:</b> %s
🎂 <b>Туған күні:</b> %s
🚚 <b>Көлік түрі:</b> %s

✅ Сіздің өтініміңіз қабылданды!
⏳ Құжаттарды тексеру 24 сағат ішінде аяқталады.

📞 Сұрақтар болса хабарласыңыз: @support`,
		driverID,
		driver.FirstName,
		driver.LastName,
		driver.ContactNumber,
		driver.StartCity,
		driver.Birthday,
		truckTypeName,
	)

	ctx := context.Background()

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    driver.TelegramID,
		Text:      message,
		ParseMode: models.ParseModeHTML,
	})

	if err != nil {
		h.logger.Error("Failed to send driver confirmation message",
			zap.Error(err),
			zap.Int64("telegram_id", driver.TelegramID),
			zap.String("driver_id", driverID))
	} else {
		h.logger.Info("Driver confirmation message sent",
			zap.Int64("telegram_id", driver.TelegramID),
			zap.String("driver_id", driverID))
	}
}

func (h *Handler) CheckDriverExist(telegramID int64) (*DriverRegistration, error) {
	query := `
        SELECT 
			id,
			telegram_id,
			first_name,
			last_name,
			birthday,
			contact_number,
			start_city,
			latitude,
			longitude,
			profile_photo,
			license_front,
			license_back,
			truck_type,
			truck_number,
			is_verified,
			status,
			created_at
        FROM drivers
        WHERE telegram_id = ?`

	var d DriverRegistration
	err := h.db.QueryRow(query, telegramID).Scan(
		&d.ID,
		&d.TelegramID,
		&d.FirstName,
		&d.LastName,
		&d.Birthday,
		&d.ContactNumber,
		&d.StartCity,
		&d.Latitude,
		&d.Longitude,
		&d.ProfilePhoto,
		&d.LicenseFront,
		&d.LicenseBack,
		&d.TruckType,
		&d.TruckNumber,
		&d.IsVerified,
		&d.Status,
		&d.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

// handleCheckWho handles the /api/check/who endpoint
func (h *Handler) handleCheckWho(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	h.logger.Info("Received check who request",
		zap.String("method", r.Method),
		zap.String("user_agent", r.Header.Get("User-Agent")))

	// Parse request body for Telegram ID
	var reqData struct {
		TelegramID int64 `json:"telegram_id"`
	}

	if r.Method == "POST" {
		if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
			h.logger.Error("Failed to parse request body", zap.Error(err))
			h.sendErrorResponse(w, "Неверные данные запроса", http.StatusBadRequest)
			return
		}
	} else {
		// Try to get from query parameters
		telegramIDStr := r.URL.Query().Get("telegram_id")
		if telegramIDStr != "" {
			if id, err := strconv.ParseInt(telegramIDStr, 10, 64); err == nil {
				reqData.TelegramID = id
			}
		}
	}

	if reqData.TelegramID == 0 {
		h.sendErrorResponse(w, "Telegram ID обязателен", http.StatusBadRequest)
		return
	}

	h.logger.Info("Checking user existence", zap.Int64("telegram_id", reqData.TelegramID))

	// Check if driver exists
	driver, err := h.CheckDriverExist(reqData.TelegramID)
	if err != nil {
		h.logger.Error("Failed to check driver existence", zap.Error(err))
		h.sendErrorResponse(w, "Ошибка проверки водителя", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"telegram_id": reqData.TelegramID,
		"exists":      driver != nil,
		"user_type":   "client", // Default to client
	}

	if driver != nil {
		response["user_type"] = "driver"
		response["driver_data"] = map[string]interface{}{
			"id":             driver.ID,
			"first_name":     driver.FirstName,
			"last_name":      driver.LastName,
			"birthday":       driver.Birthday,
			"contact_number": driver.ContactNumber,
			"start_city":     driver.StartCity,
			"latitude":       driver.Latitude,
			"longitude":      driver.Longitude,
			"truck_type":     driver.TruckType,
			"truck_number":   driver.TruckNumber,
			"profile_photo":  driver.ProfilePhoto,
			"license_front":  driver.LicenseFront,
			"license_back":   driver.LicenseBack,
			"status":         driver.Status,
			"created_at":     driver.CreatedAt,
		}

		h.logger.Info("Driver found",
			zap.String("driver_id", driver.ID),
			zap.String("status", driver.Status),
			zap.String("name", driver.FirstName+" "+driver.LastName))
	} else {
		h.logger.Info("Driver not found, treating as client", zap.Int64("telegram_id", reqData.TelegramID))
	}

	h.sendSuccessResponse(w, "Проверка выполнена", response)
}

// handleDriverUpdate handles driver profile updates
func (h *Handler) handleDriverUpdate(b *bot.Bot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		h.logger.Info("Received driver update request",
			zap.String("method", r.Method),
			zap.String("content_type", r.Header.Get("Content-Type")))

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		r = r.WithContext(ctx)

		maxMemory := int64(10 << 20) // 10 MB
		err := r.ParseMultipartForm(maxMemory)
		if err != nil {
			h.logger.Error("Failed to parse multipart form", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка обработки файлов", http.StatusBadRequest)
			return
		}

		// Get Telegram ID
		telegramIDStr := strings.TrimSpace(r.FormValue("telegramId"))
		telegramID, err := strconv.ParseInt(telegramIDStr, 10, 64)
		if err != nil || telegramID == 0 {
			h.sendErrorResponse(w, "Неверный Telegram ID", http.StatusBadRequest)
			return
		}

		// Check if driver exists
		existingDriver, err := h.CheckDriverExist(telegramID)
		if err != nil {
			h.logger.Error("Failed to check driver existence", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка проверки водителя", http.StatusInternalServerError)
			return
		}

		if existingDriver == nil {
			h.sendErrorResponse(w, "Водитель не найден", http.StatusNotFound)
			return
		}

		h.logger.Info("Updating driver profile",
			zap.String("driver_id", existingDriver.ID),
			zap.String("name", existingDriver.FirstName+" "+existingDriver.LastName))

		// Parse update data
		updateData := h.parseDriverUpdateData(r, existingDriver)

		// Handle file uploads (only if new files are provided)
		if _, _, err := r.FormFile("profilePhoto"); err == nil {
			photoPath, err := h.saveFile(r, "profilePhoto", "./ava", telegramID, "profile")
			if err != nil {
				h.logger.Error("Failed to save profile photo", zap.Error(err))
				h.sendErrorResponse(w, "Ошибка сохранения фото профиля", http.StatusInternalServerError)
				return
			}
			updateData.ProfilePhoto = photoPath
		}

		if _, _, err := r.FormFile("licenseFront"); err == nil {
			frontPath, err := h.saveFile(r, "licenseFront", "./documents", telegramID, "license_front")
			if err != nil {
				h.logger.Error("Failed to save license front", zap.Error(err))
				h.sendErrorResponse(w, "Ошибка сохранения водительского удостоверения", http.StatusInternalServerError)
				return
			}
			updateData.LicenseFront = frontPath
		}

		if _, _, err := r.FormFile("licenseBack"); err == nil {
			backPath, err := h.saveFile(r, "licenseBack", "./documents", telegramID, "license_back")
			if err != nil {
				h.logger.Error("Failed to save license back", zap.Error(err))
				h.sendErrorResponse(w, "Ошибка сохранения водительского удостоверения", http.StatusInternalServerError)
				return
			}
			updateData.LicenseBack = backPath
		}

		// Update driver in database
		err = h.updateDriverInDatabase(updateData)
		if err != nil {
			h.logger.Error("Failed to update driver", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка обновления данных", http.StatusInternalServerError)
			return
		}

		h.logger.Info("Driver profile updated successfully", zap.String("driver_id", updateData.ID))

		// Send notification
		go h.sendDriverUpdateNotification(b, updateData)

		h.sendSuccessResponse(w, "Данные успешно обновлены", map[string]interface{}{
			"driver_id": updateData.ID,
			"status":    "updated",
		})
	}
}

func parseFloatForm(r *http.Request, key string, fallback float64) float64 {
	raw := strings.TrimSpace(r.FormValue(key))
	if raw == "" {
		return fallback
	}

	// если вдруг пришло "43,2381" -> "43.2381"
	raw = strings.ReplaceAll(raw, ",", ".")

	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return v
}

// parseDriverUpdateData parses update form data
// parseDriverUpdateData parses update data from request and merges with existing driver
func (h *Handler) parseDriverUpdateData(r *http.Request, existing *DriverRegistration) *DriverRegistration {
	first := strings.TrimSpace(r.FormValue("firstName"))
	last := strings.TrimSpace(r.FormValue("lastName"))
	phone := strings.TrimSpace(r.FormValue("contactNumber"))
	city := strings.TrimSpace(r.FormValue("startCity"))
	truckType := strings.TrimSpace(r.FormValue("truckType"))

	truckNumber := strings.TrimSpace(r.FormValue("truckNumber"))
	truckNumber = strings.ToUpper(strings.ReplaceAll(truckNumber, " ", ""))

	lat := parseFloatForm(r, "latitude", existing.Latitude)
	lon := parseFloatForm(r, "longitude", existing.Longitude)

	d := &DriverRegistration{
		ID:            existing.ID,
		TelegramID:    existing.TelegramID,
		FirstName:     first,
		LastName:      last,
		Birthday:      existing.Birthday, // не меняем
		ContactNumber: phone,
		StartCity:     city,
		Latitude:      lat,
		Longitude:     lon,
		ProfilePhoto:  existing.ProfilePhoto,
		LicenseFront:  existing.LicenseFront,
		LicenseBack:   existing.LicenseBack,
		TruckType:     truckType,
		TruckNumber:   truckNumber,
		IsVerified:    existing.IsVerified,
		Status:        existing.Status,
		CreatedAt:     existing.CreatedAt,
	}

	// если пусто — оставляем старое значение
	if d.FirstName == "" {
		d.FirstName = existing.FirstName
	}
	if d.LastName == "" {
		d.LastName = existing.LastName
	}
	if d.ContactNumber == "" {
		d.ContactNumber = existing.ContactNumber
	}
	if d.StartCity == "" {
		d.StartCity = existing.StartCity
	}
	if d.TruckType == "" {
		d.TruckType = existing.TruckType
	}
	if d.TruckNumber == "" {
		d.TruckNumber = existing.TruckNumber
	}

	// lat/lon уже с fallback — сравнения со строками НЕ нужны ✅
	return d
}

// updateDriverInDatabase updates driver data in database
func (h *Handler) updateDriverInDatabase(driver *DriverRegistration) error {
	query := `
		UPDATE drivers SET
			first_name = ?, last_name = ?, contact_number = ?,
			start_city = ?, latitude = ?, longitude = ?,
			truck_type = ?, truck_number = ?,
			profile_photo = ?, license_front = ?, license_back = ?
		WHERE id = ?`

	_, err := h.db.Exec(
		query,
		driver.FirstName, driver.LastName, driver.ContactNumber,
		driver.StartCity, driver.Latitude, driver.Longitude,
		driver.TruckType, driver.TruckNumber,
		driver.ProfilePhoto, driver.LicenseFront, driver.LicenseBack,
		driver.ID,
	)
	return err
}

// sendDriverUpdateNotification sends update notification
func (h *Handler) sendDriverUpdateNotification(b *bot.Bot, driver *DriverRegistration) {
	if driver.TelegramID == 0 {
		return
	}

	message := fmt.Sprintf(`🔄 <b>Профиль жаңартылды!</b>

👤 <b>Аты-жөні:</b> %s %s
📱 <b>Байланыс:</b> %s
🏙️ <b>Жұмыс қаласы:</b> %s

✅ Сіздің деректеріңіз сәтті жаңартылды!
📋 Статус: %s

🚗 Тапсырыстар алуды жалғастыра аласыз.`,
		driver.FirstName,
		driver.LastName,
		driver.ContactNumber,
		driver.StartCity,
		driver.Status,
	)

	ctx := context.Background()
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    driver.TelegramID,
		Text:      message,
		ParseMode: models.ParseModeHTML,
	})

	if err != nil {
		h.logger.Error("Failed to send driver update notification",
			zap.Error(err),
			zap.Int64("telegram_id", driver.TelegramID))
	} else {
		h.logger.Info("Driver update notification sent",
			zap.Int64("telegram_id", driver.TelegramID))
	}
}

// driverUpdateHandler serves the driver update page
func (h *Handler) driverUpdateHandler(w http.ResponseWriter, r *http.Request) {
	path := "./static/driver-update.html"
	w.Header().Set("Content-Type", "text/html")
	http.ServeFile(w, r, path)
}

// Additional handler methods for routes, CORS, etc. (keeping existing functionality)
func (h *Handler) deliveryHandler(w http.ResponseWriter, r *http.Request) {
	path := "./static/delivery-opm.html"
	w.Header().Set("Content-Type", "text/html")
	http.ServeFile(w, r, path)
}

func (h *Handler) registerDriverHandler(w http.ResponseWriter, r *http.Request) {
	path := "./static/driver-register.html"
	w.Header().Set("Content-Type", "text/html")
	http.ServeFile(w, r, path)
}

func (h *Handler) mainClientHandler(w http.ResponseWriter, r *http.Request) {
	path := "./static/main-client.html"
	w.Header().Set("Content-Type", "text/html")
	http.ServeFile(w, r, path)
}

func (h *Handler) liveHandler(w http.ResponseWriter, r *http.Request) {
	path := "./static/live.html"
	w.Header().Set("Content-Type", "text/html")
	http.ServeFile(w, r, path)
}

// Middleware
func (h *Handler) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Telegram-Init-Data")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// welcomeHandler serves the welcome page
func (h *Handler) welcomeHandler(w http.ResponseWriter, r *http.Request) {
	path := "./static/welcome.html"
	w.Header().Set("Content-Type", "text/html")
	h.logger.Info("Serving welcome page", zap.String("user_agent", r.Header.Get("User-Agent")))
	http.ServeFile(w, r, path)
}

func (h *Handler) landingHandler(w http.ResponseWriter, r *http.Request) {
	path := "./static/landing.html"
	w.Header().Set("Content-Type", "text/html")
	h.logger.Info("Serving welcome page", zap.String("user_agent", r.Header.Get("User-Agent")))
	http.ServeFile(w, r, path)
}

// adminHandler serves the admin panel page (only for admin telegram_id)
func (h *Handler) adminHandler(w http.ResponseWriter, r *http.Request) {
	path := "./static/admin-panel.html"
	w.Header().Set("Content-Type", "text/html")

	// Просто отдаём HTML — проверка прав происходит на фронте через Telegram WebApp.id
	if _, err := os.Stat(path); os.IsNotExist(err) {
		h.logger.Error("Admin panel page not found", zap.String("path", path))
		http.Error(w, "Admin panel page not found", http.StatusNotFound)
		return
	}

	h.logger.Info("Serving admin panel page",
		zap.String("user_agent", r.Header.Get("User-Agent")))
	http.ServeFile(w, r, path)
}

// userHistoryHandler serves the user history page
func (h *Handler) userHistoryHandler(w http.ResponseWriter, r *http.Request) {
	path := "./static/user-history.html"
	w.Header().Set("Content-Type", "text/html")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		h.logger.Error("User history page not found", zap.String("path", path))
		http.Error(w, "User history page not found", http.StatusNotFound)
		return
	}
	h.logger.Info("Serving user history page", zap.String("user_agent", r.Header.Get("User-Agent")))
	http.ServeFile(w, r, path)
}

func (h *Handler) SetBot(b *bot.Bot) {
	h.bot = b
}

// Updated StartWebServer function with welcome page as default
func (h *Handler) StartWebServer(ctx context.Context, b *bot.Bot) {
	go h.ChangeDriverStatus(ctx, b)

	r := mux.NewRouter()
	h.SetBot(b)

	r.Use(h.corsMiddleware)

	// Serve static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	// Serve uploaded files
	r.PathPrefix("/ava/").Handler(http.StripPrefix("/ava/", http.FileServer(http.Dir("./ava/"))))
	r.PathPrefix("/documents/").Handler(http.StripPrefix("/documents/", http.FileServer(http.Dir("./documents/"))))
	r.PathPrefix("/files/").Handler(http.StripPrefix("/files/", http.FileServer(http.Dir("./files/"))))
	r.PathPrefix("/delivery-photo/").Handler(http.StripPrefix("/delivery-photo/", http.FileServer(http.Dir("./delivery-photo/"))))

	// Main pages
	r.HandleFunc("/", h.landingHandler).Methods("GET") // NEW - Welcome as default
	r.HandleFunc("/welcome", h.welcomeHandler)
	r.HandleFunc("/delivery", h.deliveryHandler).Methods("GET")
	r.HandleFunc("/register", h.registerDriverHandler).Methods("GET")
	r.HandleFunc("/driver-update", h.driverUpdateHandler).Methods("GET")
	r.HandleFunc("/admin", h.adminHandler).Methods("GET")
	r.HandleFunc("/delivery-list", h.deliveryListHandler).Methods("GET")
	r.HandleFunc("/main-client", h.mainClientHandler).Methods("GET")
	r.HandleFunc("/user-history", h.userHistoryHandler).Methods("GET")

	r.HandleFunc("/live", h.liveHandler).Methods("GET")
	r.HandleFunc("/ws/live-chat", h.LiveChatWS)

	r.HandleFunc("/driver", h.driverHandler).Methods("GET")
	r.HandleFunc("/api/driver/start", h.handleDriverStart(b)).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/driver/trips", h.handleDriverTrips).Methods("GET", "POST", "OPTIONS")

	// API routes
	r.HandleFunc("/api/delivery-request", h.HandleDelivery(ctx, b)).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/driver/register", h.handleDriverRegister(b)).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/driver/update", h.handleDriverUpdate(b)).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/driver/truck-number/check", h.handleCheckTruckNumber())
	r.HandleFunc("/api/check/who", h.handleCheckWho).Methods("GET", "POST", "OPTIONS")

	// ADMIN API
	r.HandleFunc("/api/admin/summary", h.handleAdminSummary).Methods("GET", "POST", "OPTIONS")
	r.HandleFunc("/api/admin/drivers", h.handleAdminDrivers).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/admin/drivers/{id}", h.handleAdminDriverDetail).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/admin/orders", h.handleAdminOrders).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/admin/drivers/{id}/reject", h.RejectDriver).Methods(http.MethodPost)

	// Delivery list routes
	r.HandleFunc("/api/delivery-list", h.handleDeliveryList).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/driver/accept-order", h.handleDriverAcceptOrder(b)).Methods("POST", "OPTIONS")

	// Driver matching routes
	r.HandleFunc("/driver-list", h.handleDriverList).Methods("GET")
	r.HandleFunc("/api/user/history", h.handleUserHistory).Methods("GET", "POST", "OPTIONS")
	r.HandleFunc("/api/driver-list", h.HandleDriverListAPI).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/driver-request", h.handleDriverRequest).Methods("POST", "OPTIONS")
	// Add this line after the user history route
	r.HandleFunc("/api/user/cancel-order", h.handleUserCancelOrder).Methods("POST", "OPTIONS")

	// Health check
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "ok",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}).Methods("GET")

	// Fix port configuration issue
	port := h.cfg.Port
	if !strings.Contains(port, ":") {
		port = ":" + port
	}

	server := &http.Server{
		Addr:           port,
		Handler:        r,
		ReadTimeout:    2 * time.Minute,
		WriteTimeout:   2 * time.Minute,
		IdleTimeout:    5 * time.Minute,
		MaxHeaderBytes: 10 << 20,
	}

	h.logger.Info("Starting web server with welcome page", zap.String("port", port))
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.logger.Fatal("Failed to start web server", zap.Error(err))
		}
	}()

	<-ctx.Done()
	h.logger.Info("Shutting down web server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		h.logger.Error("Server shutdown error", zap.Error(err))
	}
}

// getUserOrders returns last N delivery requests for given client telegram_id
func (h *Handler) getUserOrders(telegramID int64) ([]domain.DeliveryRequest, error) {
	h.logger.Info("Loading user orders",
		zap.Int64("telegram_id", telegramID))

	const query = `
SELECT 
  id,
  telegram_id,
  from_address,
  from_lat,
  from_lon,
  to_address,
  to_lat,
  to_lon,
  distance_km,
  eta_min,
  price,
  COALESCE(truck_type, ''),
  COALESCE(contact, ''),
  COALESCE(time_start, ''),
  COALESCE(comment, ''),
  COALESCE(item_photo_path, ''),
  status,
  created_at
FROM delivery_requests
WHERE telegram_id = ?
ORDER BY created_at DESC
LIMIT 100;
`

	rows, err := h.db.Query(query, telegramID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []domain.DeliveryRequest
	for rows.Next() {
		var o domain.DeliveryRequest
		if err := rows.Scan(
			&o.ID, &o.TelegramID,
			&o.FromAddress, &o.FromLat, &o.FromLon,
			&o.ToAddress, &o.ToLat, &o.ToLon,
			&o.DistanceKm, &o.EtaMin, &o.Price,
			&o.TruckType, &o.Contact, &o.TimeStart,
			&o.Comment, &o.CargoPhoto,
			&o.Status, &o.CreatedAt,
		); err != nil {
			h.logger.Error("Failed to scan user order", zap.Error(err))
			continue
		}

		// привести путь к имени файла (как в getDeliveryOrdersInRadius)
		if p := strings.TrimSpace(o.CargoPhoto); p != "" {
			o.CargoPhoto = filepath.Base(p)
		}

		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	h.logger.Info("User orders loaded",
		zap.Int64("telegram_id", telegramID),
		zap.Int("count", len(orders)))

	return orders, nil
}

// handleUserHistory returns delivery history for a client by telegram_id
func (h *Handler) handleUserHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	h.logger.Info("Received user history request",
		zap.String("method", r.Method))

	var telegramID int64

	if r.Method == http.MethodGet {
		if s := r.URL.Query().Get("telegram_id"); s != "" {
			if id, err := strconv.ParseInt(s, 10, 64); err == nil {
				telegramID = id
			}
		}
	} else { // POST / OPTIONS
		var req struct {
			TelegramID int64 `json:"telegram_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			telegramID = req.TelegramID
		}
	}

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if telegramID == 0 {
		h.sendErrorResponse(w, "Telegram ID обязателен", http.StatusBadRequest)
		return
	}

	orders, err := h.getUserOrders(telegramID)
	if err != nil {
		h.logger.Error("Failed to get user orders", zap.Error(err))
		h.sendErrorResponse(w, "Ошибка получения истории заказов", http.StatusInternalServerError)
		return
	}

	h.sendSuccessResponse(w, "История заказов получена", map[string]interface{}{
		"orders": orders,
		"count":  len(orders),
	})
}

// IMPROVED: handleDeliveryList with better debugging and fallback location
func (h *Handler) handleDeliveryList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	h.logger.Info("Received delivery list request",
		zap.String("method", r.Method),
		zap.String("user_agent", r.Header.Get("User-Agent")))

	// Parse request body
	var reqData DeliveryListRequest
	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
		h.logger.Error("Failed to parse request body", zap.Error(err))
		h.sendErrorResponse(w, "Неверные данные запроса", http.StatusBadRequest)
		return
	}

	h.logger.Info("Parsed delivery list request",
		zap.Int64("telegram_id", reqData.TelegramID),
		zap.Float64("driver_lat", reqData.DriverLat),
		zap.Float64("driver_lon", reqData.DriverLon),
		zap.Float64("radius", reqData.Radius))

	// RELAXED: Driver validation - allow requests even if driver not fully validated
	if reqData.TelegramID != 0 {
		driver, err := h.CheckDriverExist(reqData.TelegramID)
		if err != nil {
			h.logger.Error("Failed to check driver existence", zap.Error(err))
			// Don't fail - continue with request
		}

		if driver != nil && driver.Status != "approved" {
			h.logger.Warn("Driver not approved but allowing request",
				zap.Int64("telegram_id", reqData.TelegramID),
				zap.String("status", driver.Status))
			// Don't fail - just log warning
		}
	}

	// FALLBACK: Use Almaty center if no driver location provided
	if reqData.DriverLat == 0 || reqData.DriverLon == 0 {
		reqData.DriverLat = 43.238949 // Almaty center
		reqData.DriverLon = 76.889709
		h.logger.Info("Using default Almaty coordinates",
			zap.Float64("lat", reqData.DriverLat),
			zap.Float64("lon", reqData.DriverLon))
	}

	// Default/expanded radius
	if reqData.Radius == 0 {
		reqData.Radius = 50 // Increased default radius
	}

	// Get delivery orders within radius
	orders, err := h.getDeliveryOrdersInRadius(r.Context(), reqData.DriverLat, reqData.DriverLon, reqData.Radius)
	if err != nil {
		h.logger.Error("Failed to get delivery orders", zap.Error(err))
		h.sendErrorResponse(w, "Ошибка получения заказов", http.StatusInternalServerError)
		return
	}

	// Calculate statistics with more detailed logging
	nearbyCount := 0
	totalPrice := 0.0
	for i, order := range orders {
		distance := h.haversineDistance(reqData.DriverLat, reqData.DriverLon, order.FromLat, order.FromLon)
		if distance <= 5.0 {
			nearbyCount++
		}
		totalPrice += float64(order.Price)

		// Log first few orders for debugging
		if i < 3 {
			h.logger.Debug("Order details",
				zap.String("order_id", order.ID),
				zap.String("from", order.FromAddress),
				zap.String("to", order.ToAddress),
				zap.Int("price", order.Price),
				zap.Float64("distance_km", distance),
				zap.String("status", order.Status))
		}
	}

	avgPrice := 0.0
	if len(orders) > 0 {
		avgPrice = totalPrice / float64(len(orders))
	}

	response := DeliveryListResponse{
		Orders:      orders,
		TotalCount:  len(orders),
		NearbyCount: nearbyCount,
		AvgPrice:    avgPrice,
	}

	h.logger.Info("Delivery orders response prepared",
		zap.Int("total_orders", len(orders)),
		zap.Int("nearby_orders", nearbyCount),
		zap.Float64("avg_price", avgPrice))

	h.sendSuccessResponse(w, "Заказы получены", response)
}

// helper: привести путь фото к публичному URL
func publicPhotoURL(baseURL, p string) string {
	if p == "" {
		return ""
	}
	// если уже абсолютный http(s) — не трогаем
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}
	// если в БД только имя файла — дополним префиксом
	if !strings.Contains(p, "/") {
		p = "/delivery-photo/" + p
	} else {
		// если в БД 'delivery-photo/...' — добавим ведущий слеш
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
	}
	// если есть базовый домен — склеим абсолютный URL (опционально)
	if baseURL := strings.TrimRight(baseURL, "/"); baseURL != "" && !strings.HasPrefix(p, "http") {
		return baseURL + p
	}
	return p
}

// FIXED: getDeliveryOrdersInRadius - Better filtering and debugging
func (h *Handler) getDeliveryOrdersInRadius(ctx context.Context, driverLat, driverLon, radiusKm float64) ([]domain.DeliveryRequest, error) {
	h.logger.Info("Getting delivery orders in radius",
		zap.Float64("driver_lat", driverLat),
		zap.Float64("driver_lon", driverLon),
		zap.Float64("radius_km", radiusKm))

	// Get all pending orders first (with more relaxed time filter)
	query := `
SELECT 
  id,
  telegram_id,
  from_address,
  from_lat,
  from_lon,
  to_address,
  to_lat,
  to_lon,
  distance_km,
  eta_min,
  price,
  COALESCE(truck_type, ''),   -- может быть NULL
  COALESCE(contact, ''),      -- может быть NULL
  COALESCE(time_start, ''),   -- может быть NULL (если TEXT)
  COALESCE(comment, ''),      -- может быть NULL
  COALESCE(item_photo_path, ''), -- <-- ключевая правка
  status,
  created_at
FROM delivery_requests
WHERE status = 'pending'
  AND created_at >= datetime('now', '-72 hours')
ORDER BY created_at DESC
LIMIT 200;
`
	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		h.logger.Error("Database query failed", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var allOrders []domain.DeliveryRequest
	ordersProcessed := 0
	ordersInRadius := 0

	for rows.Next() {
		var order domain.DeliveryRequest
		err := rows.Scan(
			&order.ID, &order.TelegramID, &order.FromAddress, &order.FromLat, &order.FromLon,
			&order.ToAddress, &order.ToLat, &order.ToLon, &order.DistanceKm, &order.EtaMin,
			&order.Price, &order.TruckType, &order.Contact, &order.TimeStart, &order.Comment,
			&order.CargoPhoto, &order.Status, &order.CreatedAt,
		)
		if err != nil {
			h.logger.Error("Error scanning delivery order", zap.Error(err))
			continue
		}

		ordersProcessed++

		if p := strings.TrimSpace(order.CargoPhoto); p != "" {
			order.CargoPhoto = filepath.Base(p)
		}

		// FIXED: More lenient distance calculation and fallback for missing coordinates
		var distance float64
		if order.FromLat != 0 && order.FromLon != 0 && driverLat != 0 && driverLon != 0 {
			distance = h.haversineDistance(driverLat, driverLon, order.FromLat, order.FromLon)
		} else {
			// If coordinates are missing, assume it's within radius (fallback)
			distance = radiusKm / 2
			h.logger.Warn("Missing coordinates for order",
				zap.String("order_id", order.ID),
				zap.Float64("order_lat", order.FromLat),
				zap.Float64("order_lon", order.FromLon))
		}

		h.logger.Debug("Processing order",
			zap.String("order_id", order.ID),
			zap.Float64("distance", distance),
			zap.Float64("radius", radiusKm),
			zap.String("from_address", order.FromAddress))

		// EXPANDED: Include orders within expanded radius OR if they're in Almaty area
		if distance <= radiusKm || h.isInAlmatyArea(order.FromLat, order.FromLon) {
			ordersInRadius++
			allOrders = append(allOrders, order)
			h.logger.Debug("Order included",
				zap.String("order_id", order.ID),
				zap.Float64("distance", distance))
		}
	}

	h.logger.Info("Orders filtering completed",
		zap.Int("total_processed", ordersProcessed),
		zap.Int("orders_in_radius", ordersInRadius),
		zap.Float64("radius_km", radiusKm))

	// If no orders found with current radius, try with expanded radius
	if len(allOrders) == 0 && radiusKm < 50 {
		h.logger.Info("No orders found, expanding search radius")
		return h.getDeliveryOrdersInRadius(ctx, driverLat, driverLon, 50.0)
	}

	return allOrders, nil
}

// NEW: Check if coordinates are in Almaty area (fallback for missing driver location)
func (h *Handler) isInAlmatyArea(lat, lon float64) bool {
	// Almaty city bounds (approximate)
	almatyMinLat := 43.15
	almatyMaxLat := 43.35
	almatyMinLon := 76.75
	almatyMaxLon := 77.05

	return lat >= almatyMinLat && lat <= almatyMaxLat &&
		lon >= almatyMinLon && lon <= almatyMaxLon
}

// deliveryListHandler serves the delivery list page
func (h *Handler) deliveryListHandler(w http.ResponseWriter, r *http.Request) {
	path := "./static/delivery-list.html"
	w.Header().Set("Content-Type", "text/html")
	http.ServeFile(w, r, path)
}

// FIXED: handleDriverAcceptOrder handles driver accepting an order
func (h *Handler) handleDriverAcceptOrder(b *bot.Bot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		h.logger.Info("Received driver accept order request",
			zap.String("method", r.Method))

		// FIXED: Parse request body with proper types
		var reqData struct {
			TelegramID int64  `json:"telegram_id"`
			OrderID    string `json:"order_id"` // FIXED: Changed to string for UUID support
		}

		if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
			h.logger.Error("Failed to parse request body", zap.Error(err))
			h.sendErrorResponse(w, "Неверные данные запроса", http.StatusBadRequest)
			return
		}

		if reqData.TelegramID == 0 || reqData.OrderID == "" { // FIXED: Check for empty string
			h.sendErrorResponse(w, "Telegram ID и Order ID обязательны", http.StatusBadRequest)
			return
		}

		// Verify driver exists and is approved
		driver, err := h.CheckDriverExist(reqData.TelegramID)
		if err != nil {
			h.logger.Error("Failed to check driver existence", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка проверки водителя", http.StatusInternalServerError)
			return
		}

		if driver == nil {
			h.sendErrorResponse(w, "Водитель не найден", http.StatusNotFound)
			return
		}

		if driver.Status != "approved" {
			h.sendErrorResponse(w, "Водитель не одобрен", http.StatusForbidden)
			return
		}

		// Get order details
		order, err := h.getDeliveryOrderById(reqData.OrderID)
		if err != nil {
			h.logger.Error("Failed to get order", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка получения заказа", http.StatusInternalServerError)
			return
		}

		if order == nil {
			h.sendErrorResponse(w, "Заказ не найден", http.StatusNotFound)
			return
		}

		if order.Status != "pending" {
			h.sendErrorResponse(w, "Заказ уже принят другим водителем", http.StatusConflict)
			return
		}

		// Update order status to accepted
		err = h.updateOrderStatus(reqData.OrderID, "pending", driver.ID)
		if err != nil {
			h.logger.Error("Failed to update order status", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка принятия заказа", http.StatusInternalServerError)
			return
		}

		h.logger.Info("Order accepted by driver",
			zap.String("order_id", reqData.OrderID), // FIXED: String logging
			zap.String("driver_id", driver.ID),
			zap.String("driver_name", driver.FirstName+" "+driver.LastName))

		// Send notifications
		go h.sendOrderAcceptedNotifications(b, order, driver)

		h.sendSuccessResponse(w, "Заказ успешно принят", map[string]interface{}{
			"order_id":  reqData.OrderID,
			"driver_id": driver.ID,
			"status":    "accepted",
		})
	}
}

// FIXED: getDeliveryOrderById retrieves a delivery order by ID
func (h *Handler) getDeliveryOrderById(orderID string) (*domain.DeliveryRequest, error) {
	query := `
		SELECT 
			id, telegram_id, from_address, from_lat, from_lon, 
			to_address, to_lat, to_lon, distance_km, eta_min,
			price, truck_type, contact, time_start, comment, 
			status, created_at
		FROM delivery_requests 
		WHERE id = ?`

	var order domain.DeliveryRequest
	err := h.db.QueryRow(query, orderID).Scan(
		&order.ID, &order.TelegramID, &order.FromAddress, &order.FromLat, &order.FromLon,
		&order.ToAddress, &order.ToLat, &order.ToLon, &order.DistanceKm, &order.EtaMin,
		&order.Price, &order.TruckType, &order.Contact, &order.TimeStart, &order.Comment,
		&order.Status, &order.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Order doesn't exist
		}
		return nil, err
	}

	return &order, nil
}

// FIXED: updateOrderStatus updates the status of a delivery order
func (h *Handler) updateOrderStatus(orderID string, status string, driverID string) error {
	// Validate status value against allowed values
	allowedStatuses := map[string]bool{
		"pending":     true,
		"matched":     true,
		"in_progress": true,
		"completed":   true,
		"cancelled":   true,
	}

	if !allowedStatuses[status] {
		return fmt.Errorf("invalid status '%s'. Allowed values: pending, matched, in_progress, completed, cancelled", status)
	}

	var query string
	var args []interface{}

	if driverID != "" && driverID != "0" { // FIXED: Proper string comparison
		query = `
			UPDATE delivery_requests 
			SET status = ?, driver_id = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND status = 'pending'`
		args = []interface{}{status, driverID, orderID}

		log.Printf("🔄 Updating order %s: status='%s', driverID=%s", orderID, status, driverID) // FIXED: %s for strings
	} else {
		query = `
			UPDATE delivery_requests 
			SET status = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`
		args = []interface{}{status, orderID}

		log.Printf("🔄 Updating order %s: status='%s'", orderID, status) // FIXED: %s for string
	}

	result, err := h.db.Exec(query, args...)
	if err != nil {
		log.Printf("❌ Database error updating order %s: %v", orderID, err) // FIXED: %s for string
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		log.Printf("⚠️ No rows affected for order %s (not found or already accepted)", orderID) // FIXED: %s for string
		return fmt.Errorf("order not found or already accepted")
	}

	log.Printf("✅ Successfully updated order %s status to '%s'", orderID, status) // FIXED: %s for string
	return nil
}

// FIXED: sendOrderAcceptedNotifications sends notifications when order is accepted
func (h *Handler) sendOrderAcceptedNotifications(b *bot.Bot, order *domain.DeliveryRequest, driver *DriverRegistration) {
	// Send notification to client
	if order.TelegramID != 0 {
		// FIXED: Using %s for string UUID
		clientMessage := fmt.Sprintf(`🚚 Сіздің тапсырысыңыз қабылданды! 🎉

📋 Тапсырыс: #%s

👤 Жүргізуші: %s %s
📱 Байланыс: %s

📍 Қайдан: %s
🎯 Қайда: %s

💰 Бағасы: %d ₸

✅ Жүргізуші сізбен жақын арада байланысады! 😊`,
			order.ID, // FIXED: Using %s for string UUID
			driver.FirstName,
			driver.LastName,
			driver.ContactNumber,
			order.FromAddress,
			order.ToAddress,
			order.Price,
		)

		ctx := context.Background()
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: order.TelegramID,
			Text:   clientMessage,
		})

		if err != nil {
			h.logger.Error("Failed to send client notification",
				zap.Error(err),
				zap.Int64("client_telegram_id", order.TelegramID))
		} else {
			h.logger.Info("Client notification sent",
				zap.Int64("client_telegram_id", order.TelegramID),
				zap.String("order_id", order.ID))
		}
	}

	// Send notification to driver
	if driver.TelegramID != 0 {
		// FIXED: Using %s for string UUID
		driverMessage := fmt.Sprintf(`✅ Тапсырыс қабылданды! 🎊

📋 Тапсырыс: #%s

📍 Қайдан: %s
🎯 Қайда: %s

💰 Бағасы: %d ₸
📱 Клиент: %s

🚚 Толық мәліметтер үшін клиентпен байланысыңыз! 💪`,
			order.ID, // FIXED: Using %s for string UUID
			order.FromAddress,
			order.ToAddress,
			order.Price,
			order.Contact,
		)

		var onlyDigits func(s string) string
		onlyDigits = func(s string) string {
			var b strings.Builder
			for i := 0; i < len(s); i++ {
				r := rune(s[i])
				if r >= '0' && r <= '9' {
					b.WriteRune(r)
				}
			}
			return b.String()
		}

		// Add contact buttons in Kazakh
		keyboard := &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "📞 Қоңырау шалу", URL: "tel:" + order.Contact},
					{Text: "💬 WhatsApp", URL: "https://wa.me/" + onlyDigits(order.Contact)},
				},
			},
		}

		ctx := context.Background()
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      driver.TelegramID,
			Text:        driverMessage,
			ReplyMarkup: keyboard,
		})

		if err != nil {
			h.logger.Error("Failed to send driver notification",
				zap.Error(err),
				zap.Int64("driver_telegram_id", driver.TelegramID))
		} else {
			h.logger.Info("Driver notification sent",
				zap.Int64("driver_telegram_id", driver.TelegramID),
				zap.String("order_id", order.ID))
		}
	}
}

// Enhanced Point A matching - core algorithm
func (h *Handler) findDriversByPointA(params DriverRequestParams) ([]MatchedDriver, error) {
	// SQL query to find drivers whose START point is near client's PICKUP point
	query := `
		SELECT 
			d.id, d.telegram_id, d.first_name, d.last_name, d.contact_number,
			d.truck_type, d.profile_photo, d.is_verified,
			dr.id as route_id, dr.from_address, dr.to_address, 
			dr.from_lat, dr.from_lon, dr.to_lat, dr.to_lon,
			dr.price, dr.departure_time, dr.comment, dr.truck_photo,
			(6371 * acos(
				cos(radians(?)) * cos(radians(dr.from_lat)) * 
				cos(radians(dr.from_lon) - radians(?)) + 
				sin(radians(?)) * sin(radians(dr.from_lat))
			)) AS distance_to_pickup_km
		FROM drivers d
		INNER JOIN driver_trips dr ON d.id = dr.driver_id
		WHERE dr.status = 'active'
			AND dr.departure_time > NOW()
			AND (6371 * acos(
				cos(radians(?)) * cos(radians(dr.from_lat)) * 
				cos(radians(dr.from_lon) - radians(?)) + 
				sin(radians(?)) * sin(radians(dr.from_lat))
			)) <= ?
		ORDER BY distance_to_pickup_km ASC, dr.price ASC
		LIMIT 50
	`

	rows, err := h.db.Query(query,
		params.PickupLat, params.PickupLon, params.PickupLat,
		params.PickupLat, params.PickupLon, params.PickupLat,
		params.RadiusKm)

	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}
	defer rows.Close()

	var drivers []MatchedDriver
	for rows.Next() {
		var driver MatchedDriver
		err := rows.Scan(
			&driver.ID, &driver.TelegramID, &driver.FirstName, &driver.LastName,
			&driver.ContactNumber, &driver.TruckType, &driver.ProfilePhoto, &driver.IsVerified,
			&driver.RouteID, &driver.FromAddress, &driver.ToAddress,
			&driver.FromLat, &driver.FromLon, &driver.ToLat, &driver.ToLon,
			&driver.Price, &driver.DepartureTime, &driver.Comment, &driver.TruckPhoto,
			&driver.DistanceToPickupKm)

		if err != nil {
			h.logger.Warn("⚠️ Failed to scan driver row", zap.Error(err))
			continue
		}

		drivers = append(drivers, driver)
	}

	h.logger.Info("🎯 Point A search completed",
		zap.Int("drivers_found", len(drivers)),
		zap.Float64("search_radius_km", params.RadiusKm))

	return drivers, nil
}

// Filter drivers by truck type and price compatibility
func (h *Handler) filterDriversByCompatibility(drivers []MatchedDriver, params DriverRequestParams) []MatchedDriver {
	var compatible []MatchedDriver

	for _, driver := range drivers {
		// Truck type compatibility
		if params.TruckType != "" && params.TruckType != "any" {
			if driver.TruckType != params.TruckType {
				continue
			}
		}

		// Price compatibility (driver price should be reasonable)
		if params.PriceRange > 0 {
			priceDiff := math.Abs(float64(driver.Price - params.PriceRange))
			maxPriceDiff := float64(params.PriceRange) * 0.5 // 50% tolerance

			if priceDiff > maxPriceDiff {
				continue
			}
		}

		compatible = append(compatible, driver)
	}

	h.logger.Info("🔍 Compatibility filtering completed",
		zap.Int("original_count", len(drivers)),
		zap.Int("compatible_count", len(compatible)))

	return compatible
}

// Rank drivers by route match quality
func (h *Handler) rankDriversByRouteMatch(drivers []MatchedDriver, params DriverRequestParams) []MatchedDriver {
	for i := range drivers {
		driver := &drivers[i]

		// Calculate Point B distance (dropoff similarity)
		distanceToDropoff := h.haversineDistance(
			params.DropoffLat, params.DropoffLon,
			driver.ToLat, driver.ToLon)
		driver.DistanceToDropoffKm = distanceToDropoff

		// Calculate overall route match score (0-100)
		pickupScore := h.calculateProximityScore(driver.DistanceToPickupKm)
		dropoffScore := h.calculateProximityScore(driver.DistanceToDropoffKm)

		// Point A (pickup) is weighted more heavily (70% vs 30%)
		driver.RouteMatchScore = int(pickupScore*0.7 + dropoffScore*0.3)

		// Determine match quality
		if driver.DistanceToPickupKm <= 2.0 && driver.DistanceToDropoffKm <= 5.0 {
			driver.MatchQuality = "perfect"
		} else if driver.DistanceToPickupKm <= 5.0 && driver.DistanceToDropoffKm <= 10.0 {
			driver.MatchQuality = "good"
		} else {
			driver.MatchQuality = "fair"
		}

		// Calculate estimated time to pickup
		driver.ETAMin = int(driver.DistanceToPickupKm * 3.0) // ~3 min per km in city
		if driver.ETAMin < 5 {
			driver.ETAMin = 5
		}
	}

	// Sort by match score (highest first), then by pickup distance (lowest first)
	sort.Slice(drivers, func(i, j int) bool {
		if drivers[i].RouteMatchScore != drivers[j].RouteMatchScore {
			return drivers[i].RouteMatchScore > drivers[j].RouteMatchScore
		}
		return drivers[i].DistanceToPickupKm < drivers[j].DistanceToPickupKm
	})

	return drivers
}

// handleUserCancelOrder handles user canceling their order
func (h *Handler) handleUserCancelOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	h.logger.Info("Received user cancel order request",
		zap.String("method", r.Method))

	// Parse request
	var reqData struct {
		TelegramID int64  `json:"telegram_id"`
		OrderID    string `json:"order_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
		h.logger.Error("Failed to parse request body", zap.Error(err))
		h.sendErrorResponse(w, "Неверные данные запроса", http.StatusBadRequest)
		return
	}

	if reqData.TelegramID == 0 || reqData.OrderID == "" {
		h.sendErrorResponse(w, "Telegram ID и Order ID обязательны", http.StatusBadRequest)
		return
	}

	// Verify order belongs to this user and is cancellable
	var currentStatus string
	var orderTelegramID int64
	query := `SELECT status, telegram_id FROM delivery_requests WHERE id = ?`
	err := h.db.QueryRow(query, reqData.OrderID).Scan(&currentStatus, &orderTelegramID)

	if err != nil {
		if err == sql.ErrNoRows {
			h.sendErrorResponse(w, "Заказ не найден", http.StatusNotFound)
			return
		}
		h.logger.Error("Failed to query order", zap.Error(err))
		h.sendErrorResponse(w, "Ошибка проверки заказа", http.StatusInternalServerError)
		return
	}

	// Verify ownership
	if orderTelegramID != reqData.TelegramID {
		h.sendErrorResponse(w, "Нет доступа к этому заказу", http.StatusForbidden)
		return
	}

	// Check if order can be cancelled
	currentStatus = strings.ToLower(currentStatus)
	if currentStatus == "completed" || currentStatus == "cancelled" {
		h.sendErrorResponse(w, "Этот заказ нельзя отменить", http.StatusConflict)
		return
	}

	// Update order status to cancelled
	updateQuery := `
		UPDATE delivery_requests 
		SET status = 'cancelled', updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND telegram_id = ?`

	result, err := h.db.Exec(updateQuery, reqData.OrderID, reqData.TelegramID)
	if err != nil {
		h.logger.Error("Failed to cancel order", zap.Error(err))
		h.sendErrorResponse(w, "Ошибка отмены заказа", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		h.sendErrorResponse(w, "Заказ не найден или уже отменён", http.StatusNotFound)
		return
	}

	h.logger.Info("Order cancelled successfully",
		zap.String("order_id", reqData.OrderID),
		zap.Int64("telegram_id", reqData.TelegramID))

	h.sendSuccessResponse(w, "Заказ успешно отменён", map[string]interface{}{
		"order_id": reqData.OrderID,
		"status":   "cancelled",
	})
}

// Add real-time availability and status data
func (h *Handler) enrichDriversWithRealTimeData(drivers []MatchedDriver, params DriverRequestParams) []MatchedDriver {
	for i := range drivers {
		driver := &drivers[i]

		// Check real-time availability (simplified - could integrate with live tracking)
		driver.IsOnline = time.Now().Unix()%2 == 0       // Mock: 50% online
		driver.LastSeenMin = int(time.Now().Unix() % 30) // Mock: seen within 30 min

		// Add verification badges
		driver.HasWhatsApp = len(driver.ContactNumber) > 0
		driver.HasTelegram = driver.TelegramID != ""

		// Calculate response time expectation
		if driver.IsOnline {
			driver.ResponseTimeMin = 2
		} else {
			driver.ResponseTimeMin = 15
		}
	}

	return drivers
}

// Calculate search statistics
func (h *Handler) calculateSearchStats(drivers []MatchedDriver, params DriverRequestParams) SearchStats {
	if len(drivers) == 0 {
		return SearchStats{}
	}

	var totalDistance float64
	var perfectMatches, goodMatches int

	for _, driver := range drivers {
		totalDistance += driver.DistanceToPickupKm

		switch driver.MatchQuality {
		case "perfect":
			perfectMatches++
		case "good":
			goodMatches++
		}
	}

	return SearchStats{
		AvgDistance:    totalDistance / float64(len(drivers)),
		PerfectMatches: perfectMatches,
		GoodMatches:    goodMatches,
	}
}

// findMatchingDrivers finds drivers whose trip start points are near the pickup location
func (h *Handler) findMatchingDrivers(params DriverRequestParams) ([]DriverWithTrip, error) {
	h.logger.Info("Finding matching drivers",
		zap.Float64("pickup_lat", params.PickupLat),
		zap.Float64("pickup_lon", params.PickupLon),
		zap.Float64("radius_km", params.RadiusKm),
		zap.String("truck_type", params.TruckType))

	// Query to find drivers with active trips whose start point is near pickup location
	query := `
		SELECT 
			d.id, d.telegram_id, d.first_name, d.last_name, d.contact_number, d.profile_photo,
			dt.id, dt.from_address, dt.from_lat, dt.from_lon, 
			dt.to_address, dt.to_lat, dt.to_lon, dt.price, dt.start_time, 
			dt.comment, dt.distance_km, dt.eta_min
		FROM drivers d
		INNER JOIN driver_trips dt ON d.telegram_id = dt.telegram_id
		WHERE d.status = 'approved' 
		AND dt.status = 'active'
		AND dt.created_at >= datetime('now', '-24 hours')
		AND dt.from_lat != 0 AND dt.from_lon != 0
		ORDER BY dt.created_at DESC
		LIMIT 100`

	rows, err := h.db.Query(query)
	if err != nil {
		h.logger.Error("Database query failed", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var allDrivers []DriverWithTrip
	driversProcessed := 0
	driversMatched := 0

	for rows.Next() {
		var driver DriverWithTrip
		err := rows.Scan(
			&driver.ID, &driver.TelegramID, &driver.FirstName, &driver.LastName,
			&driver.ContactNumber, &driver.ProfilePhoto,
			&driver.TripID, &driver.FromAddress, &driver.FromLat, &driver.FromLon,
			&driver.ToAddress, &driver.ToLat, &driver.ToLon, &driver.Price,
			&driver.StartTime, &driver.Comment, &driver.DistanceKm, &driver.EtaMin,
		)
		if err != nil {
			h.logger.Error("Error scanning driver row", zap.Error(err))
			continue
		}

		driversProcessed++

		// Calculate distance from driver's start point to client's pickup point
		distance := h.haversineDistance(params.PickupLat, params.PickupLon, driver.FromLat, driver.FromLon)

		h.logger.Debug("Processing driver",
			zap.String("driver_id", driver.ID),
			zap.String("driver_name", driver.FirstName+" "+driver.LastName),
			zap.Float64("distance_to_pickup", distance),
			zap.Float64("max_radius", params.RadiusKm))

		// Check if driver is within radius
		if distance <= params.RadiusKm {
			driversMatched++

			// Optional: Filter by truck type if specified and not "any"
			if params.TruckType != "" && params.TruckType != "any" {
				// For now, we'll include all drivers since truck_type isn't in driver_trips table
				// You can add truck_type to driver_trips table and uncomment this filter:
				// if driver.TruckType != params.TruckType {
				//     continue
				// }
			}

			// Set truck type as "any" for now (you can extend this)
			driver.TruckType = "any"

			allDrivers = append(allDrivers, driver)

			h.logger.Debug("Driver matched",
				zap.String("driver_id", driver.ID),
				zap.Float64("distance", distance))
		}
	}

	h.logger.Info("Driver matching completed",
		zap.Int("total_processed", driversProcessed),
		zap.Int("drivers_matched", driversMatched),
		zap.Float64("radius_km", params.RadiusKm))

	// If no drivers found with current radius, try expanding
	if len(allDrivers) == 0 && params.RadiusKm < 25 {
		h.logger.Info("No drivers found, expanding search radius")
		expandedParams := params
		expandedParams.RadiusKm = 25.0
		return h.findMatchingDrivers(expandedParams)
	}

	// Sort by distance (closest first)
	if len(allDrivers) > 1 {
		for i := 0; i < len(allDrivers)-1; i++ {
			for j := i + 1; j < len(allDrivers); j++ {
				distI := h.haversineDistance(params.PickupLat, params.PickupLon, allDrivers[i].FromLat, allDrivers[i].FromLon)
				distJ := h.haversineDistance(params.PickupLat, params.PickupLon, allDrivers[j].FromLat, allDrivers[j].FromLon)
				if distI > distJ {
					allDrivers[i], allDrivers[j] = allDrivers[j], allDrivers[i]
				}
			}
		}
	}

	// Limit to top 20 results
	if len(allDrivers) > 20 {
		allDrivers = allDrivers[:20]
	}

	return allDrivers, nil
}

// handleDriverList serves the driver list page (GET /driver-list)
func (h *Handler) handleDriverList(w http.ResponseWriter, r *http.Request) {
	path := "./static/driver-list.html"
	w.Header().Set("Content-Type", "text/html")

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		h.logger.Error("Driver list page not found", zap.String("path", path))
		http.Error(w, "Driver list page not found", http.StatusNotFound)
		return
	}

	h.logger.Info("Serving driver list page",
		zap.String("user_agent", r.Header.Get("User-Agent")),
		zap.String("order_param", r.URL.Query().Get("order")))

	http.ServeFile(w, r, path)
}

// updateExpiredTrips marks old trips as completed
func (h *Handler) updateExpiredTrips() {
	// Mark trips older than 12 hours as completed
	query := `
		UPDATE driver_trips 
		SET status = 'completed' 
		WHERE status = 'active' 
		AND created_at < datetime('now', '-12 hours')`

	result, err := h.db.Exec(query)
	if err != nil {
		h.logger.Error("Failed to update expired trips", zap.Error(err))
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		h.logger.Info("Updated expired trips", zap.Int64("count", rowsAffected))
	}
}

// haversineDistance calculates the distance between two points on Earth
func (h *Handler) haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371 // Earth's radius in kilometers

	// Convert degrees to radians
	lat1Rad := lat1 * math.Pi / 180
	lon1Rad := lon1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lon2Rad := lon2 * math.Pi / 180

	// Calculate differences
	deltaLat := lat2Rad - lat1Rad
	deltaLon := lon2Rad - lon1Rad

	// Haversine formula
	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

// calculateProximityScore calculates a score (0-100) based on distance
func (h *Handler) calculateProximityScore(distanceKm float64) float64 {
	if distanceKm <= 1.0 {
		return 100.0
	} else if distanceKm <= 5.0 {
		return 100.0 - (distanceKm-1.0)*20.0 // 100 to 20
	} else if distanceKm <= 20.0 {
		return 20.0 - (distanceKm-5.0)*1.33 // 20 to 0
	}
	return 0.0
}

// DefaultHandler for Telegram bot to use welcome page
func (h *Handler) DefaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	var keyboard interface{}
	if update.Message.From.ID == h.cfg.AdminTelegramID {
		keyboard = &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{
						Text:   "🚀 Ашу | Открыть QazLine",
						WebApp: &models.WebAppInfo{URL: h.cfg.BaseURL + "/"},
					},
					{
						Text:   "👤 Admin",
						WebApp: &models.WebAppInfo{URL: h.cfg.BaseURL + "/admin"},
					},
				},
			},
		}
	} else {
		// Create inline keyboard with welcome page
		keyboard = &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{
						Text:   "🚀 Ашу | Открыть QazLine",
						WebApp: &models.WebAppInfo{URL: h.cfg.BaseURL + "/"},
					},
				},
			},
		}
	}

	// Bilingual welcome message
	message := `AlashGo — Қазақстанға арналған заманауи цифрлық платформа 🚀
Қызмет табу, тапсырыс беру, жүргізушілермен не клиенттермен байланысу — барлығы бір жерде.

Біздің мақсат — сервисті жеңіл, жылдам жəне қолжетімді ету.
AlashGo — ұлттық стильдегі жаңа буын сервисі 🇰🇿✨`

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.From.ID,
		Text:        message,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		h.logger.Error("Failed to send welcome message",
			zap.Error(err),
			zap.Int64("user_id", update.Message.From.ID))
	} else {
		h.logger.Info("Welcome message sent",
			zap.Int64("user_id", update.Message.From.ID),
			zap.String("username", update.Message.From.Username))
	}
}
