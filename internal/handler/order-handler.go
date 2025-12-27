package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"tezjet/internal/domain"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ==================
// TYPE DEFINITIONS
// ==================

type DriverTrip struct {
	ID               string    `json:"id"`
	DriverID         string    `json:"driver_id"`
	TelegramID       int64     `json:"telegram_id"`
	FromAddress      string    `json:"from_address"`
	FromLat          float64   `json:"from_lat"`
	FromLon          float64   `json:"from_lon"`
	ToAddress        string    `json:"to_address"`
	ToLat            float64   `json:"to_lat"`
	ToLon            float64   `json:"to_lon"`
	DistanceKm       float64   `json:"distance_km"`
	EtaMin           int       `json:"eta_min"`
	Price            int       `json:"price"`
	TruckType        string    `json:"truck_type"`
	StartTime        string    `json:"start_time"`
	Comment          string    `json:"comment"`
	TruckPhoto       string    `json:"truck_photo"`
	HasWhatsApp      bool      `json:"has_whatsapp"`
	HasTelegram      bool      `json:"has_telegram"`
	TelegramUsername string    `json:"telegram_username"`
	DepartureTime    time.Time `json:"departure_time"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
}

type DriverWithTrip struct {
	// Driver info
	ID            string `json:"id"`
	TelegramID    int64  `json:"telegram_id"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	FullName      string `json:"full_name"`
	ContactNumber string `json:"contact_number"`
	Contact       string `json:"contact"`
	ProfilePhoto  string `json:"profile_photo"`
	TruckType     string `json:"truck_type"`
	IsVerified    bool   `json:"is_verified"`

	// Trip details
	TripID        int64   `json:"trip_id"`
	FromAddress   string  `json:"from_address"`
	FromLat       float64 `json:"from_lat"`
	FromLon       float64 `json:"from_lon"`
	ToAddress     string  `json:"to_address"`
	ToLat         float64 `json:"to_lat"`
	ToLon         float64 `json:"to_lon"`
	Price         int     `json:"price"`
	StartTime     string  `json:"start_time"`
	DepartureTime string  `json:"departure_time"`
	Comment       string  `json:"comment"`
	TruckPhoto    string  `json:"truck_photo"`
	DistanceKm    float64 `json:"distance_km"`
	EtaMin        int     `json:"eta_min"`
	StartCity     string  `json:"start_city"`
	EndCity       string  `json:"end_city"`

	// Contact methods
	HasWhatsApp      bool   `json:"has_whatsapp"`
	HasTelegram      bool   `json:"has_telegram"`
	TelegramUsername string `json:"telegram_username"`

	// Matching scores
	DistanceToPickupKm  float64 `json:"distance_to_pickup_km,omitempty"`
	DistanceToDropoffKm float64 `json:"distance_to_dropoff_km,omitempty"`
	RouteMatchScore     int     `json:"route_match_score,omitempty"`
	MatchQuality        string  `json:"match_quality,omitempty"`
	ETAMin              int     `json:"eta_min_pickup,omitempty"`
	IsOnline            bool    `json:"is_online,omitempty"`
	LastSeenMin         int     `json:"last_seen_min,omitempty"`
	ResponseTimeMin     int     `json:"response_time_min,omitempty"`
}

type DeliveryListRequest struct {
	TelegramID int64   `json:"telegram_id"`
	DriverLat  float64 `json:"driver_lat"`
	DriverLon  float64 `json:"driver_lon"`
	Radius     float64 `json:"radius"`
}

type DeliveryListResponse struct {
	Orders      []domain.DeliveryRequest `json:"orders"`
	TotalCount  int                      `json:"total_count"`
	NearbyCount int                      `json:"nearby_count"`
	AvgPrice    float64                  `json:"avg_price"`
}

