package domain

import (
	"time"

	"github.com/google/uuid"
)

// Driver represents a driver in the system
type Driver struct {
	ID               string     `json:"id" db:"id"`                   // Changed from int64 to string (UUID)
	TelegramID       int64      `json:"telegram_id" db:"telegram_id"` // Kept as int64 for Telegram API
	TelegramUsername string     `json:"telegram_username" db:"telegram_username"`
	FirstName        string     `json:"first_name" db:"first_name"`
	LastName         string     `json:"last_name" db:"last_name"`
	Birthday         time.Time  `json:"birthday" db:"birthday"`
	ContactNumber    string     `json:"contact_number" db:"contact_number"`
	StartCity        string     `json:"start_city" db:"start_city"`
	Latitude         float64    `json:"latitude" db:"latitude"`
	Longitude        float64    `json:"longitude" db:"longitude"`
	ProfilePhotoPath string     `json:"profile_photo_path" db:"profile_photo_path"`
	LicenseFrontPath string     `json:"license_front_path" db:"license_front_path"`
	LicenseBackPath  string     `json:"license_back_path" db:"license_back_path"`
	TruckPhotoPath   string     `json:"truck_photo_path" db:"truck_photo_path"`
	TruckType        string     `json:"truck_type" db:"truck_photo_path"`
	Status           string     `json:"status" db:"status"` // pending, approved, rejected, suspended
	IsApproved       bool       `json:"is_approved" db:"is_approved"`
	IsActive         bool       `json:"is_active" db:"is_active"`
	IsOnline         bool       `json:"is_online" db:"is_online"`
	Rating           float64    `json:"rating" db:"rating"`
	TotalTrips       int        `json:"total_trips" db:"total_trips"`
	TotalEarnings    int        `json:"total_earnings" db:"total_earnings"`
	HasWhatsapp      bool       `json:"has_whatsapp" db:"has_whatsapp"`
	HasTelegram      bool       `json:"has_telegram" db:"has_telegram"`
	ApprovedAt       *time.Time `json:"approved_at" db:"approved_at"`
	LastActiveAt     *time.Time `json:"last_active_at" db:"last_active_at"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`

	FromLat float64 `json:"from_lat" db:"from_lat"`
	FromLon float64 `json:"from_lon" db:"from_lon"`
	ToLat   float64 `json:"to_lat" db:"to_lat"`
	ToLon   float64 `json:"to_lon" db:"to_lon"`
	// Alternative field names
	PickupLat  float64 `json:"pickup_lat" db:"pickup_lat"`
	PickupLon  float64 `json:"pickup_lon" db:"pickup_lon"`
	DropoffLat float64 `json:"dropoff_lat" db:"dropoff_lat"`
	DropoffLon float64 `json:"dropoff_lon" db:"dropoff_lon"`
}

