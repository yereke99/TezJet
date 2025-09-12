package database

import (
	"database/sql"
	"os"
	"tezjet/config"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// initDatabase initializes the SQLite database
func InitDatabase(cfg *config.Config, logger *zap.Logger) (*sql.DB, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(cfg.DBPath, 0755); err != nil {
		return nil, err
	}

	// Open database connection
	db, err := sql.Open("sqlite3", cfg.GetDatabasePath()+"?_foreign_keys=on&_journal_mode=WAL")
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

	return db, nil
}

// GenerateUUID generates a new UUID string
func GenerateUUID() string {
	return uuid.New().String()
}

// createTables creates delivery_requests, drivers, and driver_trips tables with UUID primary keys
func CreateTables(db *sql.DB, logger *zap.Logger) error {
	// FIXED: Updated delivery_requests table with UUID primary key AND driver_id column
	deliveryRequestsTable := `
		CREATE TABLE IF NOT EXISTS delivery_requests (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6)))),
			telegram_id INTEGER NOT NULL,
			driver_id TEXT NULL,
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
			status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'matched', 'in_progress', 'completed', 'cancelled')),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (driver_id) REFERENCES drivers(id) ON DELETE SET NULL
		);`

	// Drivers table with UUID primary key
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
			is_verified BOOLEAN DEFAULT FALSE,
			status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'suspended')),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			approved_at DATETIME NULL,
			approved_by TEXT NULL
		);`

	// Driver trips table with UUID primary key and foreign key
	driverTripsTable := `
		CREATE TABLE IF NOT EXISTS driver_trips (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6)))),
			driver_id TEXT NOT NULL,
			telegram_id BIGINT NOT NULL,
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
			start_time TEXT NOT NULL,
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

	// Table creation/verification
	tables := []struct {
		name string
		sql  string
	}{
		{"delivery_requests", deliveryRequestsTable},
		{"drivers", driversTable},
		{"driver_trips", driverTripsTable},
	}

	for _, table := range tables {
		// Check if table exists
		var tableCount int
		err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table.name).Scan(&tableCount)
		if err != nil {
			logger.Error("Failed to check table existence", zap.String("table", table.name), zap.Error(err))
			return err
		}

		if tableCount == 0 {
			// Table doesn't exist, create it
			if _, err := db.Exec(table.sql); err != nil {
				logger.Error("Failed to create table", zap.String("table", table.name), zap.Error(err))
				return err
			}
			logger.Info("Table created successfully", zap.String("table", table.name))
		} else {
			logger.Info("Table exists, checking for missing columns", zap.String("table", table.name))

			// FIXED: Add missing columns for delivery_requests including driver_id
			if table.name == "delivery_requests" {
				columnsToAdd := []struct {
					name string
					sql  string
				}{
					{"driver_id", "ALTER TABLE delivery_requests ADD COLUMN driver_id TEXT NULL;"},
					{"eta_min", "ALTER TABLE delivery_requests ADD COLUMN eta_min INTEGER DEFAULT 0;"},
					{"time_start", "ALTER TABLE delivery_requests ADD COLUMN time_start TEXT DEFAULT '';"},
					{"distance_km", "ALTER TABLE delivery_requests ADD COLUMN distance_km REAL DEFAULT 0.0;"},
					{"truck_type", "ALTER TABLE delivery_requests ADD COLUMN truck_type TEXT DEFAULT '';"},
					{"comment", "ALTER TABLE delivery_requests ADD COLUMN comment TEXT DEFAULT '';"},
				}

				for _, col := range columnsToAdd {
					if _, err := db.Exec(col.sql); err != nil {
						// Column might already exist, that's okay
						logger.Debug("Column might already exist",
							zap.String("table", table.name),
							zap.String("column", col.name),
							zap.Error(err))
					} else {
						logger.Info("Added missing column",
							zap.String("table", table.name),
							zap.String("column", col.name))
					}
				}
			}

			// Add missing columns for drivers if needed
			if table.name == "drivers" {
				columnsToAdd := []struct {
					name string
					sql  string
				}{
					{"approved_at", "ALTER TABLE drivers ADD COLUMN approved_at DATETIME NULL;"},
					{"approved_by", "ALTER TABLE drivers ADD COLUMN approved_by TEXT NULL;"},
					{"truck_type", "ALTER TABLE drivers ADD COLUMN truck_type TEXT DEFAULT '';"},
					{"is_verified", "ALTER TABLE drivers ADD COLUMN is_verified BOOLEAN DEFAULT FALSE;"},
				}

				for _, col := range columnsToAdd {
					if _, err := db.Exec(col.sql); err != nil {
						// Column might already exist, that's okay
						logger.Debug("Column might already exist",
							zap.String("table", table.name),
							zap.String("column", col.name),
							zap.Error(err))
					} else {
						logger.Info("Added missing column",
							zap.String("table", table.name),
							zap.String("column", col.name))
					}
				}
			}

			// Add missing columns for driver_trips if needed
			if table.name == "driver_trips" {
				columnsToAdd := []struct {
					name string
					sql  string
				}{
					{"departure_time", "ALTER TABLE driver_trips ADD COLUMN departure_time DATETIME DEFAULT CURRENT_TIMESTAMP;"},
					{"truck_photo", "ALTER TABLE driver_trips ADD COLUMN truck_photo TEXT DEFAULT '';"},
					{"has_whatsapp", "ALTER TABLE driver_trips ADD COLUMN has_whatsapp BOOLEAN DEFAULT FALSE;"},
					{"has_telegram", "ALTER TABLE driver_trips ADD COLUMN has_telegram BOOLEAN DEFAULT FALSE;"},
					{"telegram_username", "ALTER TABLE driver_trips ADD COLUMN telegram_username TEXT DEFAULT '';"},
					{"truck_type", "ALTER TABLE driver_trips ADD COLUMN truck_type TEXT DEFAULT '';"},
				}

				for _, col := range columnsToAdd {
					if _, err := db.Exec(col.sql); err != nil {
						// Column might already exist, that's okay
						logger.Debug("Column might already exist",
							zap.String("table", table.name),
							zap.String("column", col.name),
							zap.Error(err))
					} else {
						logger.Info("Added missing column",
							zap.String("table", table.name),
							zap.String("column", col.name))
					}
				}
			}
		}
	}

	// Create essential indexes for delivery_requests
	deliveryIndexes := []struct {
		name string
		sql  string
	}{
		{
			name: "idx_delivery_requests_telegram_id",
			sql:  "CREATE INDEX IF NOT EXISTS idx_delivery_requests_telegram_id ON delivery_requests(telegram_id);",
		},
		{
			name: "idx_delivery_requests_driver_id",
			sql:  "CREATE INDEX IF NOT EXISTS idx_delivery_requests_driver_id ON delivery_requests(driver_id);",
		},
		{
			name: "idx_delivery_requests_status",
			sql:  "CREATE INDEX IF NOT EXISTS idx_delivery_requests_status ON delivery_requests(status);",
		},
		{
			name: "idx_delivery_requests_created_at",
			sql:  "CREATE INDEX IF NOT EXISTS idx_delivery_requests_created_at ON delivery_requests(created_at);",
		},
		{
			name: "idx_delivery_requests_location",
			sql:  "CREATE INDEX IF NOT EXISTS idx_delivery_requests_location ON delivery_requests(from_lat, from_lon, to_lat, to_lon);",
		},
	}

	// Create essential indexes for drivers
	driverIndexes := []struct {
		name string
		sql  string
	}{
		{
			name: "idx_drivers_telegram_id",
			sql:  "CREATE INDEX IF NOT EXISTS idx_drivers_telegram_id ON drivers(telegram_id);",
		},
		{
			name: "idx_drivers_status",
			sql:  "CREATE INDEX IF NOT EXISTS idx_drivers_status ON drivers(status);",
		},
		{
			name: "idx_drivers_created_at",
			sql:  "CREATE INDEX IF NOT EXISTS idx_drivers_created_at ON drivers(created_at);",
		},
		{
			name: "idx_drivers_location",
			sql:  "CREATE INDEX IF NOT EXISTS idx_drivers_location ON drivers(latitude, longitude);",
		},
		{
			name: "idx_drivers_city",
			sql:  "CREATE INDEX IF NOT EXISTS idx_drivers_city ON drivers(start_city);",
		},
	}

	// Create indexes for driver_trips
	tripIndexes := []struct {
		name string
		sql  string
	}{
		{
			name: "idx_driver_trips_driver_id",
			sql:  "CREATE INDEX IF NOT EXISTS idx_driver_trips_driver_id ON driver_trips(driver_id);",
		},
		{
			name: "idx_driver_trips_telegram_id",
			sql:  "CREATE INDEX IF NOT EXISTS idx_driver_trips_telegram_id ON driver_trips(telegram_id);",
		},
		{
			name: "idx_driver_trips_status",
			sql:  "CREATE INDEX IF NOT EXISTS idx_driver_trips_status ON driver_trips(status);",
		},
		{
			name: "idx_driver_trips_created_at",
			sql:  "CREATE INDEX IF NOT EXISTS idx_driver_trips_created_at ON driver_trips(created_at);",
		},
		{
			name: "idx_driver_trips_location",
			sql:  "CREATE INDEX IF NOT EXISTS idx_driver_trips_location ON driver_trips(from_lat, from_lon, to_lat, to_lon);",
		},
		{
			name: "idx_driver_trips_time",
			sql:  "CREATE INDEX IF NOT EXISTS idx_driver_trips_time ON driver_trips(start_time, departure_time);",
		},
	}

	// Create indexes for all tables
	allIndexes := append(deliveryIndexes, append(driverIndexes, tripIndexes...)...)
	for _, index := range allIndexes {
		if _, err := db.Exec(index.sql); err != nil {
			logger.Warn("Failed to create index",
				zap.String("index", index.name),
				zap.Error(err),
			)
		} else {
			logger.Info("Index created/verified", zap.String("index", index.name))
		}
	}

	// Create triggers for updating timestamps
	triggers := []struct {
		name string
		sql  string
	}{
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

	for _, trigger := range triggers {
		if _, err := db.Exec(trigger.sql); err != nil {
			logger.Warn("Failed to create trigger",
				zap.String("trigger", trigger.name),
				zap.Error(err))
		} else {
			logger.Info("Trigger created/verified", zap.String("trigger", trigger.name))
		}
	}

	logger.Info("Database schema created successfully with UUID primary keys and driver_id column")
	return nil
}