type DriverRegistration struct {
	ID            string    `json:"id"`
	TelegramID    int64     `json:"telegram_id"`
	FirstName     string    `json:"first_name"`
	LastName      string    `json:"last_name"`
	Birthday      string    `json:"birthday"`
	ContactNumber string    `json:"contact_number"`
	StartCity     string    `json:"start_city"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	ProfilePhoto  string    `json:"profile_photo"`
	LicenseFront  string    `json:"license_front"`
	LicenseBack   string    `json:"license_back"`
	TruckType     string    `json:"truck_type"`
	TruckNumber   string    `json:"truck_number"`
	IsVerified    bool      `json:"is_verified"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

type DriverRequestParams struct {
	RequestID  string  `json:"request_id"`
	PickupLat  float64 `json:"pickup_lat"`
	PickupLon  float64 `json:"pickup_lon"`
	DropoffLat float64 `json:"dropoff_lat"`
	DropoffLon float64 `json:"dropoff_lon"`
	PriceRange int     `json:"price_range"`
	TruckType  string  `json:"truck_type"`
	RadiusKm   float64 `json:"radius_km"`
}

type DriverMatchResponse struct {
	Drivers          []DriverWithTrip       `json:"drivers"`
	TotalCount       int                    `json:"total_count"`
	AvgDistance      float64                `json:"avg_distance"`
	SearchTime       string                 `json:"search_time"`
	PerfectMatches   int                    `json:"perfect_matches"`
	GoodMatches      int                    `json:"good_matches"`
	SearchRadius     float64                `json:"search_radius"`
	MatchingCriteria map[string]interface{} `json:"matching_criteria"`
}

type SearchStats struct {
	AvgDistance    float64 `json:"avg_distance"`
	PerfectMatches int     `json:"perfect_matches"`
	GoodMatches    int     `json:"good_matches"`
}

type deliveryRequestJSON struct {
	FromAddress string  `json:"from_address"`
	FromLat     float64 `json:"from_lat"`
	FromLon     float64 `json:"from_lon"`
	ToAddress   string  `json:"to_address"`
	ToLat       float64 `json:"to_lat"`
	ToLon       float64 `json:"to_lon"`
	Contact     string  `json:"contact"`
	TruckType   string  `json:"truck_type"`
	Comment     string  `json:"comment"`
	TimeStart   string  `json:"time_start"`
	Date        string  `json:"date"`
	Time        string  `json:"time"`
	Price       int     `json:"price"`
	ETAMin      int     `json:"duration"`
	DistanceKm  float64 `json:"distance"`
	TelegramID  int64   `json:"telegram_id"`
}

// =================================
// MAIN DRIVER LIST API HANDLER
// =================================

// HandleDriverListAPI returns available drivers whose routes match client's delivery route
// CRITICAL: Driver's A→B route must match Client's A→B route within radius
// HandleDriverListAPI handles GET requests for driver list with query parameters
func (h *Handler) HandleDriverListAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	h.logger.Info("🔍 Received driver list API request (GET with query params)")

	// Parse query parameters
	fromLat, _ := strconv.ParseFloat(r.URL.Query().Get("from_lat"), 64)
	fromLon, _ := strconv.ParseFloat(r.URL.Query().Get("from_lon"), 64)
	toLat, _ := strconv.ParseFloat(r.URL.Query().Get("to_lat"), 64)
	toLon, _ := strconv.ParseFloat(r.URL.Query().Get("to_lon"), 64)
	radiusKm, _ := strconv.ParseFloat(r.URL.Query().Get("radius"), 64)
	truckType := r.URL.Query().Get("truck_type")

	h.logger.Info("📊 Parsed query parameters",
		zap.Float64("from_lat", fromLat),
		zap.Float64("from_lon", fromLon),
		zap.Float64("to_lat", toLat),
		zap.Float64("to_lon", toLon),
		zap.Float64("radius_km", radiusKm),
		zap.String("truck_type", truckType))

	// Validate coordinates
	if !h.isValidCoordinates(fromLat, fromLon) || !h.isValidCoordinates(toLat, toLon) {
		h.logger.Error("❌ Invalid coordinates provided")
		h.sendErrorResponse(w, "Некорректные координаты", http.StatusBadRequest)
		return
	}

	// Default radius if not provided
	if radiusKm == 0 {
		radiusKm = 50.0
	}

	startTime := time.Now()

	// Find drivers using route-to-route matching
	drivers, err := h.findDriversByRouteMatching(fromLat, fromLon, toLat, toLon, radiusKm, truckType)
	if err != nil {
		h.logger.Error("❌ Failed to find drivers", zap.Error(err))
		h.sendErrorResponse(w, "Ошибка поиска водителей", http.StatusInternalServerError)
		return
	}

	searchDuration := time.Since(startTime)

	h.logger.Info("✅ Driver search completed",
		zap.Int("drivers_found", len(drivers)),
		zap.Duration("duration", searchDuration))

	// Send response
	h.sendSuccessResponse(w, "Водители найдены", map[string]interface{}{
		"drivers": drivers,
		"count":   len(drivers),
		"search_params": map[string]interface{}{
			"from_lat":   fromLat,
			"from_lon":   fromLon,
			"to_lat":     toLat,
			"to_lon":     toLon,
			"radius_km":  radiusKm,
			"truck_type": truckType,
		},
		"matching_logic": map[string]interface{}{
			"type":        "route_to_route",
			"description": "Driver's A→B route matches Client's A→B route within radius",
		},
	})
}

// =================================
// CORE ROUTE MATCHING LOGIC
// =================================

// =================================
// DELIVERY REQUEST HANDLER
// =================================

func (h *Handler) HandleDelivery(ctx context.Context, b *bot.Bot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			h.sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		h.logger.Info("📦 Delivery request received",
			zap.String("method", r.Method),
			zap.String("content_type", r.Header.Get("Content-Type")),
		)

		ct := strings.ToLower(r.Header.Get("Content-Type"))

		var (
			req *domain.DeliveryRequest
			err error
		)

		switch {
		case strings.Contains(ct, "application/json"):
			req, err = h.parseDeliveryRequestJSON(r)
		case strings.Contains(ct, "multipart/form-data"):
			if err = r.ParseMultipartForm(32 << 20); err != nil {
				h.logger.Error("Failed to parse multipart form", zap.Error(err))
				h.sendErrorResponse(w, "Ошибка обработки данных", http.StatusBadRequest)
				return
			}
			req, err = h.parseDeliveryRequest(r)
		default:
			if err = r.ParseForm(); err != nil {
				h.logger.Error("Failed to parse form", zap.Error(err))
				h.sendErrorResponse(w, "Ошибка обработки данных", http.StatusBadRequest)
				return
			}
			req, err = h.parseDeliveryRequest(r)
		}

		if err != nil {
			h.logger.Error("Failed to parse delivery request", zap.Error(err))
			h.sendErrorResponse(w, err.Error(), http.StatusBadRequest)
			return
		}

		requestId := uuid.New().String()
		req.ID = requestId

		// Save photo if multipart
		if strings.Contains(ct, "multipart/form-data") {
			if path, err := h.saveUploadedPhoto(r, requestId); err != nil {
				h.logger.Error("Photo save failed", zap.Error(err))
			} else if path != "" {
				req.CargoPhoto = path
			}
		}

		// Calculate route if missing
		if req.DistanceKm == 0 || req.EtaMin == 0 {
			distance, duration := h.calculateRoute(req.FromLat, req.FromLon, req.ToLat, req.ToLon)
			if req.DistanceKm == 0 {
				req.DistanceKm = distance
			}
			if req.EtaMin == 0 {
				req.EtaMin = duration
			}
		}

		// Save to database
		if _, err := h.saveDeliveryRequest(req); err != nil {
			h.logger.Error("Failed to save delivery request", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка сохранения заявки", http.StatusInternalServerError)
			return
		}

		req.Status = "pending"
		req.CreatedAt = time.Now()

		// Send notifications asynchronously
		go h.sendConfirmationMessage(b, req, req.ID)
		go h.SendToDriver(ctx, b, req)

		h.sendSuccessResponse(w, "Заявка успешно создана", map[string]interface{}{
			"request_id": req.ID,
			"status":     "pending",
			"distance":   req.DistanceKm,
			"eta":        req.EtaMin,
			"photo":      req.CargoPhoto,
		})
	}
}