// DriverRoute represents a route offered by a driver
type DriverRoute struct {
	ID             string     `json:"id" db:"id"`                   // Changed from int64 to string (UUID)
	DriverID       string     `json:"driver_id" db:"driver_id"`     // Changed from int64 to string (UUID)
	TelegramID     int64      `json:"telegram_id" db:"telegram_id"` // Kept as int64 for Telegram API
	FromAddress    string     `json:"from_address" db:"from_address"`
	FromLat        float64    `json:"from_lat" db:"from_lat"`
	FromLon        float64    `json:"from_lon" db:"from_lon"`
	ToAddress      string     `json:"to_address" db:"to_address"`
	ToLat          float64    `json:"to_lat" db:"to_lat"`
	ToLon          float64    `json:"to_lon" db:"to_lon"`
	Price          int        `json:"price" db:"price"`
	TruckType      string     `json:"truck_type" db:"truck_type"`
	MaxWeight      int        `json:"max_weight" db:"max_weight"` // in kg
	Comment        string     `json:"comment" db:"comment"`
	DepartureTime  time.Time  `json:"departure_time" db:"departure_time"`
	ArrivalTime    *time.Time `json:"arrival_time" db:"arrival_time"`
	DistanceKm     float64    `json:"distance_km" db:"distance_km"`
	Status         string     `json:"status" db:"status"` // active, matched, completed, cancelled
	AvailableSeats int        `json:"available_seats" db:"available_seats"`
	IsActive       bool       `json:"is_active" db:"is_active"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

// DriverMatch represents a match between a driver route and delivery request
type DriverMatch struct {
	ID                string     `json:"id" db:"id"`                                   // Changed from int64 to string (UUID)
	DriverID          string     `json:"driver_id" db:"driver_id"`                     // Changed from int64 to string (UUID)
	DriverRouteID     string     `json:"driver_route_id" db:"driver_route_id"`         // Changed from int64 to string (UUID)
	DeliveryRequestID string     `json:"delivery_request_id" db:"delivery_request_id"` // Changed from int64 to string (UUID)
	ClientTelegramID  int64      `json:"client_telegram_id" db:"client_telegram_id"`   // Kept as int64 for Telegram API
	Status            string     `json:"status" db:"status"`                           // pending, accepted, rejected, completed
	ProposedPrice     int        `json:"proposed_price" db:"proposed_price"`
	FinalPrice        *int       `json:"final_price" db:"final_price"`
	PickupTime        *time.Time `json:"pickup_time" db:"pickup_time"`
	DeliveryTime      *time.Time `json:"delivery_time" db:"delivery_time"`
	DriverComment     string     `json:"driver_comment" db:"driver_comment"`
	ClientComment     string     `json:"client_comment" db:"client_comment"`
	DriverRating      *int       `json:"driver_rating" db:"driver_rating"` // 1-5
	ClientRating      *int       `json:"client_rating" db:"client_rating"` // 1-5
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
	CompletedAt       *time.Time `json:"completed_at" db:"completed_at"`
}

// CreateDriverRequest represents a request to create a new driver
type CreateDriverRequest struct {
	TelegramID       int64   `json:"telegram_id" validate:"required"`
	TelegramUsername string  `json:"telegram_username"`
	FirstName        string  `json:"first_name" validate:"required"`
	LastName         string  `json:"last_name" validate:"required"`
	Birthday         string  `json:"birthday" validate:"required"` // YYYY-MM-DD format
	ContactNumber    string  `json:"contact_number" validate:"required"`
	StartCity        string  `json:"start_city" validate:"required"`
	Latitude         float64 `json:"latitude" validate:"required"`
	Longitude        float64 `json:"longitude" validate:"required"`
	HasWhatsapp      bool    `json:"has_whatsapp"`
	HasTelegram      bool    `json:"has_telegram"`
}

// CreateDriverRouteRequest represents a request to create a driver route
type CreateDriverRouteRequest struct {
	TelegramID     int64     `json:"telegram_id" validate:"required"`
	FromAddress    string    `json:"from_address" validate:"required"`
	FromLat        float64   `json:"from_lat" validate:"required"`
	FromLon        float64   `json:"from_lon" validate:"required"`
	ToAddress      string    `json:"to_address" validate:"required"`
	ToLat          float64   `json:"to_lat" validate:"required"`
	ToLon          float64   `json:"to_lon" validate:"required"`
	Price          int       `json:"price" validate:"required,min=2000"`
	TruckType      string    `json:"truck_type" validate:"required"`
	MaxWeight      int       `json:"max_weight"`
	Comment        string    `json:"comment"`
	DepartureTime  time.Time `json:"departure_time" validate:"required"`
	AvailableSeats int       `json:"available_seats" validate:"min=1"`
}

// UpdateDriverRequest represents a request to update driver information
type UpdateDriverRequest struct {
	ContactNumber string  `json:"contact_number"`
	StartCity     string  `json:"start_city"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	HasWhatsapp   *bool   `json:"has_whatsapp"`
	HasTelegram   *bool   `json:"has_telegram"`
	IsActive      *bool   `json:"is_active"`
	IsOnline      *bool   `json:"is_online"`
}

// DriverStatistics represents driver usage statistics
type DriverStatistics struct {
	TotalRoutes      int        `json:"total_routes"`
	CompletedTrips   int        `json:"completed_trips"`
	TotalEarnings    int        `json:"total_earnings"`
	AverageRating    float64    `json:"average_rating"`
	CancellationRate float64    `json:"cancellation_rate"`
	JoinedAt         time.Time  `json:"joined_at"`
	LastActiveAt     *time.Time `json:"last_active_at"`
}

// DriverWithRoute represents a driver with their active route information
type DriverWithRoute struct {
	Driver   Driver       `json:"driver"`
	Route    *DriverRoute `json:"route,omitempty"`
	Distance float64      `json:"distance,omitempty"` // Distance from pickup point
}

