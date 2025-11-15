// admin-panel-handler.go
package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// ===== ADMIN API =====

func (h *Handler) isAdmin(telegram_id int64) bool {
	return h.cfg.AdminTelegramID == telegram_id
}

// Admin summary stats: drivers + orders (for charts)
func (h *Handler) handleAdminSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Получаем Telegram ID из query или тела (на фронте мы будем слать)
	var telegramID int64
	if r.Method == "GET" {
		if v := r.URL.Query().Get("telegram_id"); v != "" {
			id, _ := strconv.ParseInt(v, 10, 64)
			telegramID = id
		}
	} else if r.Method == "POST" {
		var req struct {
			TelegramID int64 `json:"telegram_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		telegramID = req.TelegramID
	}

	if !h.isAdmin(telegramID) {
		h.sendErrorResponse(w, "Рұқсат жоқ / Нет доступа", http.StatusForbidden)
		return
	}

	// Общая статистика по водителям
	var totalDrivers, pendingDrivers, approvedDrivers, rejectedDrivers int
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM drivers`).Scan(&totalDrivers)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM drivers WHERE status='pending'`).Scan(&pendingDrivers)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM drivers WHERE status='approved'`).Scan(&approvedDrivers)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM drivers WHERE status='rejected'`).Scan(&rejectedDrivers)

	// Статистика по заказам (delivery_requests)
	var totalOrders, pendingOrders, matchedOrders, inProgressOrders, completedOrders, cancelledOrders int
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM delivery_requests`).Scan(&totalOrders)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM delivery_requests WHERE status='pending'`).Scan(&pendingOrders)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM delivery_requests WHERE status='matched'`).Scan(&matchedOrders)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM delivery_requests WHERE status='in_progress'`).Scan(&inProgressOrders)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM delivery_requests WHERE status='completed'`).Scan(&completedOrders)
	_ = h.db.QueryRow(`SELECT COUNT(*) FROM delivery_requests WHERE status='cancelled'`).Scan(&cancelledOrders)

	// График: количество заказов по дням за последнюю неделю
	type dayStat struct {
		Date  string `json:"date"`
		Count int    `json:"count"`
	}
	rows, err := h.db.Query(`
		SELECT strftime('%Y-%m-%d', created_at) as d, COUNT(*) 
		FROM delivery_requests
		WHERE created_at >= datetime('now', '-7 days')
		GROUP BY d
		ORDER BY d ASC`,
	)
	if err != nil {
		h.logger.Error("Failed to query orders by day", zap.Error(err))
	}

	var ordersByDay []dayStat
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var s dayStat
			if err := rows.Scan(&s.Date, &s.Count); err == nil {
				ordersByDay = append(ordersByDay, s)
			}
		}
	}

	// График: количество новых водителей по дням за последнюю неделю
	rows2, err2 := h.db.Query(`
		SELECT strftime('%Y-%m-%d', created_at) as d, COUNT(*) 
		FROM drivers
		WHERE created_at >= datetime('now', '-7 days')
		GROUP BY d
		ORDER BY d ASC`,
	)
	var driversByDay []dayStat
	if err2 != nil {
		h.logger.Error("Failed to query drivers by day", zap.Error(err2))
	} else {
		defer rows2.Close()
		for rows2.Next() {
			var s dayStat
			if err := rows2.Scan(&s.Date, &s.Count); err == nil {
				driversByDay = append(driversByDay, s)
			}
		}
	}

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

