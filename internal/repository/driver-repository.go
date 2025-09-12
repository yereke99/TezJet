package repository

import (
	"context"
	"database/sql"
	"fmt"
	"tezjet/internal/domain"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type DriverRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewDriverRepository(db *sql.DB, logger *zap.Logger) *DriverRepository {
	return &DriverRepository{
		db:     db,
		logger: logger,
	}
}

// CreateDriver creates a new driver in the database
func (r *DriverRepository) CreateDriver(req *domain.CreateDriverRequest, files map[string]string) (*domain.Driver, error) {
	driverID := uuid.New().String() // Generate UUID for the driver

	birthday, err := time.Parse("2006-01-02", req.Birthday)
	if err != nil {
		return nil, fmt.Errorf("invalid birthday format: %w", err)
	}

	query := `
		INSERT INTO drivers (
			id, telegram_id, telegram_username, first_name, last_name, birthday,
			contact_number, start_city, latitude, longitude, profile_photo_path,
			license_front_path, license_back_path, truck_photo_path, status,
			is_approved, is_active, has_whatsapp, has_telegram, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()

	_, err = r.db.Exec(query,
		driverID, req.TelegramID, req.TelegramUsername, req.FirstName, req.LastName, birthday,
		req.ContactNumber, req.StartCity, req.Latitude, req.Longitude,
		files["profilePhoto"], files["licenseFront"], files["licenseBack"], files["truckPhoto"],
		domain.DriverStatusPending, false, true, req.HasWhatsapp, req.HasTelegram, now, now,
	)

	if err != nil {
		r.logger.Error("Failed to create driver", zap.Error(err))
		return nil, fmt.Errorf("failed to create driver: %w", err)
	}

	return r.GetDriverByID(driverID)
}

// GetDriverByID retrieves a driver by their database ID (UUID)
func (r *DriverRepository) GetDriverByID(driverID string) (*domain.Driver, error) {
	query := `
		SELECT id, telegram_id, telegram_username, first_name, last_name, birthday,
			   contact_number, start_city, latitude, longitude, profile_photo_path,
			   license_front_path, license_back_path, truck_photo_path, status,
			   is_approved, is_active, is_online, rating, total_trips, total_earnings,
			   has_whatsapp, has_telegram, approved_at, last_active_at, created_at, updated_at
		FROM drivers 
		WHERE id = ?`

	driver := &domain.Driver{}
	var approvedAt, lastActiveAt sql.NullTime

	err := r.db.QueryRow(query, driverID).Scan(
		&driver.ID, &driver.TelegramID, &driver.TelegramUsername, &driver.FirstName, &driver.LastName, &driver.Birthday,
		&driver.ContactNumber, &driver.StartCity, &driver.Latitude, &driver.Longitude, &driver.ProfilePhotoPath,
		&driver.LicenseFrontPath, &driver.LicenseBackPath, &driver.TruckPhotoPath, &driver.Status,
		&driver.IsApproved, &driver.IsActive, &driver.IsOnline, &driver.Rating, &driver.TotalTrips, &driver.TotalEarnings,
		&driver.HasWhatsapp, &driver.HasTelegram, &approvedAt, &lastActiveAt, &driver.CreatedAt, &driver.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("driver not found")
		}
		r.logger.Error("Failed to get driver by ID", zap.Error(err), zap.String("driver_id", driverID))
		return nil, fmt.Errorf("failed to get driver: %w", err)
	}

	if approvedAt.Valid {
		driver.ApprovedAt = &approvedAt.Time
	}
	if lastActiveAt.Valid {
		driver.LastActiveAt = &lastActiveAt.Time
	}

	return driver, nil
}

// GetDriverByTelegramID retrieves a driver by their Telegram ID
func (r *DriverRepository) GetDriverByTelegramID(telegramID int64) (*domain.Driver, error) {
	query := `
		SELECT id, telegram_id, telegram_username, first_name, last_name, birthday,
			   contact_number, start_city, latitude, longitude, profile_photo_path,
			   license_front_path, license_back_path, truck_photo_path, status,
			   is_approved, is_active, is_online, rating, total_trips, total_earnings,
			   has_whatsapp, has_telegram, approved_at, last_active_at, created_at, updated_at
		FROM drivers 
		WHERE telegram_id = ?`

	driver := &domain.Driver{}
	var approvedAt, lastActiveAt sql.NullTime

	err := r.db.QueryRow(query, telegramID).Scan(
		&driver.ID, &driver.TelegramID, &driver.TelegramUsername, &driver.FirstName, &driver.LastName, &driver.Birthday,
		&driver.ContactNumber, &driver.StartCity, &driver.Latitude, &driver.Longitude, &driver.ProfilePhotoPath,
		&driver.LicenseFrontPath, &driver.LicenseBackPath, &driver.TruckPhotoPath, &driver.Status,
		&driver.IsApproved, &driver.IsActive, &driver.IsOnline, &driver.Rating, &driver.TotalTrips, &driver.TotalEarnings,
		&driver.HasWhatsapp, &driver.HasTelegram, &approvedAt, &lastActiveAt, &driver.CreatedAt, &driver.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("driver not found")
		}
		r.logger.Error("Failed to get driver by Telegram ID", zap.Error(err), zap.Int64("telegram_id", telegramID))
		return nil, fmt.Errorf("failed to get driver: %w", err)
	}

	if approvedAt.Valid {
		driver.ApprovedAt = &approvedAt.Time
	}
	if lastActiveAt.Valid {
		driver.LastActiveAt = &lastActiveAt.Time
	}

	return driver, nil
}

// Helper method to get driver ID by telegram ID
func (r *DriverRepository) GetDriverIDByTelegramID(telegramID int64) (string, error) {
	query := `SELECT id FROM drivers WHERE telegram_id = ?`

	var driverID string
	err := r.db.QueryRow(query, telegramID).Scan(&driverID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("driver not found")
		}
		return "", fmt.Errorf("failed to get driver ID: %w", err)
	}

	return driverID, nil
}

// ChangeDriverStatus changes driver status and returns affected telegram IDs
func (r *DriverRepository) ChangeDriverStatus(ctx context.Context, actualStatus, newStatus string) ([]int64, error) {
	q := `UPDATE drivers SET status = ? WHERE status = ? RETURNING telegram_id;`
	rows, err := r.db.QueryContext(ctx, q, newStatus, actualStatus)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var telegramId []int64
	for rows.Next() {
		var tgId int64
		if err := rows.Scan(&tgId); err != nil {
			return nil, fmt.Errorf("failed to scan driver ID: %w", err)
		}
		telegramId = append(telegramId, tgId)
	}
	return telegramId, nil
}

// ApproveDriver approves a driver
func (r *DriverRepository) ApproveDriver(driverID string) error {
	query := `
		UPDATE drivers 
		SET is_approved = true, status = ?, approved_at = ?, updated_at = ?
		WHERE id = ?`

	now := time.Now()
	result, err := r.db.Exec(query, domain.DriverStatusApproved, now, now, driverID)
	if err != nil {
		r.logger.Error("Failed to approve driver", zap.Error(err), zap.String("driver_id", driverID))
		return fmt.Errorf("failed to approve driver: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("driver not found")
	}

	return nil
}

// RejectDriver rejects a driver
func (r *DriverRepository) RejectDriver(driverID string) error {
	query := `
		UPDATE drivers 
		SET is_approved = false, status = ?, updated_at = ?
		WHERE id = ?`

	result, err := r.db.Exec(query, domain.DriverStatusRejected, time.Now(), driverID)
	if err != nil {
		r.logger.Error("Failed to reject driver", zap.Error(err), zap.String("driver_id", driverID))
		return fmt.Errorf("failed to reject driver: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("driver not found")
	}

	return nil
}

// CreateDriverRoute creates a new driver route
func (r *DriverRepository) CreateDriverRoute(req *domain.CreateDriverRouteRequest, driverID string, distance float64) (*domain.DriverRoute, error) {
	routeID := uuid.New().String() // Generate UUID for the route

	query := `
		INSERT INTO driver_routes (
			id, driver_id, telegram_id, from_address, from_lat, from_lon,
			to_address, to_lat, to_lon, price, truck_type, max_weight,
			comment, departure_time, distance_km, status, available_seats,
			is_active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()

	_, err := r.db.Exec(query,
		routeID, driverID, req.TelegramID, req.FromAddress, req.FromLat, req.FromLon,
		req.ToAddress, req.ToLat, req.ToLon, req.Price, req.TruckType, req.MaxWeight,
		req.Comment, req.DepartureTime, distance, domain.RouteStatusActive, req.AvailableSeats,
		true, now, now,
	)

	if err != nil {
		r.logger.Error("Failed to create driver route", zap.Error(err))
		return nil, fmt.Errorf("failed to create driver route: %w", err)
	}

	return r.GetDriverRouteByID(routeID)
}

// GetDriverRouteByID retrieves a driver route by ID (UUID)
func (r *DriverRepository) GetDriverRouteByID(routeID string) (*domain.DriverRoute, error) {
	query := `
		SELECT id, driver_id, telegram_id, from_address, from_lat, from_lon,
			   to_address, to_lat, to_lon, price, truck_type, max_weight,
			   comment, departure_time, arrival_time, distance_km, status,
			   available_seats, is_active, created_at, updated_at
		FROM driver_routes 
		WHERE id = ?`

	route := &domain.DriverRoute{}
	var arrivalTime sql.NullTime

	err := r.db.QueryRow(query, routeID).Scan(
		&route.ID, &route.DriverID, &route.TelegramID, &route.FromAddress, &route.FromLat, &route.FromLon,
		&route.ToAddress, &route.ToLat, &route.ToLon, &route.Price, &route.TruckType, &route.MaxWeight,
		&route.Comment, &route.DepartureTime, &arrivalTime, &route.DistanceKm, &route.Status,
		&route.AvailableSeats, &route.IsActive, &route.CreatedAt, &route.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("driver route not found")
		}
		r.logger.Error("Failed to get driver route", zap.Error(err), zap.String("route_id", routeID))
		return nil, fmt.Errorf("failed to get driver route: %w", err)
	}

	if arrivalTime.Valid {
		route.ArrivalTime = &arrivalTime.Time
	}

	return route, nil
}

// GetDriverRoutes retrieves all routes for a driver
func (r *DriverRepository) GetDriverRoutes(telegramID int64, limit, offset int) ([]*domain.DriverRoute, error) {
	query := `
		SELECT id, driver_id, telegram_id, from_address, from_lat, from_lon,
			   to_address, to_lat, to_lon, price, truck_type, max_weight,
			   comment, departure_time, arrival_time, distance_km, status,
			   available_seats, is_active, created_at, updated_at
		FROM driver_routes 
		WHERE telegram_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`

	rows, err := r.db.Query(query, telegramID, limit, offset)
	if err != nil {
		r.logger.Error("Failed to get driver routes", zap.Error(err), zap.Int64("telegram_id", telegramID))
		return nil, fmt.Errorf("failed to get driver routes: %w", err)
	}
	defer rows.Close()

	var routes []*domain.DriverRoute
	for rows.Next() {
		route := &domain.DriverRoute{}
		var arrivalTime sql.NullTime

		err := rows.Scan(
			&route.ID, &route.DriverID, &route.TelegramID, &route.FromAddress, &route.FromLat, &route.FromLon,
			&route.ToAddress, &route.ToLat, &route.ToLon, &route.Price, &route.TruckType, &route.MaxWeight,
			&route.Comment, &route.DepartureTime, &arrivalTime, &route.DistanceKm, &route.Status,
			&route.AvailableSeats, &route.IsActive, &route.CreatedAt, &route.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan driver route", zap.Error(err))
			continue
		}

		if arrivalTime.Valid {
			route.ArrivalTime = &arrivalTime.Time
		}

		routes = append(routes, route)
	}

	return routes, nil
}

// GetActiveDriverRoutes retrieves active routes for a driver
func (r *DriverRepository) GetActiveDriverRoutes(telegramID int64) ([]*domain.DriverRoute, error) {
	query := `
		SELECT id, driver_id, telegram_id, from_address, from_lat, from_lon,
			   to_address, to_lat, to_lon, price, truck_type, max_weight,
			   comment, departure_time, arrival_time, distance_km, status,
			   available_seats, is_active, created_at, updated_at
		FROM driver_routes 
		WHERE telegram_id = ? AND status = ? AND is_active = true AND departure_time > ?
		ORDER BY departure_time ASC`

	rows, err := r.db.Query(query, telegramID, domain.RouteStatusActive, time.Now())
	if err != nil {
		r.logger.Error("Failed to get active driver routes", zap.Error(err), zap.Int64("telegram_id", telegramID))
		return nil, fmt.Errorf("failed to get active driver routes: %w", err)
	}
	defer rows.Close()

	var routes []*domain.DriverRoute
	for rows.Next() {
		route := &domain.DriverRoute{}
		var arrivalTime sql.NullTime

		err := rows.Scan(
			&route.ID, &route.DriverID, &route.TelegramID, &route.FromAddress, &route.FromLat, &route.FromLon,
			&route.ToAddress, &route.ToLat, &route.ToLon, &route.Price, &route.TruckType, &route.MaxWeight,
			&route.Comment, &route.DepartureTime, &arrivalTime, &route.DistanceKm, &route.Status,
			&route.AvailableSeats, &route.IsActive, &route.CreatedAt, &route.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan driver route", zap.Error(err))
			continue
		}

		if arrivalTime.Valid {
			route.ArrivalTime = &arrivalTime.Time
		}

		routes = append(routes, route)
	}

	return routes, nil
}

// FindMatchingDrivers finds drivers matching a delivery request
func (r *DriverRepository) FindMatchingDrivers(fromLat, fromLon, toLat, toLon float64, truckType string, maxDistance float64) ([]*domain.MatchedDriver, error) {
	query := `
		SELECT DISTINCT d.id, d.first_name, d.last_name, d.profile_photo_path, d.contact_number,
			   d.rating, d.total_trips, dr.from_address, dr.to_address, dr.price,
			   dr.truck_type, dr.comment, dr.departure_time, d.truck_photo_path,
			   d.has_whatsapp, d.has_telegram, d.telegram_username, dr.from_lat, dr.from_lon,
			   dr.to_lat, dr.to_lon,
			   (6371 * acos(cos(radians(?)) * cos(radians(dr.from_lat)) * 
			    cos(radians(dr.from_lon) - radians(?)) + sin(radians(?)) * 
			    sin(radians(dr.from_lat)))) AS pickup_distance,
			   (6371 * acos(cos(radians(?)) * cos(radians(dr.to_lat)) * 
			    cos(radians(dr.to_lon) - radians(?)) + sin(radians(?)) * 
			    sin(radians(dr.to_lat)))) AS dropoff_distance
		FROM drivers d
		JOIN driver_routes dr ON d.id = dr.driver_id
		WHERE d.is_approved = true 
		AND d.is_active = true 
		AND dr.status = ?
		AND dr.is_active = true
		AND dr.departure_time > ?
		AND (dr.truck_type = ? OR ? = 'any')
		AND available_seats > 0
		HAVING pickup_distance <= ? AND dropoff_distance <= ?
		ORDER BY pickup_distance ASC, d.rating DESC
		LIMIT 20`

	rows, err := r.db.Query(query,
		fromLat, fromLon, fromLat, // for pickup distance calculation
		toLat, toLon, toLat, // for dropoff distance calculation
		domain.RouteStatusActive, time.Now(), truckType, truckType,
		maxDistance, maxDistance)
	if err != nil {
		r.logger.Error("Failed to find matching drivers", zap.Error(err))
		return nil, fmt.Errorf("failed to find matching drivers: %w", err)
	}
	defer rows.Close()

	var drivers []*domain.MatchedDriver
	for rows.Next() {
		driver := &domain.MatchedDriver{}
		var pickupDistance, dropoffDistance float64
		var firstName, lastName string

		err := rows.Scan(
			&driver.DriverID, &firstName, &lastName, &driver.ProfilePhoto, &driver.ContactNumber,
			&driver.Rating, &driver.TotalTrips, &driver.FromAddress, &driver.ToAddress, &driver.Price,
			&driver.TruckType, &driver.Comment, &driver.DepartureTime, &driver.TruckPhoto,
			&driver.HasWhatsapp, &driver.HasTelegram, &driver.TelegramUsername, &driver.FromLat, &driver.FromLon,
			&driver.ToLat, &driver.ToLon, &pickupDistance, &dropoffDistance,
		)
		if err != nil {
			r.logger.Error("Failed to scan matched driver", zap.Error(err))
			continue
		}

		// Combine first and last name
		driver.FullName = firstName
		if lastName != "" {
			driver.FullName += " " + lastName
		}

		drivers = append(drivers, driver)
	}

	return drivers, nil
}

// UpdateDriverStatus updates driver status
func (r *DriverRepository) UpdateDriverStatus(driverID string, status string) error {
	query := `
		UPDATE drivers 
		SET status = ?, updated_at = ?
		WHERE id = ?`

	result, err := r.db.Exec(query, status, time.Now(), driverID)
	if err != nil {
		r.logger.Error("Failed to update driver status", zap.Error(err), zap.String("driver_id", driverID))
		return fmt.Errorf("failed to update driver status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("driver not found")
	}

	return nil
}

// UpdateDriverOnlineStatus updates driver online status
func (r *DriverRepository) UpdateDriverOnlineStatus(telegramID int64, isOnline bool) error {
	query := `
		UPDATE drivers 
		SET is_online = ?, last_active_at = ?, updated_at = ?
		WHERE telegram_id = ?`

	now := time.Now()
	result, err := r.db.Exec(query, isOnline, now, now, telegramID)
	if err != nil {
		r.logger.Error("Failed to update driver online status", zap.Error(err), zap.Int64("telegram_id", telegramID))
		return fmt.Errorf("failed to update driver online status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("driver not found")
	}

	return nil
}

// UpdateDriverRoute updates a driver route
func (r *DriverRepository) UpdateDriverRoute(routeID string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return fmt.Errorf("no updates provided")
	}

	setParts := make([]string, 0, len(updates))
	args := make([]interface{}, 0, len(updates)+2)

	for field, value := range updates {
		setParts = append(setParts, field+" = ?")
		args = append(args, value)
	}

	query := fmt.Sprintf("UPDATE driver_routes SET %s, updated_at = ? WHERE id = ?",
		setParts[0])
	for i := 1; i < len(setParts); i++ {
		query = fmt.Sprintf("%s, %s", query, setParts[i])
	}

	args = append(args, time.Now(), routeID)

	result, err := r.db.Exec(query, args...)
	if err != nil {
		r.logger.Error("Failed to update driver route", zap.Error(err), zap.String("route_id", routeID))
		return fmt.Errorf("failed to update driver route: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("driver route not found")
	}

	return nil
}

// CompleteDriverRoute marks a driver route as completed
func (r *DriverRepository) CompleteDriverRoute(routeID string) error {
	query := `
		UPDATE driver_routes 
		SET status = ?, arrival_time = ?, updated_at = ?
		WHERE id = ? AND status = ?`

	now := time.Now()
	result, err := r.db.Exec(query, domain.RouteStatusCompleted, now, now, routeID, domain.RouteStatusActive)
	if err != nil {
		r.logger.Error("Failed to complete driver route", zap.Error(err), zap.String("route_id", routeID))
		return fmt.Errorf("failed to complete driver route: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("driver route not found or not active")
	}

	return nil
}

// CreateDriverMatch creates a match between a driver and delivery request
func (r *DriverRepository) CreateDriverMatch(match *domain.DriverMatch) (*domain.DriverMatch, error) {
	matchID := uuid.New().String() // Generate UUID for the match

	query := `
		INSERT INTO driver_matches (
			id, driver_id, driver_route_id, delivery_request_id, client_telegram_id,
			status, proposed_price, driver_comment, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()

	_, err := r.db.Exec(query,
		matchID, match.DriverID, match.DriverRouteID, match.DeliveryRequestID, match.ClientTelegramID,
		domain.MatchStatusPending, match.ProposedPrice, match.DriverComment, now, now,
	)

	if err != nil {
		r.logger.Error("Failed to create driver match", zap.Error(err))
		return nil, fmt.Errorf("failed to create driver match: %w", err)
	}

	return r.GetDriverMatchByID(matchID)
}

// GetDriverMatchByID retrieves a driver match by ID (UUID)
func (r *DriverRepository) GetDriverMatchByID(matchID string) (*domain.DriverMatch, error) {
	query := `
		SELECT id, driver_id, driver_route_id, delivery_request_id, client_telegram_id,
			   status, proposed_price, final_price, pickup_time, delivery_time,
			   driver_comment, client_comment, driver_rating, client_rating,
			   created_at, updated_at, completed_at
		FROM driver_matches 
		WHERE id = ?`

	match := &domain.DriverMatch{}
	var finalPrice sql.NullInt64
	var pickupTime, deliveryTime, completedAt sql.NullTime
	var driverRating, clientRating sql.NullInt64

	err := r.db.QueryRow(query, matchID).Scan(
		&match.ID, &match.DriverID, &match.DriverRouteID, &match.DeliveryRequestID, &match.ClientTelegramID,
		&match.Status, &match.ProposedPrice, &finalPrice, &pickupTime, &deliveryTime,
		&match.DriverComment, &match.ClientComment, &driverRating, &clientRating,
		&match.CreatedAt, &match.UpdatedAt, &completedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("driver match not found")
		}
		r.logger.Error("Failed to get driver match", zap.Error(err), zap.String("match_id", matchID))
		return nil, fmt.Errorf("failed to get driver match: %w", err)
	}

	if finalPrice.Valid {
		finalPriceInt := int(finalPrice.Int64)
		match.FinalPrice = &finalPriceInt
	}
	if pickupTime.Valid {
		match.PickupTime = &pickupTime.Time
	}
	if deliveryTime.Valid {
		match.DeliveryTime = &deliveryTime.Time
	}
	if completedAt.Valid {
		match.CompletedAt = &completedAt.Time
	}
	if driverRating.Valid {
		driverRatingInt := int(driverRating.Int64)
		match.DriverRating = &driverRatingInt
	}
	if clientRating.Valid {
		clientRatingInt := int(clientRating.Int64)
		match.ClientRating = &clientRatingInt
	}

	return match, nil
}

// UpdateDriverMatchStatus updates the status of a driver match
func (r *DriverRepository) UpdateDriverMatchStatus(matchID string, status string) error {
	query := `
		UPDATE driver_matches 
		SET status = ?, updated_at = ?
		WHERE id = ?`

	result, err := r.db.Exec(query, status, time.Now(), matchID)
	if err != nil {
		r.logger.Error("Failed to update driver match status", zap.Error(err), zap.String("match_id", matchID))
		return fmt.Errorf("failed to update driver match status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("driver match not found")
	}

	return nil
}

// GetDriverStatistics retrieves driver statistics
func (r *DriverRepository) GetDriverStatistics(telegramID int64) (*domain.DriverStatistics, error) {
	query := `
		SELECT 
			COUNT(dr.id) as total_routes,
			d.total_trips as completed_trips,
			d.total_earnings,
			d.rating as average_rating,
			d.created_at as joined_at,
			d.last_active_at
		FROM drivers d
		LEFT JOIN driver_routes dr ON d.id = dr.driver_id
		WHERE d.telegram_id = ?
		GROUP BY d.id, d.total_trips, d.total_earnings, d.rating, d.created_at, d.last_active_at`

	stats := &domain.DriverStatistics{}
	var lastActiveAt sql.NullTime

	err := r.db.QueryRow(query, telegramID).Scan(
		&stats.TotalRoutes, &stats.CompletedTrips, &stats.TotalEarnings,
		&stats.AverageRating, &stats.JoinedAt, &lastActiveAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return &domain.DriverStatistics{JoinedAt: time.Now()}, nil
		}
		r.logger.Error("Failed to get driver statistics", zap.Error(err), zap.Int64("telegram_id", telegramID))
		return nil, fmt.Errorf("failed to get driver statistics: %w", err)
	}

	if lastActiveAt.Valid {
		stats.LastActiveAt = &lastActiveAt.Time
	}

	// Calculate cancellation rate using driver ID lookup
	driverID, err := r.GetDriverIDByTelegramID(telegramID)
	if err == nil {
		cancelQuery := `
			SELECT 
				COUNT(CASE WHEN status = ? THEN 1 END) as cancelled_routes,
				COUNT(*) as total_routes
			FROM driver_routes 
			WHERE driver_id = ?`

		var cancelledRoutes, totalRoutes int
		err = r.db.QueryRow(cancelQuery, domain.RouteStatusCancelled, driverID).Scan(&cancelledRoutes, &totalRoutes)
		if err == nil && totalRoutes > 0 {
			stats.CancellationRate = float64(cancelledRoutes) / float64(totalRoutes)
		}
	}

	return stats, nil
}

// GetAllDrivers retrieves all drivers with pagination
func (r *DriverRepository) GetAllDrivers(limit, offset int) ([]*domain.Driver, error) {
	query := `
		SELECT id, telegram_id, telegram_username, first_name, last_name, birthday,
			   contact_number, start_city, latitude, longitude, profile_photo_path,
			   license_front_path, license_back_path, truck_photo_path, status,
			   is_approved, is_active, is_online, rating, total_trips, total_earnings,
			   has_whatsapp, has_telegram, approved_at, last_active_at, created_at, updated_at
		FROM drivers 
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		r.logger.Error("Failed to get all drivers", zap.Error(err))
		return nil, fmt.Errorf("failed to get drivers: %w", err)
	}
	defer rows.Close()

	var drivers []*domain.Driver
	for rows.Next() {
		driver := &domain.Driver{}
		var approvedAt, lastActiveAt sql.NullTime

		err := rows.Scan(
			&driver.ID, &driver.TelegramID, &driver.TelegramUsername, &driver.FirstName, &driver.LastName, &driver.Birthday,
			&driver.ContactNumber, &driver.StartCity, &driver.Latitude, &driver.Longitude, &driver.ProfilePhotoPath,
			&driver.LicenseFrontPath, &driver.LicenseBackPath, &driver.TruckPhotoPath, &driver.Status,
			&driver.IsApproved, &driver.IsActive, &driver.IsOnline, &driver.Rating, &driver.TotalTrips, &driver.TotalEarnings,
			&driver.HasWhatsapp, &driver.HasTelegram, &approvedAt, &lastActiveAt, &driver.CreatedAt, &driver.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan driver", zap.Error(err))
			continue
		}

		if approvedAt.Valid {
			driver.ApprovedAt = &approvedAt.Time
		}
		if lastActiveAt.Valid {
			driver.LastActiveAt = &lastActiveAt.Time
		}

		drivers = append(drivers, driver)
	}

	return drivers, nil
}

// GetPendingDrivers retrieves drivers pending approval
func (r *DriverRepository) GetPendingDrivers() ([]*domain.Driver, error) {
	query := `
		SELECT id, telegram_id, telegram_username, first_name, last_name, birthday,
			   contact_number, start_city, latitude, longitude, profile_photo_path,
			   license_front_path, license_back_path, truck_photo_path, status,
			   is_approved, is_active, is_online, rating, total_trips, total_earnings,
			   has_whatsapp, has_telegram, approved_at, last_active_at, created_at, updated_at
		FROM drivers 
		WHERE status = ? AND is_approved = false
		ORDER BY created_at ASC`

	rows, err := r.db.Query(query, domain.DriverStatusPending)
	if err != nil {
		r.logger.Error("Failed to get pending drivers", zap.Error(err))
		return nil, fmt.Errorf("failed to get pending drivers: %w", err)
	}
	defer rows.Close()

	var drivers []*domain.Driver
	for rows.Next() {
		driver := &domain.Driver{}
		var approvedAt, lastActiveAt sql.NullTime

		err := rows.Scan(
			&driver.ID, &driver.TelegramID, &driver.TelegramUsername, &driver.FirstName, &driver.LastName, &driver.Birthday,
			&driver.ContactNumber, &driver.StartCity, &driver.Latitude, &driver.Longitude, &driver.ProfilePhotoPath,
			&driver.LicenseFrontPath, &driver.LicenseBackPath, &driver.TruckPhotoPath, &driver.Status,
			&driver.IsApproved, &driver.IsActive, &driver.IsOnline, &driver.Rating, &driver.TotalTrips, &driver.TotalEarnings,
			&driver.HasWhatsapp, &driver.HasTelegram, &approvedAt, &lastActiveAt, &driver.CreatedAt, &driver.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan driver", zap.Error(err))
			continue
		}

		if approvedAt.Valid {
			driver.ApprovedAt = &approvedAt.Time
		}
		if lastActiveAt.Valid {
			driver.LastActiveAt = &lastActiveAt.Time
		}

		drivers = append(drivers, driver)
	}

	return drivers, nil
}

// UpdateDriver updates driver information
func (r *DriverRepository) UpdateDriver(telegramID int64, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return fmt.Errorf("no updates provided")
	}

	setParts := make([]string, 0, len(updates))
	args := make([]interface{}, 0, len(updates)+2)

	for field, value := range updates {
		setParts = append(setParts, field+" = ?")
		args = append(args, value)
	}

	query := fmt.Sprintf("UPDATE drivers SET %s, updated_at = ? WHERE telegram_id = ?",
		setParts[0])
	for i := 1; i < len(setParts); i++ {
		query = fmt.Sprintf("%s, %s", query, setParts[i])
	}

	args = append(args, time.Now(), telegramID)

	result, err := r.db.Exec(query, args...)
	if err != nil {
		r.logger.Error("Failed to update driver", zap.Error(err), zap.Int64("telegram_id", telegramID))
		return fmt.Errorf("failed to update driver: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("driver not found")
	}

	return nil
}

// DeactivateExpiredRoutes deactivates routes that have passed their departure time
func (r *DriverRepository) DeactivateExpiredRoutes() error {
	query := `
		UPDATE driver_routes 
		SET status = ?, is_active = false, updated_at = ?
		WHERE departure_time <= ? AND status = ? AND is_active = true`

	result, err := r.db.Exec(query, domain.RouteStatusCancelled, time.Now(), time.Now(), domain.RouteStatusActive)
	if err != nil {
		r.logger.Error("Failed to deactivate expired routes", zap.Error(err))
		return fmt.Errorf("failed to deactivate expired routes: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected > 0 {
		r.logger.Info("Deactivated expired routes", zap.Int64("count", rowsAffected))
	}

	return nil
}
