package domain

import (
	"time"

	"github.com/google/uuid"
)

// User represents a client user in the system
type User struct {
	ID                string     `json:"id" db:"id"`                   // Changed from int64 to string (UUID)
	TelegramID        int64      `json:"telegram_id" db:"telegram_id"` // Kept as int64 for Telegram API
	TelegramUsername  string     `json:"telegram_username" db:"telegram_username"`
	FirstName         string     `json:"first_name" db:"first_name"`
	LastName          string     `json:"last_name" db:"last_name"`
	PhoneNumber       string     `json:"phone_number" db:"phone_number"`
	LanguageCode      string     `json:"language_code" db:"language_code"`
	IsActive          bool       `json:"is_active" db:"is_active"`
	OffertaAccepted   bool       `json:"offerta_accepted" db:"offerta_accepted"`
	OffertaAcceptedAt *time.Time `json:"offerta_accepted_at" db:"offerta_accepted_at"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
}

// DeliveryRequest represents a delivery request from a client
type DeliveryRequest struct {
	ID              string     `json:"id" db:"id"`                   // Changed from int64 to string (UUID)
	UserID          string     `json:"user_id" db:"user_id"`         // Changed from int64 to string (UUID)
	TelegramID      int64      `json:"telegram_id" db:"telegram_id"` // Kept as int64 for Telegram API
	FromAddress     string     `json:"from_address" db:"from_address"`
	FromLat         float64    `json:"from_lat" db:"from_lat"`
	FromLon         float64    `json:"from_lon" db:"from_lon"`
	ToAddress       string     `json:"to_address" db:"to_address"`
	ToLat           float64    `json:"to_lat" db:"to_lat"`
	ToLon           float64    `json:"to_lon" db:"to_lon"`
	Price           int        `json:"price" db:"price"`
	Contact         string     `json:"contact" db:"contact"`
	Comment         string     `json:"comment" db:"comment"`
	TruckType       string     `json:"truck_type" db:"truck_type"`
	DistanceKm      float64    `json:"distance_km" db:"distance_km"`
	Status          string     `json:"status" db:"status"` // pending, matched, completed, cancelled
	ItemPhotoPath   string     `json:"item_photo_path" db:"item_photo_path"`
	MatchedDriverID *string    `json:"matched_driver_id" db:"matched_driver_id"` // Changed from *int64 to *string (UUID)
	CompletedAt     *time.Time `json:"completed_at" db:"completed_at"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

// CreateUserRequest represents a request to create a new user
type CreateUserRequest struct {
	TelegramID       int64  `json:"telegram_id" validate:"required"`
	TelegramUsername string `json:"telegram_username"`
	FirstName        string `json:"first_name" validate:"required"`
	LastName         string `json:"last_name"`
	PhoneNumber      string `json:"phone_number"`
	LanguageCode     string `json:"language_code"`
}

// CreateDeliveryRequestReq represents a request to create a delivery request
type CreateDeliveryRequestReq struct {
	TelegramID    int64   `json:"telegram_id" validate:"required"`
	FromAddress   string  `json:"from_address" validate:"required"`
	FromLat       float64 `json:"from_lat" validate:"required"`
	FromLon       float64 `json:"from_lon" validate:"required"`
	ToAddress     string  `json:"to_address" validate:"required"`
	ToLat         float64 `json:"to_lat" validate:"required"`
	ToLon         float64 `json:"to_lon" validate:"required"`
	Price         int     `json:"price" validate:"required,min=2000"`
	Contact       string  `json:"contact" validate:"required"`
	Comment       string  `json:"comment"`
	TruckType     string  `json:"truck_type" validate:"required"`
	ItemPhotoPath string  `json:"item_photo_path"`
}

// UpdateDeliveryRequestStatus represents a request to update delivery status
type UpdateDeliveryRequestStatus struct {
	RequestID string  `json:"request_id" validate:"required"` // Changed from int64 to string (UUID)
	Status    string  `json:"status" validate:"required"`
	DriverID  *string `json:"driver_id"` // Changed from *int64 to *string (UUID)
}

// UserStatistics represents user usage statistics
type UserStatistics struct {
	TotalRequests     int       `json:"total_requests"`
	CompletedRequests int       `json:"completed_requests"`
	TotalSpent        int       `json:"total_spent"`
	AverageRating     float64   `json:"average_rating"`
	JoinedAt          time.Time `json:"joined_at"`
}

// TruckType constants
const (
	TruckTypeSmall        = "small"
	TruckTypeMedium       = "medium"
	TruckTypeLarge        = "large"
	TruckTypeRefrigerator = "refrigerator"
	TruckTypeTow          = "tow"
)

// DeliveryStatus constants
const (
	DeliveryStatusPending    = "pending"
	DeliveryStatusMatched    = "matched"
	DeliveryStatusInProgress = "in_progress"
	DeliveryStatusCompleted  = "completed"
	DeliveryStatusCancelled  = "cancelled"
)

// Helper functions for UUID operations
func GenerateUserID() string {
	return uuid.New().String()
}

func GenerateDeliveryRequestID() string {
	return uuid.New().String()
}

func IsValidUserID(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}

func IsValidDeliveryRequestID(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}

// Validation methods
func (u *User) IsValid() bool {
	return u.TelegramID > 0 && u.FirstName != "" && IsValidUserID(u.ID)
}

func (dr *DeliveryRequest) IsValid() bool {
	return dr.TelegramID > 0 && dr.FromAddress != "" && dr.ToAddress != "" &&
		dr.Price >= 2000 && dr.Contact != "" && dr.TruckType != "" &&
		IsValidDeliveryRequestID(dr.ID) && IsValidUserID(dr.UserID)
}

func (dr *DeliveryRequest) IsValidCoordinates() bool {
	return dr.FromLat != 0 && dr.FromLon != 0 && dr.ToLat != 0 && dr.ToLon != 0
}

// Helper methods
func (dr *DeliveryRequest) CanBeUpdated() bool {
	return dr.Status == DeliveryStatusPending || dr.Status == DeliveryStatusMatched
}

func (dr *DeliveryRequest) CanBeCompleted() bool {
	return dr.Status == DeliveryStatusInProgress
}

func (dr *DeliveryRequest) CanBeCancelled() bool {
	return dr.Status == DeliveryStatusPending || dr.Status == DeliveryStatusMatched
}

func (dr *DeliveryRequest) HasMatchedDriver() bool {
	return dr.MatchedDriverID != nil && *dr.MatchedDriverID != ""
}

func (dr *DeliveryRequest) IsMatchedToDriver(driverID string) bool {
	return dr.MatchedDriverID != nil && *dr.MatchedDriverID == driverID
}

// SetID generates and sets a new UUID for the user if not already set
func (u *User) SetID() {
	if u.ID == "" {
		u.ID = GenerateUserID()
	}
}

// SetID generates and sets a new UUID for the delivery request if not already set
func (dr *DeliveryRequest) SetID() {
	if dr.ID == "" {
		dr.ID = GenerateDeliveryRequestID()
	}
}

// GetFullName returns the full name of the user
func (u *User) GetFullName() string {
	if u.LastName != "" {
		return u.FirstName + " " + u.LastName
	}
	return u.FirstName
}

// SetMatchedDriver assigns a driver to the delivery request
func (dr *DeliveryRequest) SetMatchedDriver(driverID string) {
	if IsValidUserID(driverID) {
		dr.MatchedDriverID = &driverID
		dr.Status = DeliveryStatusMatched
	}
}

// ClearMatchedDriver removes the matched driver from the delivery request
func (dr *DeliveryRequest) ClearMatchedDriver() {
	dr.MatchedDriverID = nil
	dr.Status = DeliveryStatusPending
}
