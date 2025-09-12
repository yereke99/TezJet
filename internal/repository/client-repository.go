package repository

import (
	"database/sql"
	"fmt"
	"tezjet/internal/domain"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type UserRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewUserRepository(db *sql.DB, logger *zap.Logger) *UserRepository {
	return &UserRepository{
		db:     db,
		logger: logger,
	}
}

// CreateUser creates a new user in the database
func (r *UserRepository) CreateUser(req *domain.CreateUserRequest) (*domain.User, error) {
	userID := uuid.New().String() // Generate UUID for the user

	query := `
		INSERT INTO users (
			id, telegram_id, telegram_username, first_name, last_name, 
			phone_number, language_code, is_active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()

	_, err := r.db.Exec(query,
		userID, req.TelegramID, req.TelegramUsername, req.FirstName, req.LastName,
		req.PhoneNumber, req.LanguageCode, true, now, now,
	)

	if err != nil {
		r.logger.Error("Failed to create user", zap.Error(err))
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Return the created user
	return r.GetUserByID(userID)
}

// GetUserByID retrieves a user by their database ID (UUID)
func (r *UserRepository) GetUserByID(userID string) (*domain.User, error) {
	query := `
		SELECT id, telegram_id, telegram_username, first_name, last_name, 
			   phone_number, language_code, is_active, offerta_accepted, 
			   offerta_accepted_at, created_at, updated_at
		FROM users 
		WHERE id = ?`

	user := &domain.User{}
	var offertaAcceptedAt sql.NullTime

	err := r.db.QueryRow(query, userID).Scan(
		&user.ID, &user.TelegramID, &user.TelegramUsername, &user.FirstName, &user.LastName,
		&user.PhoneNumber, &user.LanguageCode, &user.IsActive, &user.OffertaAccepted,
		&offertaAcceptedAt, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		r.logger.Error("Failed to get user by ID", zap.Error(err), zap.String("user_id", userID))
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if offertaAcceptedAt.Valid {
		user.OffertaAcceptedAt = &offertaAcceptedAt.Time
	}

	return user, nil
}

// GetUserByTelegramID retrieves a user by their Telegram ID
func (r *UserRepository) GetUserByTelegramID(telegramID int64) (*domain.User, error) {
	query := `
		SELECT id, telegram_id, telegram_username, first_name, last_name, 
			   phone_number, language_code, is_active, offerta_accepted, 
			   offerta_accepted_at, created_at, updated_at
		FROM users 
		WHERE telegram_id = ?`

	user := &domain.User{}
	var offertaAcceptedAt sql.NullTime

	err := r.db.QueryRow(query, telegramID).Scan(
		&user.ID, &user.TelegramID, &user.TelegramUsername, &user.FirstName, &user.LastName,
		&user.PhoneNumber, &user.LanguageCode, &user.IsActive, &user.OffertaAccepted,
		&offertaAcceptedAt, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		r.logger.Error("Failed to get user by Telegram ID", zap.Error(err), zap.Int64("telegram_id", telegramID))
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if offertaAcceptedAt.Valid {
		user.OffertaAcceptedAt = &offertaAcceptedAt.Time
	}

	return user, nil
}

// CheckUserExists checks if a user exists by Telegram ID and returns acceptance status
func (r *UserRepository) CheckUserExists(telegramID int64) (bool, bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE telegram_id = ?), 
			  COALESCE((SELECT offerta_accepted FROM users WHERE telegram_id = ?), false)`

	var exists, offertaAccepted bool
	err := r.db.QueryRow(query, telegramID, telegramID).Scan(&exists, &offertaAccepted)

	if err != nil {
		r.logger.Error("Failed to check user exists", zap.Error(err), zap.Int64("telegram_id", telegramID))
		return false, false, fmt.Errorf("failed to check user exists: %w", err)
	}

	return exists, offertaAccepted, nil
}

// AcceptOfferta marks that the user has accepted the offerta
func (r *UserRepository) AcceptOfferta(telegramID int64) error {
	query := `
		UPDATE users 
		SET offerta_accepted = true, offerta_accepted_at = ?, updated_at = ?
		WHERE telegram_id = ?`

	now := time.Now()
	result, err := r.db.Exec(query, now, now, telegramID)
	if err != nil {
		r.logger.Error("Failed to accept offerta", zap.Error(err), zap.Int64("telegram_id", telegramID))
		return fmt.Errorf("failed to accept offerta: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// CreateDeliveryRequest creates a new delivery request
func (r *UserRepository) CreateDeliveryRequest(req *domain.CreateDeliveryRequestReq, userID string, distance float64) (*domain.DeliveryRequest, error) {
	requestID := uuid.New().String() // Generate UUID for the delivery request

	query := `
		INSERT INTO delivery_requests (
			id, user_id, telegram_id, from_address, from_lat, from_lon, 
			to_address, to_lat, to_lon, price, contact, comment, 
			truck_type, distance_km, status, item_photo_path, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()

	_, err := r.db.Exec(query,
		requestID, userID, req.TelegramID, req.FromAddress, req.FromLat, req.FromLon,
		req.ToAddress, req.ToLat, req.ToLon, req.Price, req.Contact, req.Comment,
		req.TruckType, distance, domain.DeliveryStatusPending, req.ItemPhotoPath, now, now,
	)

	if err != nil {
		r.logger.Error("Failed to create delivery request", zap.Error(err))
		return nil, fmt.Errorf("failed to create delivery request: %w", err)
	}

	return r.GetDeliveryRequestByID(requestID)
}

// GetDeliveryRequestByID retrieves a delivery request by ID (UUID)
func (r *UserRepository) GetDeliveryRequestByID(requestID string) (*domain.DeliveryRequest, error) {
	query := `
		SELECT id, user_id, telegram_id, from_address, from_lat, from_lon,
			   to_address, to_lat, to_lon, price, contact, comment,
			   truck_type, distance_km, status, item_photo_path,
			   matched_driver_id, completed_at, created_at, updated_at
		FROM delivery_requests 
		WHERE id = ?`

	request := &domain.DeliveryRequest{}
	var matchedDriverID sql.NullString // Changed from sql.NullInt64 to sql.NullString
	var completedAt sql.NullTime

	err := r.db.QueryRow(query, requestID).Scan(
		&request.ID, &request.UserID, &request.TelegramID, &request.FromAddress, &request.FromLat, &request.FromLon,
		&request.ToAddress, &request.ToLat, &request.ToLon, &request.Price, &request.Contact, &request.Comment,
		&request.TruckType, &request.DistanceKm, &request.Status, &request.ItemPhotoPath,
		&matchedDriverID, &completedAt, &request.CreatedAt, &request.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("delivery request not found")
		}
		r.logger.Error("Failed to get delivery request", zap.Error(err), zap.String("request_id", requestID))
		return nil, fmt.Errorf("failed to get delivery request: %w", err)
	}

	if matchedDriverID.Valid {
		request.MatchedDriverID = &matchedDriverID.String // Fixed: Now using String instead of Int64
	}
	if completedAt.Valid {
		request.CompletedAt = &completedAt.Time
	}

	return request, nil
}

// GetUserDeliveryRequests retrieves all delivery requests for a user
func (r *UserRepository) GetUserDeliveryRequests(telegramID int64, limit, offset int) ([]*domain.DeliveryRequest, error) {
	query := `
		SELECT id, user_id, telegram_id, from_address, from_lat, from_lon,
			   to_address, to_lat, to_lon, price, contact, comment,
			   truck_type, distance_km, status, item_photo_path,
			   matched_driver_id, completed_at, created_at, updated_at
		FROM delivery_requests 
		WHERE telegram_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`

	rows, err := r.db.Query(query, telegramID, limit, offset)
	if err != nil {
		r.logger.Error("Failed to get user delivery requests", zap.Error(err), zap.Int64("telegram_id", telegramID))
		return nil, fmt.Errorf("failed to get delivery requests: %w", err)
	}
	defer rows.Close()

	var requests []*domain.DeliveryRequest
	for rows.Next() {
		request := &domain.DeliveryRequest{}
		var matchedDriverID sql.NullString // Changed from sql.NullInt64 to sql.NullString
		var completedAt sql.NullTime

		err := rows.Scan(
			&request.ID, &request.UserID, &request.TelegramID, &request.FromAddress, &request.FromLat, &request.FromLon,
			&request.ToAddress, &request.ToLat, &request.ToLon, &request.Price, &request.Contact, &request.Comment,
			&request.TruckType, &request.DistanceKm, &request.Status, &request.ItemPhotoPath,
			&matchedDriverID, &completedAt, &request.CreatedAt, &request.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan delivery request", zap.Error(err))
			continue
		}

		if matchedDriverID.Valid {
			request.MatchedDriverID = &matchedDriverID.String // Fixed: Now using String instead of Int64
		}
		if completedAt.Valid {
			request.CompletedAt = &completedAt.Time
		}

		requests = append(requests, request)
	}

	return requests, nil
}

// UpdateDeliveryRequestStatus updates the status of a delivery request
func (r *UserRepository) UpdateDeliveryRequestStatus(requestID string, status string, driverID *string) error {
	var query string
	var args []interface{}

	if driverID != nil {
		query = `
			UPDATE delivery_requests 
			SET status = ?, matched_driver_id = ?, updated_at = ?
			WHERE id = ?`
		args = []interface{}{status, *driverID, time.Now(), requestID}
	} else {
		query = `
			UPDATE delivery_requests 
			SET status = ?, updated_at = ?
			WHERE id = ?`
		args = []interface{}{status, time.Now(), requestID}
	}

	result, err := r.db.Exec(query, args...)
	if err != nil {
		r.logger.Error("Failed to update delivery request status", zap.Error(err), zap.String("request_id", requestID))
		return fmt.Errorf("failed to update delivery request status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("delivery request not found")
	}

	return nil
}

// CompleteDeliveryRequest marks a delivery request as completed
func (r *UserRepository) CompleteDeliveryRequest(requestID string) error {
	query := `
		UPDATE delivery_requests 
		SET status = ?, completed_at = ?, updated_at = ?
		WHERE id = ? AND status = ?`

	now := time.Now()
	result, err := r.db.Exec(query, domain.DeliveryStatusCompleted, now, now, requestID, domain.DeliveryStatusInProgress)
	if err != nil {
		r.logger.Error("Failed to complete delivery request", zap.Error(err), zap.String("request_id", requestID))
		return fmt.Errorf("failed to complete delivery request: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("delivery request not found or not in progress")
	}

	return nil
}

// GetPendingDeliveryRequests retrieves all pending delivery requests
func (r *UserRepository) GetPendingDeliveryRequests(limit int) ([]*domain.DeliveryRequest, error) {
	query := `
		SELECT id, user_id, telegram_id, from_address, from_lat, from_lon,
			   to_address, to_lat, to_lon, price, contact, comment,
			   truck_type, distance_km, status, item_photo_path,
			   matched_driver_id, completed_at, created_at, updated_at
		FROM delivery_requests 
		WHERE status = ?
		ORDER BY created_at ASC
		LIMIT ?`

	rows, err := r.db.Query(query, domain.DeliveryStatusPending, limit)
	if err != nil {
		r.logger.Error("Failed to get pending delivery requests", zap.Error(err))
		return nil, fmt.Errorf("failed to get pending delivery requests: %w", err)
	}
	defer rows.Close()

	var requests []*domain.DeliveryRequest
	for rows.Next() {
		request := &domain.DeliveryRequest{}
		var matchedDriverID sql.NullString // Changed from sql.NullInt64 to sql.NullString
		var completedAt sql.NullTime

		err := rows.Scan(
			&request.ID, &request.UserID, &request.TelegramID, &request.FromAddress, &request.FromLat, &request.FromLon,
			&request.ToAddress, &request.ToLat, &request.ToLon, &request.Price, &request.Contact, &request.Comment,
			&request.TruckType, &request.DistanceKm, &request.Status, &request.ItemPhotoPath,
			&matchedDriverID, &completedAt, &request.CreatedAt, &request.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan delivery request", zap.Error(err))
			continue
		}

		if matchedDriverID.Valid {
			request.MatchedDriverID = &matchedDriverID.String // Fixed: Now using String instead of Int64
		}
		if completedAt.Valid {
			request.CompletedAt = &completedAt.Time
		}

		requests = append(requests, request)
	}

	return requests, nil
}

// GetUserStatistics retrieves user statistics
func (r *UserRepository) GetUserStatistics(telegramID int64) (*domain.UserStatistics, error) {
	query := `
		SELECT 
			COUNT(*) as total_requests,
			COUNT(CASE WHEN status = ? THEN 1 END) as completed_requests,
			COALESCE(SUM(CASE WHEN status = ? THEN price ELSE 0 END), 0) as total_spent,
			u.created_at as joined_at
		FROM delivery_requests dr
		RIGHT JOIN users u ON dr.telegram_id = u.telegram_id
		WHERE u.telegram_id = ?
		GROUP BY u.created_at`

	stats := &domain.UserStatistics{}
	err := r.db.QueryRow(query, domain.DeliveryStatusCompleted, domain.DeliveryStatusCompleted, telegramID).Scan(
		&stats.TotalRequests, &stats.CompletedRequests, &stats.TotalSpent, &stats.JoinedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return &domain.UserStatistics{JoinedAt: time.Now()}, nil
		}
		r.logger.Error("Failed to get user statistics", zap.Error(err), zap.Int64("telegram_id", telegramID))
		return nil, fmt.Errorf("failed to get user statistics: %w", err)
	}

	// Calculate average rating (this would require a ratings table)
	stats.AverageRating = 0.0 // Placeholder

	return stats, nil
}

// DeleteDeliveryRequest soft deletes a delivery request (marks as cancelled)
func (r *UserRepository) DeleteDeliveryRequest(requestID string, telegramID int64) error {
	query := `
		UPDATE delivery_requests 
		SET status = ?, updated_at = ?
		WHERE id = ? AND telegram_id = ? AND status IN (?, ?)`

	now := time.Now()
	result, err := r.db.Exec(query, domain.DeliveryStatusCancelled, now, requestID, telegramID,
		domain.DeliveryStatusPending, domain.DeliveryStatusMatched)
	if err != nil {
		r.logger.Error("Failed to cancel delivery request", zap.Error(err), zap.String("request_id", requestID))
		return fmt.Errorf("failed to cancel delivery request: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("delivery request not found or cannot be cancelled")
	}

	return nil
}

// UpdateUser updates user information
func (r *UserRepository) UpdateUser(telegramID int64, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return fmt.Errorf("no updates provided")
	}

	setParts := make([]string, 0, len(updates))
	args := make([]interface{}, 0, len(updates)+2)

	for field, value := range updates {
		setParts = append(setParts, field+" = ?")
		args = append(args, value)
	}

	query := fmt.Sprintf("UPDATE users SET %s, updated_at = ? WHERE telegram_id = ?",
		string(setParts[0]))
	for i := 1; i < len(setParts); i++ {
		query = fmt.Sprintf("%s, %s", query, setParts[i])
	}

	args = append(args, time.Now(), telegramID)

	result, err := r.db.Exec(query, args...)
	if err != nil {
		r.logger.Error("Failed to update user", zap.Error(err), zap.Int64("telegram_id", telegramID))
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// GetActiveDeliveryRequests retrieves active delivery requests for a user
func (r *UserRepository) GetActiveDeliveryRequests(telegramID int64) ([]*domain.DeliveryRequest, error) {
	query := `
		SELECT id, user_id, telegram_id, from_address, from_lat, from_lon,
			   to_address, to_lat, to_lon, price, contact, comment,
			   truck_type, distance_km, status, item_photo_path,
			   matched_driver_id, completed_at, created_at, updated_at
		FROM delivery_requests 
		WHERE telegram_id = ? AND status IN (?, ?, ?)
		ORDER BY created_at DESC`

	rows, err := r.db.Query(query, telegramID, domain.DeliveryStatusPending,
		domain.DeliveryStatusMatched, domain.DeliveryStatusInProgress)
	if err != nil {
		r.logger.Error("Failed to get active delivery requests", zap.Error(err), zap.Int64("telegram_id", telegramID))
		return nil, fmt.Errorf("failed to get active delivery requests: %w", err)
	}
	defer rows.Close()

	var requests []*domain.DeliveryRequest
	for rows.Next() {
		request := &domain.DeliveryRequest{}
		var matchedDriverID sql.NullString // Changed from sql.NullInt64 to sql.NullString
		var completedAt sql.NullTime

		err := rows.Scan(
			&request.ID, &request.UserID, &request.TelegramID, &request.FromAddress, &request.FromLat, &request.FromLon,
			&request.ToAddress, &request.ToLat, &request.ToLon, &request.Price, &request.Contact, &request.Comment,
			&request.TruckType, &request.DistanceKm, &request.Status, &request.ItemPhotoPath,
			&matchedDriverID, &completedAt, &request.CreatedAt, &request.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan delivery request", zap.Error(err))
			continue
		}

		if matchedDriverID.Valid {
			request.MatchedDriverID = &matchedDriverID.String // Fixed: Now using String instead of Int64
		}
		if completedAt.Valid {
			request.CompletedAt = &completedAt.Time
		}

		requests = append(requests, request)
	}

	return requests, nil
}

// Helper method to get user ID by telegram ID
func (r *UserRepository) GetUserIDByTelegramID(telegramID int64) (string, error) {
	query := `SELECT id FROM users WHERE telegram_id = ?`

	var userID string
	err := r.db.QueryRow(query, telegramID).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("user not found")
		}
		return "", fmt.Errorf("failed to get user ID: %w", err)
	}

	return userID, nil
}