// MatchedDriver represents a driver matched to a delivery request
type MatchedDriver struct {
	DriverID         string    `json:"driver_id"` // Changed from int64 to string (UUID)
	FullName         string    `json:"full_name"`
	ProfilePhoto     string    `json:"profile_photo"`
	ContactNumber    string    `json:"contact"`
	Rating           float64   `json:"rating"`
	TotalTrips       int       `json:"total_trips"`
	FromAddress      string    `json:"from_address"`
	ToAddress        string    `json:"to_address"`
	Price            int       `json:"price"`
	TruckType        string    `json:"truck_type"`
	Comment          string    `json:"comment"`
	DepartureTime    time.Time `json:"departure_time"`
	TruckPhoto       string    `json:"truck_photo"`
	HasWhatsapp      bool      `json:"has_whatsapp"`
	HasTelegram      bool      `json:"has_telegram"`
	TelegramUsername string    `json:"telegram_username"`
	FromLat          float64   `json:"from_lat"`
	FromLon          float64   `json:"from_lon"`
	ToLat            float64   `json:"to_lat"`
	ToLon            float64   `json:"to_lon"`
}

type NearADriver struct {
	MinLat  float64
	MaxLat  float64
	MinLong float64
	MaxLong float64
}

// Driver Status constants
const (
	DriverStatusPending   = "pending"
	DriverStatusApproved  = "approved"
	DriverStatusRejected  = "rejected"
	DriverStatusSuspended = "suspended"
)

// Route Status constants
const (
	RouteStatusActive    = "active"
	RouteStatusMatched   = "matched"
	RouteStatusCompleted = "completed"
	RouteStatusCancelled = "cancelled"
)

// Match Status constants
const (
	MatchStatusPending   = "pending"
	MatchStatusAccepted  = "accepted"
	MatchStatusRejected  = "rejected"
	MatchStatusCompleted = "completed"
)

// Helper functions for UUID operations
func GenerateDriverID() string {
	return uuid.New().String()
}

func IsValidDriverID(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}

// Validation methods
func (d *Driver) IsValid() bool {
	return d.TelegramID > 0 && d.FirstName != "" && d.LastName != "" &&
		d.ContactNumber != "" && d.StartCity != "" && IsValidDriverID(d.ID)
}

func (d *Driver) IsValidAge() bool {
	age := time.Now().Year() - d.Birthday.Year()
	if time.Now().YearDay() < d.Birthday.YearDay() {
		age--
	}
	return age >= 18
}

func (dr *DriverRoute) IsValid() bool {
	return IsValidDriverID(dr.DriverID) && dr.FromAddress != "" && dr.ToAddress != "" &&
		dr.Price >= 2000 && !dr.DepartureTime.IsZero() && IsValidDriverID(dr.ID)
}

func (dr *DriverRoute) IsValidCoordinates() bool {
	return dr.FromLat != 0 && dr.FromLon != 0 && dr.ToLat != 0 && dr.ToLon != 0
}

func (dr *DriverRoute) IsExpired() bool {
	return time.Now().After(dr.DepartureTime)
}

func (dr *DriverRoute) CanBeUpdated() bool {
	return dr.Status == RouteStatusActive && !dr.IsExpired()
}

// Helper methods
func (d *Driver) GetFullName() string {
	if d.LastName != "" {
		return d.FirstName + " " + d.LastName
	}
	return d.FirstName
}

func (d *Driver) CanTakeOrders() bool {
	return d.IsApproved && d.IsActive && d.IsOnline
}

func (dm *DriverMatch) CanBeAccepted() bool {
	return dm.Status == MatchStatusPending
}

func (dm *DriverMatch) CanBeCompleted() bool {
	return dm.Status == MatchStatusAccepted
}

func (dm *DriverMatch) IsCompleted() bool {
	return dm.Status == MatchStatusCompleted && dm.CompletedAt != nil
}

func (dm *DriverMatch) IsValid() bool {
	return IsValidDriverID(dm.ID) && IsValidDriverID(dm.DriverID) &&
		IsValidDriverID(dm.DriverRouteID) && IsValidDriverID(dm.DeliveryRequestID) &&
		dm.ClientTelegramID > 0
}

// SetID generates and sets a new UUID for the driver if not already set
func (d *Driver) SetID() {
	if d.ID == "" {
		d.ID = GenerateDriverID()
	}
}

// SetID generates and sets a new UUID for the driver route if not already set
func (dr *DriverRoute) SetID() {
	if dr.ID == "" {
		dr.ID = GenerateDriverID()
	}
}

// SetID generates and sets a new UUID for the driver match if not already set
func (dm *DriverMatch) SetID() {
	if dm.ID == "" {
		dm.ID = GenerateDriverID()
	}
}
