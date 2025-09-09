package domain

import (
	"time"
)

// User represents a client user in the system
type User struct {
	ID                int64      `json:"id" db:"id"`
	TelegramID        int64      `json:"telegram_id" db:"telegram_id"`
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
	ID              int64      `json:"id" db:"id"`
	UserID          int64      `json:"user_id" db:"user_id"`
	TelegramID      int64      `json:"telegram_id" db:"telegram_id"`
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
	MatchedDriverID *int64     `json:"matched_driver_id" db:"matched_driver_id"`
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
	RequestID int64  `json:"request_id" validate:"required"`
	Status    string `json:"status" validate:"required"`
	DriverID  *int64 `json:"driver_id"`
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

// Validation methods
func (u *User) IsValid() bool {
	return u.TelegramID > 0 && u.FirstName != ""
}

func (dr *DeliveryRequest) IsValid() bool {
	return dr.TelegramID > 0 && dr.FromAddress != "" && dr.ToAddress != "" &&
		dr.Price >= 2000 && dr.Contact != "" && dr.TruckType != ""
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
