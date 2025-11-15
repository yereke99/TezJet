package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config contains application configuration parameters
type Config struct {
	// Server configuration
	Port         string        `json:"port"`
	Host         string        `json:"host"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout"`

	// Telegram Bot configuration
	Token      string `json:"token"`
	BaseURL    string `json:"base_url"`
	WebhookURL string `json:"webhook_url"`
	AdminToken string `json:"admin_token"`

	// Database configuration
	DBName          string        `json:"db_name"`
	DBPath          string        `json:"db_path"`
	MaxOpenConns    int           `json:"max_open_conns"`
	MaxIdleConns    int           `json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
	CargoPhoto      string
	MaxUploadSize   int64

	// AdminId
	AdminTelegramID int64

	// File upload configuration
	UploadDir   string   `json:"upload_dir"`
	MaxFileSize int64    `json:"max_file_size"` // in bytes
	AllowedExts []string `json:"allowed_extensions"`

	// App configuration
	Environment string `json:"environment"` // development, production
	LogLevel    string `json:"log_level"`   // debug, info, warn, error

	// Business logic configuration
	MinPrice           int     `json:"min_price"`
	MaxDistance        float64 `json:"max_distance_km"` // for driver matching
	DefaultMatchRadius float64 `json:"default_match_radius_km"`

	// Rate limiting
	RateLimitRequests int           `json:"rate_limit_requests"`
	RateLimitWindow   time.Duration `json:"rate_limit_window"`
}

// NewConfig creates and returns a new configuration instance
func NewConfig() (*Config, error) {
	cfg := &Config{
		// Server defaults
		Port:         ":8081",
		Host:         "0.0.0.0",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,

		// Telegram defaults
		Token:      "8511244047:AAFHI2ikGyCimPwLviZ9ufN4Iprdp5FAfnU",
		BaseURL:    "https://meily.kz",
		AdminToken: "admin-secret-token-change-in-production",

		// Database defaults
		DBName:          "zholda.db",
		DBPath:          "./data/",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,

		// File upload defaults
		UploadDir:   "./uploads",
		MaxFileSize: 10 * 1024 * 1024, // 10MB
		AllowedExts: []string{".jpg", ".jpeg", ".png", ".pdf"},

		CargoPhoto:    "./delivery-photo",
		MaxUploadSize: 20 << 20,

		AdminTelegramID: 800703982,

		// App defaults
		Environment: "development",
		LogLevel:    "info",

		// Business defaults
		MinPrice:           2000,
		MaxDistance:        50.0, // 50km
		DefaultMatchRadius: 10.0, // 10km

		// Rate limiting defaults
		RateLimitRequests: 100,
		RateLimitWindow:   time.Hour,
	}

	// Override with environment variables if set
	if port := os.Getenv("PORT"); port != "" {
		if port[0] != ':' {
			cfg.Port = ":" + port
		} else {
			cfg.Port = port
		}
	}

	if host := os.Getenv("HOST"); host != "" {
		cfg.Host = host
	}

	if token := os.Getenv("BOT_TOKEN"); token != "" {
		cfg.Token = token
	}

	if baseURL := os.Getenv("BASE_URL"); baseURL != "" {
		cfg.BaseURL = baseURL
	}

	if webhookURL := os.Getenv("WEBHOOK_URL"); webhookURL != "" {
		cfg.WebhookURL = webhookURL
	}

	if adminToken := os.Getenv("ADMIN_TOKEN"); adminToken != "" {
		cfg.AdminToken = adminToken
	}

	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		cfg.DBName = dbName
	}

	if dbPath := os.Getenv("DB_PATH"); dbPath != "" {
		cfg.DBPath = dbPath
	}

	if uploadDir := os.Getenv("UPLOAD_DIR"); uploadDir != "" {
		cfg.UploadDir = uploadDir
	}

	if env := os.Getenv("ENVIRONMENT"); env != "" {
		cfg.Environment = env
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		cfg.LogLevel = logLevel
	}

	// Parse numeric environment variables
	if maxFileSize := os.Getenv("MAX_FILE_SIZE"); maxFileSize != "" {
		if size, err := strconv.ParseInt(maxFileSize, 10, 64); err == nil {
			cfg.MaxFileSize = size
		}
	}

	if minPrice := os.Getenv("MIN_PRICE"); minPrice != "" {
		if price, err := strconv.Atoi(minPrice); err == nil {
			cfg.MinPrice = price
		}
	}

	if maxDistance := os.Getenv("MAX_DISTANCE"); maxDistance != "" {
		if distance, err := strconv.ParseFloat(maxDistance, 64); err == nil {
			cfg.MaxDistance = distance
		}
	}

	if matchRadius := os.Getenv("DEFAULT_MATCH_RADIUS"); matchRadius != "" {
		if radius, err := strconv.ParseFloat(matchRadius, 64); err == nil {
			cfg.DefaultMatchRadius = radius
		}
	}

	if maxOpenConns := os.Getenv("DB_MAX_OPEN_CONNS"); maxOpenConns != "" {
		if conns, err := strconv.Atoi(maxOpenConns); err == nil {
			cfg.MaxOpenConns = conns
		}
	}

	if maxIdleConns := os.Getenv("DB_MAX_IDLE_CONNS"); maxIdleConns != "" {
		if conns, err := strconv.Atoi(maxIdleConns); err == nil {
			cfg.MaxIdleConns = conns
		}
	}

	if rateLimitRequests := os.Getenv("RATE_LIMIT_REQUESTS"); rateLimitRequests != "" {
		if requests, err := strconv.Atoi(rateLimitRequests); err == nil {
			cfg.RateLimitRequests = requests
		}
	}

	// Parse duration environment variables
	if readTimeout := os.Getenv("READ_TIMEOUT"); readTimeout != "" {
		if timeout, err := time.ParseDuration(readTimeout); err == nil {
			cfg.ReadTimeout = timeout
		}
	}

	if writeTimeout := os.Getenv("WRITE_TIMEOUT"); writeTimeout != "" {
		if timeout, err := time.ParseDuration(writeTimeout); err == nil {
			cfg.WriteTimeout = timeout
		}
	}

	if idleTimeout := os.Getenv("IDLE_TIMEOUT"); idleTimeout != "" {
		if timeout, err := time.ParseDuration(idleTimeout); err == nil {
			cfg.IdleTimeout = timeout
		}
	}

	if connMaxLifetime := os.Getenv("DB_CONN_MAX_LIFETIME"); connMaxLifetime != "" {
		if lifetime, err := time.ParseDuration(connMaxLifetime); err == nil {
			cfg.ConnMaxLifetime = lifetime
		}
	}

	if rateLimitWindow := os.Getenv("RATE_LIMIT_WINDOW"); rateLimitWindow != "" {
		if window, err := time.ParseDuration(rateLimitWindow); err == nil {
			cfg.RateLimitWindow = window
		}
	}

	return cfg, nil
}

// IsDevelopment returns true if the environment is development
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction returns true if the environment is production
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// GetDatabasePath returns the full path to the database file
func (c *Config) GetDatabasePath() string {
	return c.DBPath + c.DBName
}

// GetWebhookURL returns the webhook URL for Telegram
func (c *Config) GetWebhookURL() string {
	if c.WebhookURL != "" {
		return c.WebhookURL
	}
	return c.BaseURL + "/webhook"
}

// GetServerAddress returns the server address
func (c *Config) GetServerAddress() string {
	return c.Host + c.Port
}

// ValidateConfig validates the configuration
func (c *Config) ValidateConfig() error {
	if c.Token == "" {
		return fmt.Errorf("telegram bot token is required")
	}

	if c.BaseURL == "" {
		return fmt.Errorf("base URL is required")
	}

	if c.DBName == "" {
		return fmt.Errorf("database name is required")
	}

	if c.MinPrice < 0 {
		return fmt.Errorf("minimum price cannot be negative")
	}

	if c.MaxDistance <= 0 {
		return fmt.Errorf("maximum distance must be positive")
	}

	if c.DefaultMatchRadius <= 0 {
		return fmt.Errorf("default match radius must be positive")
	}

	if c.MaxFileSize <= 0 {
		return fmt.Errorf("maximum file size must be positive")
	}

	return nil
}

// GetAllowedExtensions returns the list of allowed file extensions
func (c *Config) GetAllowedExtensions() []string {
	return c.AllowedExts
}

// IsAllowedExtension checks if a file extension is allowed
func (c *Config) IsAllowedExtension(ext string) bool {
	for _, allowedExt := range c.AllowedExts {
		if allowedExt == ext {
			return true
		}
	}
	return false
}
