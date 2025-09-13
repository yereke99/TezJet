package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// DriverTrip represents the driver trip data
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

// Enhanced DriverWithTrip for client matching
type DriverWithTrip struct {
	// Driver info
	ID            string `json:"id"`
	TelegramID    int64  `json:"telegram_id"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	FullName      string `json:"full_name"` // Computed field
	ContactNumber string `json:"contact_number"`
	Contact       string `json:"contact"` // Alias for ContactNumber
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

	// Matching scores (computed during search)
	DistanceToPickupKm  float64 `json:"distance_to_pickup_km,omitempty"`
	DistanceToDropoffKm float64 `json:"distance_to_dropoff_km,omitempty"`
	RouteMatchScore     int     `json:"route_match_score,omitempty"`
	MatchQuality        string  `json:"match_quality,omitempty"`
	ETAMin              int     `json:"eta_min_pickup,omitempty"`
	IsOnline            bool    `json:"is_online,omitempty"`
	LastSeenMin         int     `json:"last_seen_min,omitempty"`
	ResponseTimeMin     int     `json:"response_time_min,omitempty"`
}

// DeliveryRequest represents the delivery request data
type DeliveryRequest struct {
	ID          string    `json:"id"`
	FromAddress string    `json:"from_address"`
	FromLat     float64   `json:"from_lat"`
	FromLon     float64   `json:"from_lon"`
	ToAddress   string    `json:"to_address"`
	ToLat       float64   `json:"to_lat"`
	ToLon       float64   `json:"to_lon"`
	DistanceKm  float64   `json:"distance_km"`
	EtaMin      int       `json:"eta_min"`
	Price       int       `json:"price"`
	TruckType   string    `json:"truck_type"`
	Contact     string    `json:"contact"`
	TimeStart   string    `json:"time_start"`
	Comment     string    `json:"comment"`
	TelegramID  int64     `json:"telegram_id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// DeliveryListRequest represents the request for getting delivery orders
type DeliveryListRequest struct {
	TelegramID int64   `json:"telegram_id"`
	DriverLat  float64 `json:"driver_lat"`
	DriverLon  float64 `json:"driver_lon"`
	Radius     float64 `json:"radius"` // in kilometers
}

// DeliveryListResponse represents the response with orders
type DeliveryListResponse struct {
	Orders      []DeliveryRequest `json:"orders"`
	TotalCount  int               `json:"total_count"`
	NearbyCount int               `json:"nearby_count"`
	AvgPrice    float64           `json:"avg_price"`
}

// DriverRegistration represents the driver registration data
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
	IsVerified    bool      `json:"is_verified"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// DriverRequestParams represents the request parameters for finding drivers
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

// Enhanced response structures
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

// ===== CORE DRIVER MATCHING SYSTEM =====

// Updated handleDriverRequest to use route matching
func (h *Handler) handleDriverRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	h.logger.Info("🔍 Processing driver route matching request",
		zap.String("method", r.Method))

	// Parse request parameters
	var params DriverRequestParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		h.logger.Error("❌ Failed to parse request body", zap.Error(err))
		h.sendErrorResponse(w, "Неверные данные запроса", http.StatusBadRequest)
		return
	}

	// Validate coordinates
	if !h.isValidCoordinates(params.PickupLat, params.PickupLon) ||
		!h.isValidCoordinates(params.DropoffLat, params.DropoffLon) {
		h.logger.Error("❌ Invalid coordinates provided")
		h.sendErrorResponse(w, "Некорректные координаты", http.StatusBadRequest)
		return
	}

	// Force 50km radius as requested
	params.RadiusKm = 50.0

	h.logger.Info("📊 Route matching parameters",
		zap.String("request_id", params.RequestID),
		zap.Float64("client_pickup_lat", params.PickupLat),
		zap.Float64("client_pickup_lon", params.PickupLon),
		zap.Float64("client_dropoff_lat", params.DropoffLat),
		zap.Float64("client_dropoff_lon", params.DropoffLon),
		zap.Float64("radius_km", params.RadiusKm),
		zap.String("truck_type", params.TruckType))

	// Execute route-to-route matching
	startTime := time.Now()

	drivers, err := h.findDriversByRouteMatching(
		params.PickupLat, params.PickupLon, 
		params.DropoffLat, params.DropoffLon,
		params.RadiusKm, params.TruckType)
	if err != nil {
		h.logger.Error("❌ Route matching failed", zap.Error(err))
		h.sendErrorResponse(w, "Ошибка поиска водителей по маршруту", http.StatusInternalServerError)
		return
	}

	searchDuration := time.Since(startTime)

	// Calculate statistics
	stats := h.calculateRouteMatchingStats(drivers)

	response := DriverMatchResponse{
		Drivers:        drivers,
		TotalCount:     len(drivers),
		AvgDistance:    stats.AvgDistance,
		SearchTime:     searchDuration.String(),
		PerfectMatches: stats.PerfectMatches,
		GoodMatches:    stats.GoodMatches,
		SearchRadius:   params.RadiusKm,
		MatchingCriteria: map[string]interface{}{
			"matching_type":        "route_to_route",
			"radius_km":            params.RadiusKm,
			"description":          "A→B driver route matches A→B client route within radius",
			"truck_type_filter":    params.TruckType != "" && params.TruckType != "any",
			"distance_calculation": "haversine_go_dual_point",
		},
	}

	h.logger.Info("✅ Route matching completed successfully",
		zap.Int("route_matches_found", len(drivers)),
		zap.Int("perfect_matches", stats.PerfectMatches),
		zap.Int("good_matches", stats.GoodMatches),
		zap.Float64("avg_pickup_distance_km", stats.AvgDistance),
		zap.String("search_duration", searchDuration.String()))

	h.sendSuccessResponse(w, "Водители по маршруту найдены успешно", response)
}



// Calculate statistics for route matching
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


// Helper function to calculate search statistics for Go-calculated distances
func (h *Handler) calculateSearchStatsForGoCalculation(drivers []DriverWithTrip, params DriverRequestParams) SearchStats {
	if len(drivers) == 0 {
		return SearchStats{}
	}

	var totalDistance float64
	var perfectMatches, goodMatches int

	for _, driver := range drivers {
		totalDistance += driver.DistanceKm

		// Calculate dropoff distance for match quality assessment
		dropoffDistance := h.haversineDistance(params.DropoffLat, params.DropoffLon, driver.ToLat, driver.ToLon)

		// Determine match quality
		if driver.DistanceKm <= 2.0 && dropoffDistance <= 5.0 {
			perfectMatches++
		} else if driver.DistanceKm <= 5.0 && dropoffDistance <= 10.0 {
			goodMatches++
		}
	}

	return SearchStats{
		AvgDistance:    totalDistance / float64(len(drivers)),
		PerfectMatches: perfectMatches,
		GoodMatches:    goodMatches,
	}
}

// Fixed driver matching function with proper city names and distance filtering
func (h *Handler) findMatchingDriversFixed(params DriverRequestParams) ([]DriverWithTrip, error) {
	h.logger.Info("🔍 Finding drivers within radius",
		zap.Float64("pickup_lat", params.PickupLat),
		zap.Float64("pickup_lon", params.PickupLon),
		zap.Float64("radius_km", params.RadiusKm),
		zap.String("truck_type", params.TruckType))

	// SQL query with distance calculation and address fields
	const query = `
SELECT
  d.id,
  d.telegram_id,
  d.first_name,
  d.last_name,
  d.contact_number,
  d.profile_photo,
  d.truck_type,
  d.is_verified,
  d.status,
  
  -- Driver trip data with addresses
  dt.from_address,
  dt.to_address,
  CAST(dt.from_lat AS REAL) as from_lat,
  CAST(dt.from_lon AS REAL) as from_lon,
  CAST(dt.to_lat AS REAL) as to_lat,
  CAST(dt.to_lon AS REAL) as to_lon,
  dt.price,
  dt.departure_time,
  dt.comment,
  dt.has_whatsapp,
  dt.has_telegram,
  dt.telegram_username,
  dt.created_at,
  
  -- Distance calculation using Haversine formula
  (
    6371 * acos(
      cos(radians(?)) * cos(radians(CAST(dt.from_lat AS REAL))) *
      cos(radians(CAST(dt.from_lon AS REAL)) - radians(?)) +
      sin(radians(?)) * sin(radians(CAST(dt.from_lat AS REAL)))
    )
  ) AS distance_km

FROM drivers d
JOIN driver_trips dt ON d.id = dt.driver_id
WHERE 
  1=1  -- Always true base condition
  
  -- Status filter: only approved drivers
  AND LOWER(d.status) = 'approved' 
  
  -- Data quality filters
  AND dt.from_lat IS NOT NULL 
  AND dt.from_lon IS NOT NULL
  AND dt.from_address IS NOT NULL
  AND dt.from_address != ''
  AND dt.from_address != 'Адрес не указан'
  
  -- Distance filter: within specified radius
  AND (
    6371 * acos(
      cos(radians(?)) * cos(radians(CAST(dt.from_lat AS REAL))) *
      cos(radians(CAST(dt.from_lon AS REAL)) - radians(?)) +
      sin(radians(?)) * sin(radians(CAST(dt.from_lat AS REAL)))
    )
  ) <= ?
  
  -- Truck type filter (if specified and not 'any')
  AND (? = 'any' OR LOWER(d.truck_type) = LOWER(?))

ORDER BY distance_km ASC, dt.created_at DESC
LIMIT 200;
`

	// Execute query with parameters for distance calculation
	rows, err := h.db.Query(query,
		params.PickupLat, // for distance calculation in SELECT
		params.PickupLon,
		params.PickupLat,
		params.PickupLat, // for distance calculation in WHERE
		params.PickupLon,
		params.PickupLat,
		params.RadiusKm,
		params.TruckType, // for truck type filter
		params.TruckType,
	)
	if err != nil {
		h.logger.Error("❌ Database query failed", zap.Error(err))
		return nil, fmt.Errorf("database query failed: %w", err)
	}
	defer rows.Close()

	var results []DriverWithTrip
	seenDrivers := make(map[string]bool) // Prevent duplicate drivers

	for rows.Next() {
		var (
			driver          DriverWithTrip
			driverTruckType sql.NullString
			isVerifiedB     sql.NullBool
			status          sql.NullString
			fromAddress     sql.NullString
			toAddress       sql.NullString
			fromLatN        sql.NullFloat64
			fromLonN        sql.NullFloat64
			toLatN          sql.NullFloat64
			toLonN          sql.NullFloat64
			price           sql.NullInt64
			departureTime   sql.NullString
			comment         sql.NullString
			hasWhatsAppB    sql.NullBool
			hasTelegramB    sql.NullBool
			tgUsername      sql.NullString
			createdAt       sql.NullString
			distanceKm      sql.NullFloat64
		)

		if err := rows.Scan(
			&driver.ID,
			&driver.TelegramID,
			&driver.FirstName,
			&driver.LastName,
			&driver.ContactNumber,
			&driver.ProfilePhoto,
			&driverTruckType,
			&isVerifiedB,
			&status,
			&fromAddress,
			&toAddress,
			&fromLatN,
			&fromLonN,
			&toLatN,
			&toLonN,
			&price,
			&departureTime,
			&comment,
			&hasWhatsAppB,
			&hasTelegramB,
			&tgUsername,
			&createdAt,
			&distanceKm,
		); err != nil {
			h.logger.Warn("⚠️ Failed to scan driver row", zap.Error(err))
			continue
		}

		// Skip duplicate drivers (only take the most recent trip per driver)
		if seenDrivers[driver.ID] {
			continue
		}
		seenDrivers[driver.ID] = true

		// Skip inactive drivers
		if status.Valid && strings.ToLower(status.String) != "approved" {
			continue
		}

		// Set addresses - KEY FIX: Extract real city names
		driver.FromAddress = "Адрес не указан"
		if fromAddress.Valid && strings.TrimSpace(fromAddress.String) != "" {
			driver.FromAddress = strings.TrimSpace(fromAddress.String)
		}

		driver.ToAddress = "Адрес не указан"
		if toAddress.Valid && strings.TrimSpace(toAddress.String) != "" {
			driver.ToAddress = strings.TrimSpace(toAddress.String)
		}

		// Extract city names from addresses
		driver.StartCity = h.extractCityFromAddress(driver.FromAddress)
		driver.EndCity = h.extractCityFromAddress(driver.ToAddress)

		// Set coordinates
		if fromLatN.Valid && fromLonN.Valid {
			driver.FromLat = fromLatN.Float64
			driver.FromLon = fromLonN.Float64
		}
		if toLatN.Valid && toLonN.Valid {
			driver.ToLat = toLatN.Float64
			driver.ToLon = toLonN.Float64
		}

		// Truck type with fallback
		driver.TruckType = "any"
		if driverTruckType.Valid && driverTruckType.String != "" {
			driver.TruckType = strings.ToLower(driverTruckType.String)
		}

		// Basic driver info
		driver.IsVerified = isVerifiedB.Valid && isVerifiedB.Bool
		driver.FullName = strings.TrimSpace(driver.FirstName + " " + driver.LastName)
		driver.Contact = driver.ContactNumber

		// Price with validation
		if price.Valid && price.Int64 > 0 {
			driver.Price = int(price.Int64)
		}

		// Departure time - keep as string for simplicity
		if departureTime.Valid && strings.TrimSpace(departureTime.String) != "" {
			driver.DepartureTime = strings.TrimSpace(departureTime.String)
		} else {
			// Set default departure time if not provided
			driver.DepartureTime = time.Now().Add(time.Hour).Format("2006-01-02 15:04:05")
		}

		// Comments
		if comment.Valid && comment.String != "" {
			driver.Comment = strings.TrimSpace(comment.String)
		}

		// Contact preferences
		driver.HasWhatsApp = hasWhatsAppB.Valid && hasWhatsAppB.Bool
		driver.HasTelegram = hasTelegramB.Valid && hasTelegramB.Bool
		if tgUsername.Valid && tgUsername.String != "" {
			driver.TelegramUsername = strings.TrimSpace(tgUsername.String)
		}

		// Distance and ETA calculations
		if distanceKm.Valid {
			driver.DistanceKm = distanceKm.Float64
			// ETA estimation: 2 minutes per km + 5 minutes base time
			driver.ETAMin = int(distanceKm.Float64*2) + 5

			// Cap ETA at reasonable limits
			if driver.ETAMin > 120 {
				driver.ETAMin = 120
			}
			if driver.ETAMin < 5 {
				driver.ETAMin = 5
			}
		}

		results = append(results, driver)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	// Log results summary
	h.logger.Info("✅ Found matching drivers",
		zap.Int("total_found", len(results)),
		zap.Float64("within_km", params.RadiusKm),
		zap.String("truck_type", params.TruckType))

	// Log some details about found drivers for debugging
	if len(results) > 0 {
		h.logger.Info("📊 Driver search results summary",
			zap.Float64("closest_distance", results[0].DistanceKm),
			zap.Float64("farthest_distance", results[len(results)-1].DistanceKm),
			zap.String("closest_driver", results[0].FullName),
			zap.String("closest_start_city", results[0].StartCity))
	}

	return results, nil
}

// Helper function to extract city name from full address
func (h *Handler) extractCityFromAddress(address string) string {
	if address == "" || address == "Адрес не указан" {
		return "Неизвестный город"
	}

	// Common Kazakhstan cities (in both Russian and English)
	cities := []string{
		"Алматы", "Almaty",
		"Астана", "Нур-Султан", "Nur-Sultan", "Astana",
		"Шымкент", "Shymkent",
		"Караганда", "Karaganda",
		"Актобе", "Aktobe", "Ақтөбе",
		"Тараз", "Taraz",
		"Павлодар", "Pavlodar",
		"Усть-Каменогорск", "Ust-Kamenogorsk", "Өскемен",
		"Семей", "Semey", "Семипалатинск",
		"Атырау", "Atyrau",
		"Костанай", "Kostanay",
		"Кызылорда", "Kyzylorda",
		"Уральск", "Uralsk", "Орал",
		"Петропавловск", "Petropavl", "Петропавл",
		"Актау", "Aktau", "Ақтау",
		"Темиртау", "Temirtau",
		"Турксиб", "Turksib",
		"Рудный", "Rudny",
		"Талдыкорган", "Taldykorgan",
		"Жезказган", "Zhezkazgan",
		"Балхаш", "Balkhash",
		"Экибастуз", "Ekibastuz",
		"Капчагай", "Kapchagay",
	}

	addressUpper := strings.ToUpper(address)

	// Try to find exact city match
	for _, city := range cities {
		cityUpper := strings.ToUpper(city)
		if strings.Contains(addressUpper, cityUpper) {
			// Return the original case version
			if strings.Contains(cityUpper, "ALMA") {
				return "Алматы"
			} else if strings.Contains(cityUpper, "ASTAN") || strings.Contains(cityUpper, "НУР-СУЛТАН") {
				return "Нур-Султан"
			} else if strings.Contains(cityUpper, "SHYM") {
				return "Шымкент"
			} else if strings.Contains(cityUpper, "KARAG") {
				return "Караганда"
			} else if strings.Contains(cityUpper, "AKTOBE") {
				return "Актобе"
			}
			return city
		}
	}

	// Try to extract city from structured address patterns
	// Pattern: "City, Region, Kazakhstan" or "City, Street, etc."
	parts := strings.Split(address, ",")
	if len(parts) >= 2 {
		// Try first part (usually city)
		firstPart := strings.TrimSpace(parts[0])
		if len(firstPart) > 2 && len(firstPart) < 50 {
			// Check if it looks like a city name (not a street number or building)
			if !strings.ContainsAny(firstPart, "0123456789") ||
				strings.Contains(strings.ToUpper(firstPart), "ГОРОД") ||
				strings.Contains(strings.ToUpper(firstPart), "Г.") {
				return firstPart
			}
		}

		// Try second part if first didn't work
		if len(parts) > 1 {
			secondPart := strings.TrimSpace(parts[1])
			if len(secondPart) > 2 && len(secondPart) < 50 {
				return secondPart
			}
		}
	}

	// Last resort: return shortened address
	if len(address) > 25 {
		return address[:22] + "..."
	}

	return address
}

// passesDriverFilters applies business logic filters
func (h *Handler) passesDriverFilters(driver DriverWithTrip, params DriverRequestParams) bool {
	// Truck type filter
	if params.TruckType != "" && params.TruckType != "any" {
		if driver.TruckType != params.TruckType && driver.TruckType != "any" {
			return false
		}
	}

	// Price compatibility filter (within 50% range)
	if params.PriceRange > 0 {
		priceDiff := math.Abs(float64(driver.Price - params.PriceRange))
		maxPriceDiff := float64(params.PriceRange) * 0.5
		if priceDiff > maxPriceDiff {
			return false
		}
	}

	if !isDepartureInFuture(driver.DepartureTime) {
		return false // Skip this driver, departure time has passed
	}

	return true
}

// Time filter - driver should depart in the future
func isDepartureInFuture(departureTimeStr string) bool {
	if departureTimeStr == "" {
		return true // If no departure time set, assume it's valid
	}

	// Try to parse the departure time string with multiple formats
	timeFormats := []string{
		"2006-01-02 15:04:05",        // Standard SQL datetime
		"2006-01-02T15:04:05Z",       // RFC3339 with Z
		"2006-01-02T15:04:05",        // RFC3339 without Z
		"2006-01-02T15:04:05.000Z",   // With milliseconds
		"2006-01-02 15:04:05.000000", // SQL with microseconds
		time.RFC3339,                 // Standard RFC3339
		time.DateTime,                // Go 1.20+ standard format
	}

	var departureTime time.Time
	var parseErr error

	for _, format := range timeFormats {
		if departureTime, parseErr = time.Parse(format, departureTimeStr); parseErr == nil {
			break
		}
	}

	// If all parsing attempts failed, log warning and assume future departure
	if parseErr != nil {
		// You can log this warning if needed:
		// h.logger.Warn("Failed to parse departure time", zap.String("departure_time", departureTimeStr), zap.Error(parseErr))
		return true // Assume valid if we can't parse it
	}

	// Check if departure time is in the future (with 5 minute buffer)
	return time.Now().Add(-5 * time.Minute).Before(departureTime)
}

// rankAndEnhanceDrivers sorts drivers and adds computed fields
func (h *Handler) rankAndEnhanceDrivers(drivers []DriverWithTrip, params DriverRequestParams) []DriverWithTrip {
	for i := range drivers {
		driver := &drivers[i]

		// Calculate route match score
		pickupScore := h.calculateProximityScore(driver.DistanceToPickupKm)
		dropoffScore := h.calculateProximityScore(driver.DistanceToDropoffKm)
		driver.RouteMatchScore = int(pickupScore*0.7 + dropoffScore*0.3)

		// Determine match quality
		if driver.DistanceToPickupKm <= 2.0 && driver.DistanceToDropoffKm <= 5.0 {
			driver.MatchQuality = "perfect"
		} else if driver.DistanceToPickupKm <= 5.0 && driver.DistanceToDropoffKm <= 10.0 {
			driver.MatchQuality = "good"
		} else {
			driver.MatchQuality = "fair"
		}

		// Calculate ETA to pickup
		driver.ETAMin = int(driver.DistanceToPickupKm * 3.0) // ~3 min per km in city
		if driver.ETAMin < 5 {
			driver.ETAMin = 5
		}

		// Add real-time status (mock data for now)
		driver.IsOnline = time.Now().Unix()%3 != 0       // ~67% online
		driver.LastSeenMin = int(time.Now().Unix() % 15) // Seen within 15 min
		driver.ResponseTimeMin = 2
		if !driver.IsOnline {
			driver.ResponseTimeMin = 10
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

// calculateSearchStatsFixed calculates search statistics
func (h *Handler) calculateSearchStatsFixed(drivers []DriverWithTrip, params DriverRequestParams) SearchStats {
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

// ===== CLIENT-DRIVER MATCHING API =====

// Fixed route-to-route matching: A→B driver route matches A→B client route
func (h *Handler) handleDriverListAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	h.logger.Info("📋 Received driver list API request", zap.String("method", r.Method))

	// Parse query parameters
	fromLatStr := r.URL.Query().Get("from_lat")
	fromLonStr := r.URL.Query().Get("from_lon")
	toLatStr := r.URL.Query().Get("to_lat")
	toLonStr := r.URL.Query().Get("to_lon")
	radiusStr := r.URL.Query().Get("radius")
	truckType := r.URL.Query().Get("truck_type")

	// Parse coordinates with fallback to Almaty
	fromLat, _ := strconv.ParseFloat(fromLatStr, 64)
	fromLon, _ := strconv.ParseFloat(fromLonStr, 64)
	toLat, _ := strconv.ParseFloat(toLatStr, 64)
	toLon, _ := strconv.ParseFloat(toLonStr, 64)
	radius, _ := strconv.ParseFloat(radiusStr, 64)

	// Set defaults for Almaty if coordinates not provided
	if fromLat == 0 || fromLon == 0 {
		fromLat = 43.238949
		fromLon = 76.889709
	}
	if toLat == 0 || toLon == 0 {
		toLat = 43.238949
		toLon = 76.889709
	}
	if radius == 0 {
		radius = 50.0 // Fixed 50km radius as requested
	}
	if truckType == "" {
		truckType = "any"
	}

	h.logger.Info("📊 Route matching search parameters",
		zap.Float64("client_from_lat", fromLat),
		zap.Float64("client_from_lon", fromLon),
		zap.Float64("client_to_lat", toLat),
		zap.Float64("client_to_lon", toLon),
		zap.Float64("radius", radius),
		zap.String("truck_type", truckType))

	// Find drivers whose A→B route matches client's A→B route
	drivers, err := h.findDriversByRouteMatching(fromLat, fromLon, toLat, toLon, radius, truckType)
	if err != nil {
		h.logger.Error("❌ Failed to find matching drivers", zap.Error(err))
		h.sendErrorResponse(w, "Ошибка получения списка водителей", http.StatusInternalServerError)
		return
	}

	h.logger.Info("✅ Route matching completed",
		zap.Int("drivers_found", len(drivers)),
		zap.Float64("search_radius", radius))

	// Build response with route matching metadata
	response := map[string]interface{}{
		"drivers": drivers,
		"count":   len(drivers),
		"search_params": map[string]interface{}{
			"client_from_lat": fromLat,
			"client_from_lon": fromLon,
			"client_to_lat":   toLat,
			"client_to_lon":   toLon,
			"radius":          radius,
			"truck_type":      truckType,
		},
		"matching_logic": map[string]interface{}{
			"type":        "route_to_route",
			"description": "Drivers whose A→B route matches client A→B route within radius",
			"radius_km":   radius,
		},
	}

	h.sendSuccessResponse(w, "Список водителей получен успешно", response)
}


// Core route-to-route matching function: A→B driver matches A→B client
func (h *Handler) findDriversByRouteMatching(clientFromLat, clientFromLon, clientToLat, clientToLon, radiusKm float64, truckType string) ([]DriverWithTrip, error) {
	h.logger.Info("🔍 Finding drivers by route matching (A→B matches A→B)",
		zap.Float64("client_A_lat", clientFromLat),
		zap.Float64("client_A_lon", clientFromLon),
		zap.Float64("client_B_lat", clientToLat),
		zap.Float64("client_B_lon", clientToLon),
		zap.Float64("radius_km", radiusKm))

	// Get all active driver trips with coordinates
	const query = `
SELECT
  d.id,
  d.telegram_id,
  d.first_name,
  d.last_name,
  d.contact_number,
  d.profile_photo,
  d.truck_type,
  d.is_verified,
  d.status,
  
  -- Driver trip data
  dt.from_address,
  dt.to_address,
  CAST(dt.from_lat AS REAL) as from_lat,
  CAST(dt.from_lon AS REAL) as from_lon,
  CAST(dt.to_lat AS REAL) as to_lat,
  CAST(dt.to_lon AS REAL) as to_lon,
  dt.price,
  dt.departure_time,
  dt.comment,
  dt.has_whatsapp,
  dt.has_telegram,
  dt.telegram_username,
  dt.created_at

FROM drivers d
JOIN driver_trips dt ON d.id = dt.driver_id
WHERE 
  -- Status filters
  LOWER(d.status) = 'approved' 
  AND dt.status = 'active'
  
  -- Coordinate data quality
  AND dt.from_lat IS NOT NULL 
  AND dt.from_lon IS NOT NULL
  AND dt.to_lat IS NOT NULL 
  AND dt.to_lon IS NOT NULL
  AND dt.from_lat != 0 AND dt.from_lon != 0
  AND dt.to_lat != 0 AND dt.to_lon != 0
  
  -- Address quality
  AND dt.from_address IS NOT NULL
  AND dt.from_address != ''
  AND dt.from_address != 'Адрес не указан'
  AND dt.to_address IS NOT NULL
  AND dt.to_address != ''
  AND dt.to_address != 'Адрес не указан'
  
  -- Recent trips only
  AND dt.created_at >= datetime('now', '-24 hours')
  
  -- Truck type filter
  AND (? = 'any' OR LOWER(d.truck_type) = LOWER(?))

ORDER BY dt.created_at DESC
LIMIT 500;
`

	rows, err := h.db.Query(query, truckType, truckType)
	if err != nil {
		h.logger.Error("❌ Database query failed", zap.Error(err))
		return nil, fmt.Errorf("database query failed: %w", err)
	}
	defer rows.Close()

	var matchingDrivers []DriverWithTrip
	var totalDrivers, routeMatches int

	for rows.Next() {
		var (
			driver          DriverWithTrip
			driverTruckType sql.NullString
			isVerifiedB     sql.NullBool
			status          sql.NullString
			fromAddress     sql.NullString
			toAddress       sql.NullString
			fromLatN        sql.NullFloat64
			fromLonN        sql.NullFloat64
			toLatN          sql.NullFloat64
			toLonN          sql.NullFloat64
			price           sql.NullInt64
			departureTime   sql.NullString
			comment         sql.NullString
			hasWhatsAppB    sql.NullBool
			hasTelegramB    sql.NullBool
			tgUsername      sql.NullString
			createdAt       sql.NullString
		)

		if err := rows.Scan(
			&driver.ID,
			&driver.TelegramID,
			&driver.FirstName,
			&driver.LastName,
			&driver.ContactNumber,
			&driver.ProfilePhoto,
			&driverTruckType,
			&isVerifiedB,
			&status,
			&fromAddress,
			&toAddress,
			&fromLatN,
			&fromLonN,
			&toLatN,
			&toLonN,
			&price,
			&departureTime,
			&comment,
			&hasWhatsAppB,
			&hasTelegramB,
			&tgUsername,
			&createdAt,
		); err != nil {
			h.logger.Warn("⚠️ Failed to scan driver row", zap.Error(err))
			continue
		}

		totalDrivers++

		// Skip if missing coordinates
		if !fromLatN.Valid || !fromLonN.Valid || !toLatN.Valid || !toLonN.Valid {
			continue
		}

		driverFromLat, driverFromLon := fromLatN.Float64, fromLonN.Float64
		driverToLat, driverToLon := toLatN.Float64, toLonN.Float64

		// CORE LOGIC: Route-to-route matching
		// Check if driver's A point is within radius of client's A point
		distanceA := h.haversineDistance(clientFromLat, clientFromLon, driverFromLat, driverFromLon)
		// Check if driver's B point is within radius of client's B point  
		distanceB := h.haversineDistance(clientToLat, clientToLon, driverToLat, driverToLon)

		h.logger.Debug("Checking route match",
			zap.String("driver_id", driver.ID),
			zap.String("driver_name", driver.FirstName+" "+driver.LastName),
			zap.Float64("distance_A_km", distanceA),
			zap.Float64("distance_B_km", distanceB),
			zap.Float64("max_radius", radiusKm))

		// MATCH CRITERIA: Both A and B points must be within radius
		if distanceA <= radiusKm && distanceB <= radiusKm {
			routeMatches++

			// Build driver data
			driver.FromAddress = "Адрес не указан"
			if fromAddress.Valid && strings.TrimSpace(fromAddress.String) != "" {
				driver.FromAddress = strings.TrimSpace(fromAddress.String)
			}

			driver.ToAddress = "Адрес не указан"
			if toAddress.Valid && strings.TrimSpace(toAddress.String) != "" {
				driver.ToAddress = strings.TrimSpace(toAddress.String)
			}

			// Extract city names
			driver.StartCity = h.extractCityFromAddress(driver.FromAddress)
			driver.EndCity = h.extractCityFromAddress(driver.ToAddress)

			// Set coordinates
			driver.FromLat = driverFromLat
			driver.FromLon = driverFromLon
			driver.ToLat = driverToLat
			driver.ToLon = driverToLon

			// Store matching distances for sorting/display
			driver.DistanceKm = distanceA // Primary distance (pickup point)
			driver.DistanceToPickupKm = distanceA
			driver.DistanceToDropoffKm = distanceB

			// Truck type
			driver.TruckType = "any"
			if driverTruckType.Valid && driverTruckType.String != "" {
				driver.TruckType = strings.ToLower(driverTruckType.String)
			}

			// Driver info
			driver.IsVerified = isVerifiedB.Valid && isVerifiedB.Bool
			driver.FullName = strings.TrimSpace(driver.FirstName + " " + driver.LastName)
			driver.Contact = driver.ContactNumber

			// Price
			if price.Valid && price.Int64 > 0 {
				driver.Price = int(price.Int64)
			}

			// Departure time
			if departureTime.Valid && strings.TrimSpace(departureTime.String) != "" {
				driver.DepartureTime = strings.TrimSpace(departureTime.String)
			} else {
				driver.DepartureTime = time.Now().Add(time.Hour).Format("2006-01-02 15:04:05")
			}

			// Comment
			if comment.Valid && comment.String != "" {
				driver.Comment = strings.TrimSpace(comment.String)
			}

			// Contact preferences
			driver.HasWhatsApp = hasWhatsAppB.Valid && hasWhatsAppB.Bool
			driver.HasTelegram = hasTelegramB.Valid && hasTelegramB.Bool
			if tgUsername.Valid && tgUsername.String != "" {
				driver.TelegramUsername = strings.TrimSpace(tgUsername.String)
			}

			// ETA calculation (based on pickup distance)
			driver.ETAMin = int(distanceA*2) + 5 // 2 minutes per km + 5 minutes base

			// Match quality assessment
			if distanceA <= 5.0 && distanceB <= 10.0 {
				driver.MatchQuality = "perfect"
			} else if distanceA <= 15.0 && distanceB <= 25.0 {
				driver.MatchQuality = "good" 
			} else {
				driver.MatchQuality = "fair"
			}

			matchingDrivers = append(matchingDrivers, driver)

			h.logger.Debug("✅ Route matched",
				zap.String("driver", driver.FullName),
				zap.String("route", fmt.Sprintf("%s → %s", driver.StartCity, driver.EndCity)),
				zap.Float64("A_distance", distanceA),
				zap.Float64("B_distance", distanceB),
				zap.String("quality", driver.MatchQuality))
		}
	}

	// Sort by combined route matching quality (A distance + B distance)
	sort.Slice(matchingDrivers, func(i, j int) bool {
		scoreI := matchingDrivers[i].DistanceToPickupKm + matchingDrivers[j].DistanceToDropoffKm*0.5
		scoreJ := matchingDrivers[j].DistanceToPickupKm + matchingDrivers[j].DistanceToDropoffKm*0.5
		return scoreI < scoreJ
	})

	h.logger.Info("✅ Route matching completed",
		zap.Int("total_drivers_checked", totalDrivers),
		zap.Int("route_matches_found", routeMatches),
		zap.Float64("radius_km", radiusKm),
		zap.String("matching_logic", "A→B driver route matches A→B client route"))

	// Log some examples
	for i, driver := range matchingDrivers {
		if i < 3 { // Log first 3 matches
			h.logger.Info("📊 Route match example",
				zap.Int("rank", i+1),
				zap.String("driver", driver.FullName),
				zap.String("driver_route", fmt.Sprintf("%s → %s", driver.StartCity, driver.EndCity)),
				zap.Float64("A_distance_km", driver.DistanceToPickupKm),
				zap.Float64("B_distance_km", driver.DistanceToDropoffKm),
				zap.String("match_quality", driver.MatchQuality),
				zap.Int("price", driver.Price))
		}
	}

	return matchingDrivers, nil
}


// Fixed driver matching function - calculates distance in Go instead of SQL
func (h *Handler) findMatchingDriversWithGoCalculation(params DriverRequestParams) ([]DriverWithTrip, error) {
	h.logger.Info("🔍 Finding drivers (Go distance calculation)",
		zap.Float64("pickup_lat", params.PickupLat),
		zap.Float64("pickup_lon", params.PickupLon),
		zap.Float64("radius_km", params.RadiusKm),
		zap.String("truck_type", params.TruckType))

	// SQL query WITHOUT distance calculation - fetch all active drivers
	const query = `
SELECT
  d.id,
  d.telegram_id,
  d.first_name,
  d.last_name,
  d.contact_number,
  d.profile_photo,
  d.truck_type,
  d.is_verified,
  d.status,
  
  -- Driver trip data with addresses
  dt.from_address,
  dt.to_address,
  CAST(dt.from_lat AS REAL) as from_lat,
  CAST(dt.from_lon AS REAL) as from_lon,
  CAST(dt.to_lat AS REAL) as to_lat,
  CAST(dt.to_lon AS REAL) as to_lon,
  dt.price,
  dt.departure_time,
  dt.comment,
  dt.has_whatsapp,
  dt.has_telegram,
  dt.telegram_username,
  dt.created_at

FROM drivers d
JOIN driver_trips dt ON d.id = dt.driver_id
WHERE 
  1=1  -- Always true base condition
  
  -- Status filter: only approved drivers
  AND LOWER(d.status) = 'approved' 
  
  -- Data quality filters
  AND dt.from_lat IS NOT NULL 
  AND dt.from_lon IS NOT NULL
  AND dt.from_address IS NOT NULL
  AND dt.from_address != ''
  AND dt.from_address != 'Адрес не указан'
  
  -- Time filter: recent trips only
  AND dt.created_at >= datetime('now', '-24 hours')
  
  -- Truck type filter (if specified and not 'any')
  AND (? = 'any' OR LOWER(d.truck_type) = LOWER(?))

ORDER BY dt.created_at DESC
LIMIT 500;
`

	// Execute query
	rows, err := h.db.Query(query, params.TruckType, params.TruckType)
	if err != nil {
		h.logger.Error("❌ Database query failed", zap.Error(err))
		return nil, fmt.Errorf("database query failed: %w", err)
	}
	defer rows.Close()

	var allDrivers []DriverWithTrip
	seenDrivers := make(map[string]bool)

	for rows.Next() {
		var (
			driver          DriverWithTrip
			driverTruckType sql.NullString
			isVerifiedB     sql.NullBool
			status          sql.NullString
			fromAddress     sql.NullString
			toAddress       sql.NullString
			fromLatN        sql.NullFloat64
			fromLonN        sql.NullFloat64
			toLatN          sql.NullFloat64
			toLonN          sql.NullFloat64
			price           sql.NullInt64
			departureTime   sql.NullString
			comment         sql.NullString
			hasWhatsAppB    sql.NullBool
			hasTelegramB    sql.NullBool
			tgUsername      sql.NullString
			createdAt       sql.NullString
		)

		if err := rows.Scan(
			&driver.ID,
			&driver.TelegramID,
			&driver.FirstName,
			&driver.LastName,
			&driver.ContactNumber,
			&driver.ProfilePhoto,
			&driverTruckType,
			&isVerifiedB,
			&status,
			&fromAddress,
			&toAddress,
			&fromLatN,
			&fromLonN,
			&toLatN,
			&toLonN,
			&price,
			&departureTime,
			&comment,
			&hasWhatsAppB,
			&hasTelegramB,
			&tgUsername,
			&createdAt,
		); err != nil {
			h.logger.Warn("⚠️ Failed to scan driver row", zap.Error(err))
			continue
		}

		// Skip duplicate drivers (only take the most recent trip per driver)
		if seenDrivers[driver.ID] {
			continue
		}
		seenDrivers[driver.ID] = true

		// Skip inactive drivers
		if status.Valid && strings.ToLower(status.String) != "approved" {
			continue
		}

		// Set addresses - Extract real city names
		driver.FromAddress = "Адрес не указан"
		if fromAddress.Valid && strings.TrimSpace(fromAddress.String) != "" {
			driver.FromAddress = strings.TrimSpace(fromAddress.String)
		}

		driver.ToAddress = "Адрес не указан"
		if toAddress.Valid && strings.TrimSpace(toAddress.String) != "" {
			driver.ToAddress = strings.TrimSpace(toAddress.String)
		}

		// Extract city names from addresses
		driver.StartCity = h.extractCityFromAddress(driver.FromAddress)
		driver.EndCity = h.extractCityFromAddress(driver.ToAddress)

		// Set coordinates
		if fromLatN.Valid && fromLonN.Valid {
			driver.FromLat = fromLatN.Float64
			driver.FromLon = fromLonN.Float64
		} else {
			// Skip drivers without valid coordinates
			continue
		}
		
		if toLatN.Valid && toLonN.Valid {
			driver.ToLat = toLatN.Float64
			driver.ToLon = toLonN.Float64
		}

		// CRITICAL: Calculate distance in Go using Haversine formula
		distance := h.haversineDistance(params.PickupLat, params.PickupLon, driver.FromLat, driver.FromLon)
		driver.DistanceKm = distance

		// Filter by radius in Go code
		if distance > params.RadiusKm {
			continue // Skip drivers outside radius
		}

		// Truck type with fallback
		driver.TruckType = "any"
		if driverTruckType.Valid && driverTruckType.String != "" {
			driver.TruckType = strings.ToLower(driverTruckType.String)
		}

		// Basic driver info
		driver.IsVerified = isVerifiedB.Valid && isVerifiedB.Bool
		driver.FullName = strings.TrimSpace(driver.FirstName + " " + driver.LastName)
		driver.Contact = driver.ContactNumber

		// Price with validation
		if price.Valid && price.Int64 > 0 {
			driver.Price = int(price.Int64)
		}

		// Departure time - keep as string for simplicity
		if departureTime.Valid && strings.TrimSpace(departureTime.String) != "" {
			driver.DepartureTime = strings.TrimSpace(departureTime.String)
		} else {
			// Set default departure time if not provided
			driver.DepartureTime = time.Now().Add(time.Hour).Format("2006-01-02 15:04:05")
		}

		// Comments
		if comment.Valid && comment.String != "" {
			driver.Comment = strings.TrimSpace(comment.String)
		}

		// Contact preferences
		driver.HasWhatsApp = hasWhatsAppB.Valid && hasWhatsAppB.Bool
		driver.HasTelegram = hasTelegramB.Valid && hasTelegramB.Bool
		if tgUsername.Valid && tgUsername.String != "" {
			driver.TelegramUsername = strings.TrimSpace(tgUsername.String)
		}

		// ETA calculation based on distance
		driver.ETAMin = int(distance*2) + 5 // 2 minutes per km + 5 minutes base time

		// Cap ETA at reasonable limits
		if driver.ETAMin > 120 {
			driver.ETAMin = 120
		}
		if driver.ETAMin < 5 {
			driver.ETAMin = 5
		}

		allDrivers = append(allDrivers, driver)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	// Sort by distance (closest first)
	sort.Slice(allDrivers, func(i, j int) bool {
		return allDrivers[i].DistanceKm < allDrivers[j].DistanceKm
	})

	// Log results summary
	h.logger.Info("✅ Found matching drivers (Go calculation)",
		zap.Int("total_found", len(allDrivers)),
		zap.Float64("within_km", params.RadiusKm),
		zap.String("truck_type", params.TruckType))

	// Log some details about found drivers for debugging
	if len(allDrivers) > 0 {
		h.logger.Info("📊 Driver search results summary",
			zap.Float64("closest_distance", allDrivers[0].DistanceKm),
			zap.String("closest_driver", allDrivers[0].FullName),
			zap.String("closest_start_city", allDrivers[0].StartCity))
		
		if len(allDrivers) > 1 {
			h.logger.Info("📊 Driver search results - farthest",
				zap.Float64("farthest_distance", allDrivers[len(allDrivers)-1].DistanceKm),
				zap.String("farthest_driver", allDrivers[len(allDrivers)-1].FullName))
		}
	}

	return allDrivers, nil
}


// isValidCoordinates validates if coordinates are within Kazakhstan bounds
func (h *Handler) isValidCoordinates(lat, lon float64) bool {
	// Kazakhstan approximate bounds
	return lat >= 40.0 && lat <= 56.0 && lon >= 46.0 && lon <= 88.0
}

// ===== EXISTING METHODS (keeping all the original functionality) =====
// FIXED: handleDelivery with better form parsing and route calculation
func (h *Handler) handleDelivery(ctx context.Context, b *bot.Bot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		h.logger.Info("Received delivery request",
			zap.String("method", r.Method),
			zap.String("content_type", r.Header.Get("Content-Type")),
			zap.String("user_agent", r.Header.Get("User-Agent")))

		// Parse form data - handle both form-urlencoded and multipart
		var err error
		if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
			err = r.ParseMultipartForm(32 << 20) // 32 MB max
		} else {
			err = r.ParseForm()
		}

		if err != nil {
			h.logger.Error("Failed to parse form", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка обработки данных", http.StatusBadRequest)
			return
		}

		// Log all form values for debugging
		h.logger.Info("Form values received", zap.Any("form", r.Form))

		// Extract and validate data
		req, err := h.parseDeliveryRequest(r)
		if err != nil {
			h.logger.Error("Failed to parse delivery request", zap.Error(err))
			h.sendErrorResponse(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Calculate route distance and time if not provided
		if req.DistanceKm == 0 || req.EtaMin == 0 {
			distance, duration := h.calculateRoute(req.FromLat, req.FromLon, req.ToLat, req.ToLon)
			if req.DistanceKm == 0 {
				req.DistanceKm = distance
			}
			if req.EtaMin == 0 {
				req.EtaMin = duration
			}
		}

		h.logger.Info("Parsed delivery request",
			zap.String("from", req.FromAddress),
			zap.String("to", req.ToAddress),
			zap.Int("price", req.Price),
			zap.Float64("distance", req.DistanceKm),
			zap.Int("eta", req.EtaMin),
			zap.String("truck_type", req.TruckType),
			zap.String("time_start", req.TimeStart),
			zap.Int64("telegram_id", req.TelegramID))

		// Save delivery request to database
		requestID, err := h.saveDeliveryRequest(req)
		if err != nil {
			h.logger.Error("Failed to save delivery request", zap.Error(err))
			h.sendErrorResponse(w, "Ошибка сохранения заявки", http.StatusInternalServerError)
			return
		}

		req.ID = requestID
		req.Status = "pending"
		req.CreatedAt = time.Now()

		h.logger.Info("Delivery request saved successfully", zap.String("request_id", requestID))

		// Send confirmation message to user
		go h.sendConfirmationMessage(b, req, requestID)
		// Send to client order to driver
		go h.SendToDriver(ctx, b, req)

		// Send success response
		h.sendSuccessResponse(w, "Заявка успешно создана", map[string]interface{}{
			"request_id": requestID,
			"status":     "pending",
			"distance":   req.DistanceKm,
			"eta":        req.EtaMin,
		})
	}
}

func (h *Handler) SendToDriver(ctx context.Context, b *bot.Bot, request *DeliveryRequest) {

	// Look here write logic to send nearest driver to user
	// and send him client request from  just A point

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: 0,
		Text:   "Sorry",
	})
	if err != nil {
		h.logger.Error("Failed to send message", zap.Error(err))
		return
	}
}

// parseDeliveryRequest parses the delivery request form data
func (h *Handler) parseDeliveryRequest(r *http.Request) (*DeliveryRequest, error) {
	req := &DeliveryRequest{}

	// Helper function to get form value
	getValue := func(key string) string {
		return strings.TrimSpace(r.FormValue(key))
	}

	// Required fields
	req.FromAddress = getValue("from_address")
	req.ToAddress = getValue("to_address")
	req.Contact = getValue("contact")

	h.logger.Debug("Parsing form fields",
		zap.String("from_address", req.FromAddress),
		zap.String("to_address", req.ToAddress),
		zap.String("contact", req.Contact))

	if req.FromAddress == "" {
		return nil, fmt.Errorf("адрес отправления обязателен")
	}
	if req.ToAddress == "" {
		return nil, fmt.Errorf("адрес назначения обязателен")
	}
	if req.Contact == "" {
		return nil, fmt.Errorf("контактный номер обязателен")
	}

	// Parse coordinates
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

	// Parse price
	if priceStr := getValue("price"); priceStr != "" {
		req.Price, err = strconv.Atoi(priceStr)
		if err != nil || req.Price < 2000 {
			return nil, fmt.Errorf("цена должна быть не менее 2000 тенге")
		}
	} else {
		return nil, fmt.Errorf("цена обязательна")
	}

	// Parse distance and ETA from frontend
	if distStr := getValue("distance"); distStr != "" {
		req.DistanceKm, _ = strconv.ParseFloat(distStr, 64)
	}

	if etaStr := getValue("duration"); etaStr != "" {
		req.EtaMin, _ = strconv.Atoi(etaStr)
	}

	// Parse Telegram ID
	if telegramIDStr := getValue("telegram_id"); telegramIDStr != "" {
		req.TelegramID, _ = strconv.ParseInt(telegramIDStr, 10, 64)
	}

	// Optional fields
	req.TruckType = getValue("truck_type")
	req.Comment = getValue("comment")

	// Parse date and time to create proper time_start
	dateStr := getValue("date")
	timeStr := getValue("time")

	if dateStr != "" && timeStr != "" {
		// Combine date and time
		req.TimeStart = dateStr + "T" + timeStr
		h.logger.Debug("Parsed time_start", zap.String("time_start", req.TimeStart))
	} else {
		// Use current time if not specified
		req.TimeStart = time.Now().Format("2006-01-02T15:04")
	}

	return req, nil
}

// IMPROVED: Better route calculation with actual road routing
func (h *Handler) calculateRoute(fromLat, fromLon, toLat, toLon float64) (float64, int) {
	// If coordinates are missing, return defaults
	if fromLat == 0 || fromLon == 0 || toLat == 0 || toLon == 0 {
		return 10.0, 30 // Default values
	}

	// Try to get actual route from OSRM (free routing service)
	actualDistance, actualDuration := h.getOSRMRoute(fromLat, fromLon, toLat, toLon)
	if actualDistance > 0 {
		return actualDistance, actualDuration
	}

	// Fallback to Haversine calculation with road factor
	straightDistance := h.haversineDistance(fromLat, fromLon, toLat, toLon)
	roadDistance := straightDistance * 1.3                // 30% longer for actual roads
	drivingTimeMinutes := int((roadDistance / 35.0) * 60) // 35 km/h average city speed

	h.logger.Debug("Calculated route (fallback)",
		zap.Float64("straight_distance", straightDistance),
		zap.Float64("road_distance", roadDistance),
		zap.Int("duration_minutes", drivingTimeMinutes))

	return roadDistance, drivingTimeMinutes
}

// Get actual route from OSRM routing service
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
			Distance float64 `json:"distance"` // meters
			Duration float64 `json:"duration"` // seconds
		} `json:"routes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		h.logger.Debug("OSRM response decode failed", zap.Error(err))
		return 0, 0
	}

	if len(result.Routes) == 0 {
		return 0, 0
	}

	distanceKm := result.Routes[0].Distance / 1000.0
	durationMin := int(result.Routes[0].Duration / 60.0)

	h.logger.Debug("OSRM route calculated",
		zap.Float64("distance_km", distanceKm),
		zap.Int("duration_min", durationMin))

	return distanceKm, durationMin
}