// =================================
// DELIVERY LIST FOR DRIVERS
// =================================

func (h *Handler) HandleDeliveryList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		var req DeliveryListRequest
		switch r.Method {
		case http.MethodPost:
			ct := strings.ToLower(r.Header.Get("Content-Type"))
			if !strings.Contains(ct, "application/json") {
				h.sendErrorResponse(w, "Content-Type должен быть application/json", http.StatusBadRequest)
				return
			}
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&req); err != nil {
				h.sendErrorResponse(w, fmt.Sprintf("неверный JSON: %v", err), http.StatusBadRequest)
				return
			}
		case http.MethodGet:
			req.TelegramID, _ = strconv.ParseInt(r.URL.Query().Get("telegram_id"), 10, 64)
			req.DriverLat, _ = strconv.ParseFloat(r.URL.Query().Get("driver_lat"), 64)
			req.DriverLon, _ = strconv.ParseFloat(r.URL.Query().Get("driver_lon"), 64)
			req.Radius, _ = strconv.ParseFloat(r.URL.Query().Get("radius"), 64)
		default:
			h.sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if req.DriverLat == 0 || req.DriverLon == 0 {
			h.sendErrorResponse(w, "driver_lat и driver_lon обязательны", http.StatusBadRequest)
			return
		}
		if req.Radius <= 0 {
			req.Radius = 30.0
		}
		if req.Radius > 200 {
			req.Radius = 200
		}

		orders, totalCount, err := h.getPendingDeliveryRequestsLast24h()
		if err != nil {
			h.logger.Error("Failed to load delivery requests", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка получения заказов", http.StatusInternalServerError)
			return
		}

		type orderWithDist struct {
			o    domain.DeliveryRequest
			dist float64
		}
		var nearby []orderWithDist
		var sumPrice int

		for _, o := range orders {
			if o.FromLat == 0 || o.FromLon == 0 {
				continue
			}
			d := h.haversineDistance(req.DriverLat, req.DriverLon, o.FromLat, o.FromLon)
			if d <= req.Radius {
				sumPrice += o.Price
				nearby = append(nearby, orderWithDist{o: o, dist: d})
			}
		}

		sort.Slice(nearby, func(i, j int) bool { return nearby[i].dist < nearby[j].dist })

		out := make([]domain.DeliveryRequest, 0, len(nearby))
		for _, it := range nearby {
			out = append(out, it.o)
		}

		avg := 0.0
		if len(out) > 0 {
			avg = float64(sumPrice) / float64(len(out))
		}

		resp := DeliveryListResponse{
			Orders:      out,
			TotalCount:  totalCount,
			NearbyCount: len(out),
			AvgPrice:    avg,
		}

		h.sendSuccessResponse(w, "Список заказов получен успешно", resp)
	}
}

func (h *Handler) getPendingDeliveryRequestsLast24h() ([]domain.DeliveryRequest, int, error) {
	const q = `
SELECT
  id,
  telegram_id,
  from_address,
  CAST(from_lat AS REAL) as from_lat,
  CAST(from_lon AS REAL) as from_lon,
  to_address,
  CAST(to_lat AS REAL) as to_lat,
  CAST(to_lon AS REAL) as to_lon,
  CAST(distance_km AS REAL) as distance_km,
  eta_min,
  price,
  truck_type,
  contact,
  time_start,
  comment,
  item_photo_path,
  status,
  created_at
FROM delivery_requests
WHERE
  created_at >= datetime('now', '-24 hours')
  AND (LOWER(status) = 'pending' OR LOWER(status) = 'active')
ORDER BY created_at DESC
LIMIT 500;
`

	rows, err := h.db.Query(q)
	if err != nil {
		return nil, 0, fmt.Errorf("query delivery_requests: %w", err)
	}
	defer rows.Close()

	out := make([]domain.DeliveryRequest, 0, 128)

	for rows.Next() {
		var (
			o             domain.DeliveryRequest
			id            sql.NullString
			tgID          sql.NullInt64
			fromAddr      sql.NullString
			fromLat       sql.NullFloat64
			fromLon       sql.NullFloat64
			toAddr        sql.NullString
			toLat         sql.NullFloat64
			toLon         sql.NullFloat64
			distKm        sql.NullFloat64
			etaMin        sql.NullInt64
			price         sql.NullInt64
			truckType     sql.NullString
			contact       sql.NullString
			timeStart     sql.NullString
			comment       sql.NullString
			photoPath     sql.NullString
			status        sql.NullString
			createdAtText sql.NullString
		)

		if err := rows.Scan(
			&id, &tgID, &fromAddr, &fromLat, &fromLon,
			&toAddr, &toLat, &toLon, &distKm, &etaMin,
			&price, &truckType, &contact, &timeStart,
			&comment, &photoPath, &status, &createdAtText,
		); err != nil {
			continue
		}

		if id.Valid {
			o.ID = id.String
		}
		if tgID.Valid {
			o.TelegramID = tgID.Int64
		}
		if fromAddr.Valid {
			o.FromAddress = strings.TrimSpace(fromAddr.String)
		}
		if toAddr.Valid {
			o.ToAddress = strings.TrimSpace(toAddr.String)
		}
		if fromLat.Valid {
			o.FromLat = fromLat.Float64
		}
		if fromLon.Valid {
			o.FromLon = fromLon.Float64
		}
		if toLat.Valid {
			o.ToLat = toLat.Float64
		}
		if toLon.Valid {
			o.ToLon = toLon.Float64
		}
		if distKm.Valid {
			o.DistanceKm = distKm.Float64
		}
		if etaMin.Valid {
			o.EtaMin = int(etaMin.Int64)
		}
		if price.Valid {
			o.Price = int(price.Int64)
		}
		if truckType.Valid {
			o.TruckType = strings.TrimSpace(truckType.String)
		}
		if contact.Valid {
			o.Contact = strings.TrimSpace(contact.String)
		}
		if timeStart.Valid {
			o.TimeStart = strings.TrimSpace(timeStart.String)
		}
		if comment.Valid {
			o.Comment = strings.TrimSpace(comment.String)
		}
		if photoPath.Valid {
			o.CargoPhoto = strings.TrimSpace(photoPath.String)
		}
		if status.Valid {
			o.Status = strings.TrimSpace(status.String)
		}

		out = append(out, o)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}

	return out, len(out), nil
}