// Admin: list drivers (with basic info + coords)
func (h *Handler) handleAdminDrivers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var telegramID int64
	if v := r.URL.Query().Get("telegram_id"); v != "" {
		id, _ := strconv.ParseInt(v, 10, 64)
		telegramID = id
	}
	if !h.isAdmin(telegramID) {
		h.sendErrorResponse(w, "Рұқсат жоқ / Нет доступа", http.StatusForbidden)
		return
	}

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

	type adminDriver struct {
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
	}

	var drivers []adminDriver
	for rows.Next() {
		var d adminDriver
		var createdAt, updatedAt time.Time
		if err := rows.Scan(
			&d.ID, &d.TelegramID, &d.FirstName, &d.LastName, &d.Birthday,
			&d.ContactNumber, &d.StartCity, &d.Latitude, &d.Longitude,
			&d.ProfilePhoto, &d.LicenseFront, &d.LicenseBack,
			&d.TruckType, &d.IsVerified, &d.Status, &createdAt, &updatedAt,
		); err != nil {
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

// Admin: driver details by ID
func (h *Handler) handleAdminDriverDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var telegramID int64
	if v := r.URL.Query().Get("telegram_id"); v != "" {
		id, _ := strconv.ParseInt(v, 10, 64)
		telegramID = id
	}
	if !h.isAdmin(telegramID) {
		h.sendErrorResponse(w, "Рұқсат жоқ / Нет доступа", http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	driverID := vars["id"]
	if driverID == "" {
		h.sendErrorResponse(w, "Driver ID қажет / нужен", http.StatusBadRequest)
		return
	}

	query := `
		SELECT 
			id, telegram_id, first_name, last_name, birthday,
			contact_number, start_city, latitude, longitude,
			profile_photo, license_front, license_back,
			truck_type, is_verified, status, created_at, updated_at, approved_at, approved_by
		FROM drivers
		WHERE id = ?`

	var d struct {
		ID            string     `json:"id"`
		TelegramID    int64      `json:"telegram_id"`
		FirstName     string     `json:"first_name"`
		LastName      string     `json:"last_name"`
		Birthday      string     `json:"birthday"`
		ContactNumber string     `json:"contact_number"`
		StartCity     string     `json:"start_city"`
		Latitude      float64    `json:"latitude"`
		Longitude     float64    `json:"longitude"`
		ProfilePhoto  string     `json:"profile_photo"`
		LicenseFront  string     `json:"license_front"`
		LicenseBack   string     `json:"license_back"`
		TruckType     string     `json:"truck_type"`
		IsVerified    bool       `json:"is_verified"`
		Status        string     `json:"status"`
		CreatedAt     time.Time  `json:"-"`
		UpdatedAt     time.Time  `json:"-"`
		ApprovedAt    *time.Time `json:"-"`
		ApprovedBy    *string    `json:"-"`
	}

	err := h.db.QueryRow(query, driverID).Scan(
		&d.ID, &d.TelegramID, &d.FirstName, &d.LastName, &d.Birthday,
		&d.ContactNumber, &d.StartCity, &d.Latitude, &d.Longitude,
		&d.ProfilePhoto, &d.LicenseFront, &d.LicenseBack,
		&d.TruckType, &d.IsVerified, &d.Status, &d.CreatedAt, &d.UpdatedAt, &d.ApprovedAt, &d.ApprovedBy,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			h.sendErrorResponse(w, "Driver табылмады / не найден", http.StatusNotFound)
			return
		}
		h.sendErrorResponse(w, "Қате / Ошибка", http.StatusInternalServerError)
		return
	}

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
		"created_at":     d.CreatedAt.Format(time.RFC3339),
		"updated_at":     d.UpdatedAt.Format(time.RFC3339),
	}

	if d.ApprovedAt != nil {
		resp["approved_at"] = d.ApprovedAt.Format(time.RFC3339)
	}
	if d.ApprovedBy != nil {
		resp["approved_by"] = *d.ApprovedBy
	}

	h.sendSuccessResponse(w, "Driver detail", resp)
}

// Admin: orders list
func (h *Handler) handleAdminOrders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var telegramID int64
	if v := r.URL.Query().Get("telegram_id"); v != "" {
		id, _ := strconv.ParseInt(v, 10, 64)
		telegramID = id
	}
	if !h.isAdmin(telegramID) {
		h.sendErrorResponse(w, "Рұқсат жоқ / Нет доступа", http.StatusForbidden)
		return
	}

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

	type adminOrder struct {
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

	var orders []adminOrder
	for rows.Next() {
		var o adminOrder
		var createdAt, updatedAt time.Time
		if err := rows.Scan(
			&o.ID, &o.TelegramID, &o.FromAddress, &o.FromLat, &o.FromLon,
			&o.ToAddress, &o.ToLat, &o.ToLon, &o.DistanceKm, &o.EtaMin,
			&o.Price, &o.TruckType, &o.Contact, &o.TimeStart, &o.Comment,
			&o.ItemPhoto, &o.Status, &createdAt, &updatedAt,
		); err != nil {
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