// saveDeliveryRequest saves the delivery request to database
func (h *Handler) saveDeliveryRequest(req *DeliveryRequest) (string, error) {
	// Generate UUID for the delivery request
	requestID := uuid.New().String()

	query := `
		INSERT INTO delivery_requests (
			id, telegram_id, from_address, from_lat, from_lon, 
			to_address, to_lat, to_lon, distance_km, eta_min,
			price, truck_type, contact, time_start, comment, 
			status, created_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', CURRENT_TIMESTAMP
		)`

	_, err := h.db.Exec(
		query,
		requestID, req.TelegramID, req.FromAddress, req.FromLat, req.FromLon,
		req.ToAddress, req.ToLat, req.ToLon, req.DistanceKm, req.EtaMin,
		req.Price, req.TruckType, req.Contact, req.TimeStart, req.Comment,
	)

	if err != nil {
		return "", err
	}

	return requestID, nil
}

// sendConfirmationMessage sends confirmation message to client
func (h *Handler) sendConfirmationMessage(b *bot.Bot, req *DeliveryRequest, requestID string) {
	if req.TelegramID == 0 {
		h.logger.Warn("No Telegram ID provided, skipping confirmation message")
		return
	}

	// Format truck type in Kazakh/Russian
	truckTypeText := h.formatTruckType(req.TruckType)

	// Format time
	timeText := h.formatTimeStart(req.TimeStart)

	// Create detailed confirmation message with all data
	message := fmt.Sprintf(`🚛 <b>Жеткізу тапсырысы берілді!</b>

📋 <b>Өтінім нөмірі:</b> #%d

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

	// Add comment if provided
	if req.Comment != "" {
		message += fmt.Sprintf("\n💬 <b>Түсініктеме:</b> %s", req.Comment)
	}

	message += "\n\n✅ Сіздің өтініміңіз жүргізушілерге жіберілді!"

	// Send message with order tracking keyboard
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "📋 Менің тапсырыстарым", CallbackData: "my_orders"},
				{Text: "🔄 Жаңарту", CallbackData: fmt.Sprintf("refresh_order_%d", requestID)},
			},
		},
	}

	ctx := context.Background()

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      req.TelegramID,
		Text:        message,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		h.logger.Error("Failed to send confirmation message",
			zap.Error(err),
			zap.Int64("telegram_id", req.TelegramID),
			zap.String("request_id", requestID))
	} else {
		h.logger.Info("Confirmation message sent",
			zap.Int64("telegram_id", req.TelegramID),
			zap.String("request_id", requestID))
	}
}

// formatTruckType formats truck type for display
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

// formatTimeStart formats time for display
func (h *Handler) formatTimeStart(timeStart string) string {
	if timeStart == "" {
		return "🕐 <b>Кету уақыты:</b> Қазір | Сейчас"
	}

	// Parse ISO format time (2006-01-02T15:04)
	t, err := time.Parse("2006-01-02T15:04", timeStart)
	if err != nil {
		// Try alternative formats
		if t2, err2 := time.Parse("2006-01-02 15:04", timeStart); err2 == nil {
			t = t2
		} else {
			return fmt.Sprintf("🕐 <b>Кету уақыты:</b> %s", timeStart)
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

// sendErrorResponse sends an error response
func (h *Handler) sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(Response{
		Success: false,
		Message: message,
	})
}

// sendSuccessResponse sends a success response with optional data
func (h *Handler) sendSuccessResponse(w http.ResponseWriter, message string, data ...interface{}) {
	response := Response{
		Success: true,
		Message: message,
	}

	if len(data) > 0 {
		response.Data = data[0]
	}

	json.NewEncoder(w).Encode(response)
}