// =================================
// HELPER FUNCTIONS
// =================================

func (h *Handler) parseDeliveryRequestJSON(r *http.Request) (*domain.DeliveryRequest, error) {
	var in deliveryRequestJSON
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return nil, fmt.Errorf("неверный JSON: %v", err)
	}

	req := &domain.DeliveryRequest{}
	req.FromAddress = strings.TrimSpace(in.FromAddress)
	req.ToAddress = strings.TrimSpace(in.ToAddress)
	req.Contact = strings.TrimSpace(in.Contact)
	req.TruckType = strings.TrimSpace(in.TruckType)
	req.Comment = strings.TrimSpace(in.Comment)

	if req.FromAddress == "" {
		return nil, fmt.Errorf("адрес отправления обязателен")
	}
	if req.ToAddress == "" {
		return nil, fmt.Errorf("адрес назначения обязателен")
	}
	if req.Contact == "" {
		return nil, fmt.Errorf("контактный номер обязателен")
	}

	req.FromLat, req.FromLon = in.FromLat, in.FromLon
	req.ToLat, req.ToLon = in.ToLat, in.ToLon

	req.Price = in.Price
	if req.Price < 2000 {
		return nil, fmt.Errorf("цена должна быть не менее 2000 тенге")
	}

	req.DistanceKm = in.DistanceKm
	req.EtaMin = in.ETAMin
	req.TelegramID = in.TelegramID

	if strings.TrimSpace(in.TimeStart) != "" {
		req.TimeStart = strings.TrimSpace(in.TimeStart)
	} else if strings.TrimSpace(in.Date) != "" && strings.TrimSpace(in.Time) != "" {
		req.TimeStart = strings.TrimSpace(in.Date) + "T" + strings.TrimSpace(in.Time)
	} else {
		req.TimeStart = time.Now().Format("2006-01-02T15:04")
	}

	return req, nil
}

func (h *Handler) parseDeliveryRequest(r *http.Request) (*domain.DeliveryRequest, error) {
	req := &domain.DeliveryRequest{}
	getValue := func(key string) string { return strings.TrimSpace(r.FormValue(key)) }

	req.FromAddress = getValue("from_address")
	req.ToAddress = getValue("to_address")
	req.Contact = getValue("contact")

	if req.FromAddress == "" {
		return nil, fmt.Errorf("адрес отправления обязателен")
	}
	if req.ToAddress == "" {
		return nil, fmt.Errorf("адрес назначения обязателен")
	}
	if req.Contact == "" {
		return nil, fmt.Errorf("контактный номер обязателен")
	}

	var err error
	if latStr := getValue("from_lat"); latStr != "" {
		req.FromLat, err = strconv.ParseFloat(latStr, 64)
		if err != nil {
			return nil, fmt.Errorf("неверная широта отправления: %v", err)
		}
	}
	if lonStr := getValue("from_lon"); lonStr != "" {
		req.FromLon, err = strconv.ParseFloat(lonStr, 64)
		if err != nil {
			return nil, fmt.Errorf("неверная долгота отправления: %v", err)
		}
	}
	if latStr := getValue("to_lat"); latStr != "" {
		req.ToLat, err = strconv.ParseFloat(latStr, 64)
		if err != nil {
			return nil, fmt.Errorf("неверная широта назначения: %v", err)
		}
	}
	if lonStr := getValue("to_lon"); lonStr != "" {
		req.ToLon, err = strconv.ParseFloat(lonStr, 64)
		if err != nil {
			return nil, fmt.Errorf("неверная долгота назначения: %v", err)
		}
	}

	if priceStr := getValue("price"); priceStr != "" {
		req.Price, err = strconv.Atoi(priceStr)
		if err != nil || req.Price < 2000 {
			return nil, fmt.Errorf("цена должна быть не менее 2000 тенге")
		}
	} else {
		return nil, fmt.Errorf("цена обязательна")
	}

	if distStr := getValue("distance"); distStr != "" {
		req.DistanceKm, _ = strconv.ParseFloat(distStr, 64)
	}
	if etaStr := getValue("duration"); etaStr != "" {
		req.EtaMin, _ = strconv.Atoi(etaStr)
	}
	if telegramIDStr := getValue("telegram_id"); telegramIDStr != "" {
		req.TelegramID, _ = strconv.ParseInt(telegramIDStr, 10, 64)
	}

	req.TruckType = getValue("truck_type")
	req.Comment = getValue("comment")

	dateStr := getValue("date")
	timeStr := getValue("time")
	if dateStr != "" && timeStr != "" {
		req.TimeStart = dateStr + "T" + timeStr
	} else {
		req.TimeStart = time.Now().Format("2006-01-02T15:04")
	}

	return req, nil
}

