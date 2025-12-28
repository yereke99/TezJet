// traits/database/database.go
package database

import (
	"database/sql"
	"os"
	"tezjet/config"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

var _ = time.Second

// InitDatabase initializes the SQLite database
func InitDatabase(cfg *config.Config, logger *zap.Logger) (*sql.DB, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(cfg.DBPath, 0755); err != nil {
		return nil, err
	}

	// Open database connection
	db, err := sql.Open("sqlite3", cfg.GetDatabasePath()+"?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	logger.Info("Database initialized successfully",
		zap.String("path", cfg.GetDatabasePath()),
		zap.Int("max_open_conns", cfg.MaxOpenConns),
		zap.Int("max_idle_conns", cfg.MaxIdleConns),
	)

	// Create schema
	if err := CreateTables(db, logger); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

// GenerateUUID generates a new UUID string
func GenerateUUID() string {
	return uuid.New().String()
}

// CreateTables creates core tables
func CreateTables(db *sql.DB, logger *zap.Logger) error {
	// privicy police
	offertaTable := `
	CREATE TABLE IF NOT EXISTS offerta (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		id_user BIGINT NOT NULL UNIQUE,
		role TEXT DEFAULT '', -- driver or client
		approve BOOLEAN NOT NULL DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	// just users
	justTable := `
	CREATE TABLE IF NOT EXISTS just (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		id_user BIGINT NOT NULL UNIQUE,
		userName VARCHAR(255) NOT NULL,
		dataRegistred VARCHAR(50) NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	// Users table (нужна репозиторию)
	usersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6)))),
		telegram_id INTEGER NOT NULL UNIQUE,
		telegram_username TEXT DEFAULT '',
		first_name TEXT DEFAULT '',
		last_name  TEXT DEFAULT '',
		phone_number TEXT DEFAULT '',
		language_code TEXT DEFAULT 'ru',
		is_active BOOLEAN DEFAULT TRUE,
		offerta_accepted BOOLEAN DEFAULT FALSE,
		offerta_accepted_at DATETIME NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// delivery_requests: добавлены user_id, matched_driver_id, item_photo_path
	deliveryRequestsTable := `
	CREATE TABLE IF NOT EXISTS delivery_requests (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6)))),
		user_id TEXT NULL,
		telegram_id INTEGER NOT NULL,
		driver_id TEXT NULL,             -- legacy (оставляем)
		matched_driver_id TEXT NULL,     -- актуальная колонка для матчей
		from_address TEXT NOT NULL,
		from_lat REAL NOT NULL,
		from_lon REAL NOT NULL,
		to_address TEXT NOT NULL,
		to_lat REAL NOT NULL,
		to_lon REAL NOT NULL,
		distance_km REAL DEFAULT 0.0,
		eta_min INTEGER DEFAULT 0,
		price INTEGER NOT NULL CHECK (price >= 2000),
		truck_type TEXT DEFAULT '',
		contact TEXT NOT NULL,
		time_start TEXT DEFAULT '',
		comment TEXT DEFAULT '',
		item_photo_path TEXT DEFAULT '',
		status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'matched', 'in_progress', 'completed', 'cancelled')),
		completed_at DATETIME NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (driver_id) REFERENCES drivers(id) ON DELETE SET NULL,
		FOREIGN KEY (matched_driver_id) REFERENCES drivers(id) ON DELETE SET NULL,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
	);`

	driversTable := `
	CREATE TABLE IF NOT EXISTS drivers (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6)))),
		telegram_id INTEGER NOT NULL UNIQUE,
		first_name TEXT NOT NULL,
		last_name TEXT NOT NULL,
		birthday DATE NOT NULL,
		contact_number TEXT NOT NULL,
		start_city TEXT NOT NULL,
		latitude REAL NOT NULL,
		longitude REAL NOT NULL,
		profile_photo TEXT NOT NULL,
		license_front TEXT NOT NULL,
		license_back TEXT NOT NULL,
		truck_type TEXT DEFAULT '',
		truck_number TEXT DEFAULT '',
		is_verified BOOLEAN DEFAULT FALSE,
		status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'suspended')),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		approved_at DATETIME NULL,
		approved_by TEXT NULL
	);`

	driverTripsTable := `
	CREATE TABLE IF NOT EXISTS driver_trips (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6)))),
		driver_id TEXT NOT NULL,
		telegram_id INTEGER NOT NULL,
		from_address TEXT NOT NULL DEFAULT '',
		from_lat REAL NOT NULL DEFAULT 0.0,
		from_lon REAL NOT NULL DEFAULT 0.0,
		to_address TEXT NOT NULL DEFAULT '',
		to_lat REAL NOT NULL DEFAULT 0.0,
		to_lon REAL NOT NULL DEFAULT 0.0,
		distance_km REAL DEFAULT 0.0,
		eta_min INTEGER DEFAULT 0,
		price INTEGER NOT NULL DEFAULT 2000 CHECK (price >= 2000),
		truck_type TEXT DEFAULT 'any',
		start_time TEXT NOT NULL DEFAULT '',
		departure_time DATETIME DEFAULT CURRENT_TIMESTAMP,
		comment TEXT DEFAULT '',
		truck_photo TEXT DEFAULT '',
		has_whatsapp BOOLEAN DEFAULT FALSE,
		has_telegram BOOLEAN DEFAULT FALSE,
		telegram_username TEXT DEFAULT '',
		status TEXT DEFAULT 'active' CHECK (status IN ('active', 'completed', 'cancelled')),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (driver_id) REFERENCES drivers(id) ON DELETE CASCADE
	);`

	// Create tables
	for _, sql := range []string{offertaTable, justTable, usersTable, driversTable, driverTripsTable, deliveryRequestsTable} {
		if _, err := db.Exec(sql); err != nil {
			logger.Error("Failed to create table", zap.Error(err))
			return err
		}
	}

	// Ensure columns exist for delivery_requests (safe ALTERs)
	addCols := []string{
		"ALTER TABLE delivery_requests ADD COLUMN user_id TEXT NULL;",
		"ALTER TABLE delivery_requests ADD COLUMN matched_driver_id TEXT NULL;",
		"ALTER TABLE delivery_requests ADD COLUMN item_photo_path TEXT DEFAULT '';",
		"ALTER TABLE delivery_requests ADD COLUMN completed_at DATETIME NULL;",
		"ALTER TABLE drivers ADD COLUMN truck_number TEXT DEFAULT '';",
	}
	for _, q := range addCols {
		if _, err := db.Exec(q); err != nil {
			// ignore "duplicate column name"
			logger.Debug("ALTER delivery_requests (might exist)", zap.Error(err))
		}
	}

	// Indexes
	indexes := []string{
		// users
		"CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id);",

		// delivery_requests
		"CREATE INDEX IF NOT EXISTS idx_dr_telegram_id ON delivery_requests(telegram_id);",
		"CREATE INDEX IF NOT EXISTS idx_dr_user_id ON delivery_requests(user_id);",
		"CREATE INDEX IF NOT EXISTS idx_dr_driver_id ON delivery_requests(driver_id);",
		"CREATE INDEX IF NOT EXISTS idx_dr_matched_driver_id ON delivery_requests(matched_driver_id);",
		"CREATE INDEX IF NOT EXISTS idx_dr_status ON delivery_requests(status);",
		"CREATE INDEX IF NOT EXISTS idx_dr_created_at ON delivery_requests(created_at);",
		"CREATE INDEX IF NOT EXISTS idx_dr_location ON delivery_requests(from_lat, from_lon, to_lat, to_lon);",

		// drivers
		"CREATE INDEX IF NOT EXISTS idx_drivers_telegram_id ON drivers(telegram_id);",
		"CREATE INDEX IF NOT EXISTS idx_drivers_status ON drivers(status);",
		"CREATE INDEX IF NOT EXISTS idx_drivers_created_at ON drivers(created_at);",
		"CREATE INDEX IF NOT EXISTS idx_drivers_location ON drivers(latitude, longitude);",
		"CREATE INDEX IF NOT EXISTS idx_drivers_city ON drivers(start_city);",
		"CREATE UNIQUE INDEX IF NOT EXISTS ux_drivers_truck_number ON drivers(truck_number) WHERE truck_number <> '';",

		// driver_trips
		"CREATE INDEX IF NOT EXISTS idx_dt_driver_id ON driver_trips(driver_id);",
		"CREATE INDEX IF NOT EXISTS idx_dt_telegram_id ON driver_trips(telegram_id);",
		"CREATE INDEX IF NOT EXISTS idx_dt_status ON driver_trips(status);",
		"CREATE INDEX IF NOT EXISTS idx_dt_created_at ON driver_trips(created_at);",
		"CREATE INDEX IF NOT EXISTS idx_dt_location ON driver_trips(from_lat, from_lon, to_lat, to_lon);",
		"CREATE INDEX IF NOT EXISTS idx_dt_time ON driver_trips(start_time, departure_time);",
	}
	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			logger.Warn("Failed to create index", zap.String("sql", idx), zap.Error(err))
		}
	}

	// Triggers for updated_at
	triggers := []struct {
		name string
		sql  string
	}{
		{
			name: "trigger_users_updated_at",
			sql: `
				CREATE TRIGGER IF NOT EXISTS trigger_users_updated_at 
				AFTER UPDATE ON users
				BEGIN
					UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
				END;
			`,
		},
		{
			name: "trigger_delivery_requests_updated_at",
			sql: `
				CREATE TRIGGER IF NOT EXISTS trigger_delivery_requests_updated_at 
				AFTER UPDATE ON delivery_requests
				BEGIN
					UPDATE delivery_requests SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
				END;`,
		},
		{
			name: "trigger_drivers_updated_at",
			sql: `
				CREATE TRIGGER IF NOT EXISTS trigger_drivers_updated_at 
				AFTER UPDATE ON drivers
				BEGIN
					UPDATE drivers SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
				END;`,
		},
		{
			name: "trigger_driver_trips_updated_at",
			sql: `
				CREATE TRIGGER IF NOT EXISTS trigger_driver_trips_updated_at 
				AFTER UPDATE ON driver_trips
				BEGIN
					UPDATE driver_trips SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
				END;`,
		},
	}
	for _, t := range triggers {
		if _, err := db.Exec(t.sql); err != nil {
			logger.Warn("Failed to create trigger", zap.String("trigger", t.name), zap.Error(err))
		}
	}

	logger.Info("Database schema created/verified successfully")
	return nil
}