func (h *Handler) saveUploadedPhoto(r *http.Request, requestID string) (string, error) {
	file, header, err := r.FormFile("cargo_photo")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return "", nil
		}
		return "", fmt.Errorf("не удалось прочитать файл: %w", err)
	}
	defer file.Close()

	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", fmt.Errorf("ошибка чтения файла: %w", err)
	}
	head = head[:n]

	ctype := http.DetectContentType(head)
	if !strings.HasPrefix(ctype, "image/") {
		return "", fmt.Errorf("недопустимый тип файла: %s", ctype)
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		switch ctype {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/webp":
			ext = ".webp"
		default:
			ext = ".img"
		}
	}

	fname := requestID + ext
	dstPath := filepath.Join(h.cfg.CargoPhoto, fname)

	dst, err := os.Create(dstPath)
	if err != nil {
		return "", fmt.Errorf("не удалось создать файл: %w", err)
	}
	defer func() { _ = dst.Close() }()

	if _, err := dst.Write(head); err != nil {
		return "", fmt.Errorf("ошибка записи файла: %w", err)
	}

	remaining := h.cfg.MaxUploadSize - int64(len(head))
	if remaining < 0 {
		return "", fmt.Errorf("файл превышает 20MB")
	}

	written, err := io.Copy(dst, io.LimitReader(file, remaining))
	if err != nil {
		return "", fmt.Errorf("ошибка копирования файла: %w", err)
	}

	if header.Size > 0 && header.Size > h.cfg.MaxUploadSize {
		_ = os.Remove(dstPath)
		return "", fmt.Errorf("файл слишком большой (>20MB)")
	}

	if header.Size == 0 && written == remaining {
		buf := make([]byte, 1)
		if _, err := file.Read(buf); err == nil {
			_ = os.Remove(dstPath)
			return "", fmt.Errorf("файл слишком большой (>20MB)")
		}
	}

	return dstPath, nil
}

func (h *Handler) saveDeliveryRequest(req *domain.DeliveryRequest) (string, error) {
	requestID := req.ID
	if requestID == "" {
		requestID = uuid.New().String()
		req.ID = requestID
	}

	query := `
INSERT INTO delivery_requests (
    id, telegram_id, from_address, from_lat, from_lon,
    to_address, to_lat, to_lon, distance_km, eta_min,
    price, truck_type, contact, time_start, comment,
    item_photo_path, status, created_at
) VALUES (
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, 'pending', CURRENT_TIMESTAMP
)`

	_, err := h.db.Exec(
		query,
		requestID, req.TelegramID, req.FromAddress, req.FromLat, req.FromLon,
		req.ToAddress, req.ToLat, req.ToLon, req.DistanceKm, req.EtaMin,
		req.Price, req.TruckType, req.Contact, req.TimeStart, req.Comment,
		nullableString(req.CargoPhoto),
	)
	if err != nil {
		return "", err
	}
	return requestID, nil
}

func (h *Handler) SendToDriver(ctx context.Context, b *bot.Bot, req *domain.DeliveryRequest) {
	deltaLat := 30.0 / 111.32
	latRad := req.FromLat * math.Pi / 180.0
	deltaLon := 30.0 / (111.32 * math.Cos(latRad))

	minLat, maxLat := req.FromLat-deltaLat, req.FromLat+deltaLat
	minLon, maxLon := req.FromLon-deltaLon, req.FromLon+deltaLon

	nearADriver := domain.NearADriver{
		MinLat:  minLat,
		MaxLat:  maxLat,
		MinLong: minLon,
		MaxLong: maxLon,
	}

	nearDrivers, err := h.driverRepo.GetDriverNearA(ctx, nearADriver, req)
	if err != nil {
		h.logger.Error("NO DRIVERS", zap.Error(err))
		return
	}

	text := buildKZOrderText(req)
	replyMarkup := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Қабылдау", CallbackData: fmt.Sprintf("accept:%s", req.ID)},
				{Text: "📍 Көру", CallbackData: fmt.Sprintf("view:%s", req.ID)},
			},
		},
	}

	ticker := time.NewTicker(60 * time.Millisecond)
	defer ticker.Stop()

	sent, failed := 0, 0

	for i := 0; i < len(nearDrivers); i++ {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			nearDriver := nearDrivers[i]

			if req.CargoPhoto != "" {
				p := strings.TrimSpace(req.CargoPhoto)
				if p != "" {
					file, err := os.Open(p)
					if err != nil {
						h.logger.Warn("open cargo photo", zap.String("path", req.CargoPhoto), zap.Error(err))
					} else {
						_, err = b.SendPhoto(ctx, &bot.SendPhotoParams{
							ChatID: nearDriver.TelegramID,
							Photo: &models.InputFileUpload{
								Filename: filepath.Base(p),
								Data:     file,
							},
							Caption:     text,
							ReplyMarkup: replyMarkup,
						})
						_ = file.Close()

						if err != nil {
							failed++
							h.logger.Warn("send to driver with photo", zap.Int64("tg_id", nearDriver.TelegramID), zap.Error(err))
							continue
						}
						sent++
						continue
					}
				}
			}

			_, err := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:      nearDriver.TelegramID,
				Text:        text,
				ReplyMarkup: replyMarkup,
			})
			if err != nil {
				failed++
				h.logger.Warn("send to driver", zap.Int64("tg_id", nearDriver.TelegramID), zap.Error(err))
				continue
			}
			sent++
		}
	}

	h.logger.Info("broadcast finished",
		zap.Int("candidates", len(nearDrivers)),
		zap.Int("sent", sent),
		zap.Int("failed", failed),
	)
}

func (h *Handler) sendConfirmationMessage(b *bot.Bot, req *domain.DeliveryRequest, requestID string) {
	if req.TelegramID == 0 {
		h.logger.Warn("No Telegram ID provided, skipping confirmation message")
		return
	}

	truckTypeText := h.formatTruckType(req.TruckType)
	timeText := h.formatTimeStart(req.TimeStart)

	message := fmt.Sprintf(`🚛 <b>Жеткізу тапсырысы берілді!</b>

📋 <b>Өтінім нөмірі:</b> %s

📍 <b>Қайдан:</b> %s
🎯 <b>Қайда:</b> %s

💰 <b>Бағасы:</b> %d ₸
🚚 <b>Көлік түрі:</b> %s
📱 <b>Байланыс:</b> %s

🛣️ <b>Қашықтық:</b> %.1f км
⏱️ <b>Болжамды уақыт:</b> %d мин
%s`,
		requestID,
		req.FromAddress,
		req.ToAddress,
		req.Price,
		truckTypeText,
		req.Contact,
		req.DistanceKm,
		req.EtaMin,
		timeText,
	)

	if req.Comment != "" {
		message += fmt.Sprintf("\n💬 <b>Түсініктеме:</b> %s", req.Comment)
	}
	message += "\n\n✅ Сіздің өтініміңіз жүргізушілерге жіберілді!"

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "📋 Менің тапсырыстарым", CallbackData: "my_orders"},
				{Text: "🔄 Жаңарту", CallbackData: fmt.Sprintf("refresh_order_%s", requestID)},
			},
		},
	}

	ctx := context.Background()

	if req.CargoPhoto != "" {
		file, err := os.Open(req.CargoPhoto)
		if err != nil {
			h.logger.Error("Failed to open cargo photo", zap.Error(err))
		} else {
			defer file.Close()
			_, err = b.SendPhoto(ctx, &bot.SendPhotoParams{
				ChatID: req.TelegramID,
				Photo: &models.InputFileUpload{
					Filename: filepath.Base(file.Name()),
					Data:     file,
				},
				Caption:     message,
				ParseMode:   models.ParseModeHTML,
				ReplyMarkup: keyboard,
			})
			if err == nil {
				return
			}
			h.logger.Error("Failed to send confirmation photo", zap.Error(err))
		}
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      req.TelegramID,
		Text:        message,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		h.logger.Error("Failed to send confirmation message", zap.Error(err))
	}
}

func (h *Handler) calculateRoute(fromLat, fromLon, toLat, toLon float64) (float64, int) {
	if fromLat == 0 || fromLon == 0 || toLat == 0 || toLon == 0 {
		return 10.0, 30
	}

	actualDistance, actualDuration := h.getOSRMRoute(fromLat, fromLon, toLat, toLon)
	if actualDistance > 0 {
		return actualDistance, actualDuration
	}

	straightDistance := h.haversineDistance(fromLat, fromLon, toLat, toLon)
	roadDistance := straightDistance * 1.3
	drivingTimeMinutes := int((roadDistance / 35.0) * 60)

	return roadDistance, drivingTimeMinutes
}

func (h *Handler) getOSRMRoute(fromLat, fromLon, toLat, toLon float64) (float64, int) {
	url := fmt.Sprintf("http://router.project-osrm.org/route/v1/driving/%f,%f;%f,%f?overview=false&steps=false",
		fromLon, fromLat, toLon, toLat)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		h.logger.Debug("OSRM request failed", zap.Error(err))
		return 0, 0
	}
	defer resp.Body.Close()

	var result struct {
		Routes []struct {
			Distance float64 `json:"distance"`
			Duration float64 `json:"duration"`
		} `json:"routes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		h.logger.Debug("OSRM response decode failed", zap.Error(err))
		return 0, 0
	}
	if len(result.Routes) == 0 {
		return 0, 0
	}

	return result.Routes[0].Distance / 1000.0, int(result.Routes[0].Duration / 60.0)
}

// =================================
// DRIVER MATCHING REQUEST HANDLER
// =================================

// handleDriverRequest handles POST requests for driver matching
func (h *Handler) handleDriverRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	h.logger.Info("🔍 Processing driver route matching request (POST)")

	var params DriverRequestParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		h.logger.Error("❌ Failed to parse request body", zap.Error(err))
		h.sendErrorResponse(w, "Неверные данные запроса", http.StatusBadRequest)
		return
	}

	if !h.isValidCoordinates(params.PickupLat, params.PickupLon) ||
		!h.isValidCoordinates(params.DropoffLat, params.DropoffLon) {
		h.sendErrorResponse(w, "Некорректные координаты", http.StatusBadRequest)
		return
	}

	if params.RadiusKm == 0 {
		params.RadiusKm = 50.0
	}

	drivers, err := h.findDriversByRouteMatching(
		params.PickupLat, params.PickupLon,
		params.DropoffLat, params.DropoffLon,
		params.RadiusKm, params.TruckType,
	)
	if err != nil {
		h.logger.Error("❌ Route matching failed", zap.Error(err))
		h.sendErrorResponse(w, "Ошибка поиска водителей", http.StatusInternalServerError)
		return
	}

	h.sendSuccessResponse(w, "Водители найдены", map[string]interface{}{
		"drivers": drivers,
		"count":   len(drivers),
	})
}

// =================================
// CORE ROUTE-TO-ROUTE MATCHING LOGIC
// =================================

// findDriversByRouteMatching finds drivers whose trip A→B matches client's route A→B
func (h *Handler) findDriversByRouteMatching(
	clientFromLat, clientFromLon, clientToLat, clientToLon, radiusKm float64, truckType string,
) ([]DriverWithTrip, error) {

	h.logger.Info("🔍 Starting ROUTE-TO-ROUTE matching",
		zap.Float64("client_from_lat", clientFromLat),
		zap.Float64("client_from_lon", clientFromLon),
		zap.Float64("client_to_lat", clientToLat),
		zap.Float64("client_to_lon", clientToLon),
		zap.Float64("radius_km", radiusKm),
		zap.String("truck_type", truckType))

	// Build query with optional truck_type filter
	baseQuery := `
		SELECT 
			d.id, d.telegram_id, d.first_name, d.last_name, 
			d.contact_number, d.profile_photo,
			dt.id, dt.from_address, dt.from_lat, dt.from_lon,
			dt.to_address, dt.to_lat, dt.to_lon,
			dt.price, dt.start_time, dt.comment, 
			dt.distance_km, dt.eta_min, dt.truck_type
		FROM drivers d
		INNER JOIN driver_trips dt ON d.id = dt.driver_id
		WHERE d.status = 'approved'
		  AND dt.status = 'active'
		  AND dt.from_lat IS NOT NULL AND dt.from_lat != 0
		  AND dt.from_lon IS NOT NULL AND dt.from_lon != 0
		  AND dt.to_lat IS NOT NULL AND dt.to_lat != 0
		  AND dt.to_lon IS NOT NULL AND dt.to_lon != 0
		  AND dt.from_address IS NOT NULL AND dt.from_address != ''
		  AND dt.to_address IS NOT NULL AND dt.to_address != ''
		  AND dt.created_at >= datetime('now', '-48 hours')
	`

	// Add truck_type filter if specified
	if truckType != "" && truckType != "any" {
		baseQuery += ` AND dt.truck_type = ?`
	}

	baseQuery += ` ORDER BY dt.created_at DESC LIMIT 200`

	var rows *sql.Rows
	var err error

	if truckType != "" && truckType != "any" {
		rows, err = h.db.Query(baseQuery, truckType)
	} else {
		rows, err = h.db.Query(baseQuery)
	}

	if err != nil {
		h.logger.Error("❌ Database query failed", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var matchedDrivers []DriverWithTrip
	scannedCount := 0
	matchedCount := 0

	for rows.Next() {
		var driver DriverWithTrip
		err := rows.Scan(
			&driver.ID, &driver.TelegramID, &driver.FirstName, &driver.LastName,
			&driver.ContactNumber, &driver.ProfilePhoto,
			&driver.TripID, &driver.FromAddress, &driver.FromLat, &driver.FromLon,
			&driver.ToAddress, &driver.ToLat, &driver.ToLon,
			&driver.Price, &driver.StartTime, &driver.Comment,
			&driver.DistanceKm, &driver.EtaMin, &driver.TruckType,
		)
		if err != nil {
			h.logger.Error("❌ Row scan error", zap.Error(err))
			continue
		}

		scannedCount++

		// Calculate distances using Go's haversine (not SQL)
		distancePickupToPickup := h.haversineDistance(
			clientFromLat, clientFromLon,
			driver.FromLat, driver.FromLon,
		)

		distanceDropoffToDropoff := h.haversineDistance(
			clientToLat, clientToLon,
			driver.ToLat, driver.ToLon,
		)

		// BOTH points must be within radius
		if distancePickupToPickup <= radiusKm && distanceDropoffToDropoff <= radiusKm {
			matchedCount++
			driver.DistanceToPickupKm = distancePickupToPickup
			driver.DistanceToDropoffKm = distanceDropoffToDropoff

			// Classify match quality
			if distancePickupToPickup <= 5 && distanceDropoffToDropoff <= 10 {
				driver.MatchQuality = "perfect"
			} else if distancePickupToPickup <= 15 && distanceDropoffToDropoff <= 25 {
				driver.MatchQuality = "good"
			} else {
				driver.MatchQuality = "fair"
			}

			matchedDrivers = append(matchedDrivers, driver)

			h.logger.Debug("✅ Driver matched",
				zap.String("driver_id", driver.ID),
				zap.String("name", driver.FirstName+" "+driver.LastName),
				zap.Float64("pickup_distance_km", distancePickupToPickup),
				zap.Float64("dropoff_distance_km", distanceDropoffToDropoff),
				zap.String("quality", driver.MatchQuality))
		} else {
			h.logger.Debug("❌ Driver filtered out",
				zap.String("driver_id", driver.ID),
				zap.Float64("pickup_distance_km", distancePickupToPickup),
				zap.Float64("dropoff_distance_km", distanceDropoffToDropoff),
				zap.Float64("max_radius_km", radiusKm))
		}
	}

	// Sort by combined distance score
	sort.Slice(matchedDrivers, func(i, j int) bool {
		scoreI := matchedDrivers[i].DistanceToPickupKm + matchedDrivers[i].DistanceToDropoffKm*0.5
		scoreJ := matchedDrivers[j].DistanceToPickupKm + matchedDrivers[j].DistanceToDropoffKm*0.5
		return scoreI < scoreJ
	})

	h.logger.Info("🎯 Route-to-route matching completed",
		zap.Int("scanned_count", scannedCount),
		zap.Int("matched_count", matchedCount),
		zap.Float64("radius_km", radiusKm))

	return matchedDrivers, nil
}

// isValidCoordinates checks if coordinates are valid
func (h *Handler) isValidCoordinates(lat, lon float64) bool {
	// Kazakhstan bounds approximately
	return lat >= 40.0 && lat <= 56.0 && lon >= 46.0 && lon <= 88.0
}

func (h *Handler) calculateRouteMatchingStats(drivers []DriverWithTrip) SearchStats {
	if len(drivers) == 0 {
		return SearchStats{}
	}

	var totalPickupDistance float64
	var perfectMatches, goodMatches int

	for _, driver := range drivers {
		totalPickupDistance += driver.DistanceToPickupKm
		switch driver.MatchQuality {
		case "perfect":
			perfectMatches++
		case "good":
			goodMatches++
		}
	}

	return SearchStats{
		AvgDistance:    totalPickupDistance / float64(len(drivers)),
		PerfectMatches: perfectMatches,
		GoodMatches:    goodMatches,
	}
}

// =================================
// GEO UTILITY FUNCTIONS
// =================================

func (h *Handler) extractCityFromAddress(address string) string {
	addr := strings.TrimSpace(address)
	if addr == "" || addr == "Адрес не указан" {
		return "Неизвестный город"
	}

	cities := []string{
		"Алматы", "Almaty",
		"Астана", "Нур-Султан", "Nur-Sultan", "Astana",
		"Шымкент", "Shymkent",
		"Караганда", "Karaganda",
		"Актобе", "Aktobe", "Ақтөбе",
		"Тараз", "Taraz",
		"Павлодар", "Pavlodar",
		"Усть-Каменогорск", "Ust-Kamenogorsk", "Өскемен",
		"Семей", "Semey",
		"Атырау", "Atyrau",
		"Костанай", "Kostanay",
		"Петропавловск", "Petropavl",
		"Актау", "Aktau", "Ақтау",
	}

	up := strings.ToUpper(addr)

	for _, c := range cities {
		cu := strings.ToUpper(c)
		if strings.Contains(up, cu) {
			if strings.Contains(cu, "ALMA") || cu == "АЛМАТЫ" {
				return "Алматы"
			}
			if strings.Contains(cu, "ASTAN") || strings.Contains(cu, "НУР-СУЛТАН") || cu == "АСТАНА" {
				return "Астана"
			}
			if strings.Contains(cu, "SHYM") || cu == "ШЫМКЕНТ" {
				return "Шымкент"
			}
			return c
		}
	}

	parts := strings.Split(addr, ",")
	if len(parts) > 0 {
		first := strings.TrimSpace(parts[0])
		if first != "" && len(first) < 50 {
			first = strings.TrimSpace(strings.TrimPrefix(first, "г."))
			first = strings.TrimSpace(strings.TrimPrefix(first, "Г."))
			if first != "" {
				return first
			}
		}
	}

	if len(addr) > 30 {
		return addr[:27] + "..."
	}
	return addr
}

// =================================
// RESPONSE HELPERS
// =================================

func (h *Handler) sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(Response{
		Success: false,
		Message: message,
	})
}

func (h *Handler) sendSuccessResponse(w http.ResponseWriter, message string, data ...interface{}) {
	response := Response{
		Success: true,
		Message: message,
	}
	if len(data) > 0 {
		response.Data = data[0]
	}
	_ = json.NewEncoder(w).Encode(response)
}

// =================================
// TEXT FORMATTING HELPERS
// =================================

func (h *Handler) formatTruckType(truckType string) string {
	types := map[string]string{
		"small":        "Кіші көлік (до 1.5т) | Малый (до 1.5т)",
		"medium":       "Орташа көлік (до 5т) | Средний (до 5т)",
		"large":        "Үлкен көлік (до 20т) | Большой (до 20т)",
		"refrigerator": "Рефрижератор | Рефрижератор",
		"tow":          "Эвакуатор | Эвакуатор",
		"any":          "Кез келген көлік | Любой транспорт",
		"":             "Көрсетілмеген | Не указан",
	}
	if formatted, exists := types[truckType]; exists {
		return formatted
	}
	return fmt.Sprintf("%s | %s", truckType, truckType)
}

func (h *Handler) formatTimeStart(timeStart string) string {
	if timeStart == "" {
		return "🕐 <b>Кету уақыты:</b> Қазір | Сейчас"
	}

	t, err := time.Parse("2006-01-02T15:04", timeStart)
	if err != nil {
		if t2, err2 := time.Parse("2006-01-02 15:04", timeStart); err2 == nil {
			t = t2
		} else {
			return fmt.Sprintf("🕐 <b>Кету уақыты:</b> %s", timeStart)
		}
	}

	now := time.Now()
	if t.Format("2006-01-02") == now.Format("2006-01-02") {
		return fmt.Sprintf("🕐 <b>Кету уақыты:</b> Бүгін %s | Сегодня %s",
			t.Format("15:04"), t.Format("15:04"))
	}

	if t.After(now) && t.Before(now.Add(24*time.Hour)) && t.Day() != now.Day() {
		return fmt.Sprintf("🕐 <b>Кету уақыты:</b> Ертең %s | Завтра %s",
			t.Format("15:04"), t.Format("15:04"))
	}

	return fmt.Sprintf("🕐 <b>Кету уақыты:</b> %s %s",
		t.Format("02.01.2006"), t.Format("15:04"))
}

func buildKZOrderText(r *domain.DeliveryRequest) string {
	comment := r.Comment
	if comment == "" {
		comment = "—"
	}

	return fmt.Sprintf(
		`Жаңа тапсырыс!
Бастау: %s
Мекенжай: %s
Қашықтық: %.1f км
ETA: %d мин
Тасымал түрі: %s
Байланыс: %s
Баға: %d ₸
Ескерту: %s

Қабылдағыңыз келсе, төмендегі «Қабылдау» батырмасын басыңыз.`,
		nonEmpty(r.FromAddress, "A нүктесі"),
		nonEmpty(r.ToAddress, "B нүктесі"),
		r.DistanceKm, r.EtaMin,
		nonEmpty(r.TruckType, "кез келген"),
		r.Contact, r.Price, comment,
	)
}

func nonEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func nullableString(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return sql.NullString{Valid: false}
	}
	return s
}
