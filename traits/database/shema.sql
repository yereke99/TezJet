-- TezJet Database Schema
-- SQLite Database Schema for Delivery Service Application

-- Enable foreign key constraints
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

-- Users table - stores client information
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_id INTEGER UNIQUE NOT NULL,
    telegram_username TEXT,
    first_name TEXT NOT NULL,
    last_name TEXT,
    phone_number TEXT,
    language_code TEXT DEFAULT 'ru',
    is_active BOOLEAN DEFAULT true,
    offerta_accepted BOOLEAN DEFAULT false,
    offerta_accepted_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Drivers table - stores driver information
CREATE TABLE IF NOT EXISTS drivers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_id INTEGER UNIQUE NOT NULL,
    telegram_username TEXT,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    birthday DATE NOT NULL,
    contact_number TEXT NOT NULL,
    start_city TEXT NOT NULL,
    latitude REAL NOT NULL,
    longitude REAL NOT NULL,
    profile_photo_path TEXT,
    license_front_path TEXT,
    license_back_path TEXT,
    truck_photo_path TEXT,
    status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'suspended')),
    is_approved BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    is_online BOOLEAN DEFAULT false,
    rating REAL DEFAULT 0.0,
    total_trips INTEGER DEFAULT 0,
    total_earnings INTEGER DEFAULT 0,
    has_whatsapp BOOLEAN DEFAULT false,
    has_telegram BOOLEAN DEFAULT true,
    approved_at DATETIME,
    last_active_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Delivery requests table - stores client delivery requests
CREATE TABLE IF NOT EXISTS delivery_requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    telegram_id INTEGER NOT NULL,
    from_address TEXT NOT NULL,
    from_lat REAL NOT NULL,
    from_lon REAL NOT NULL,
    to_address TEXT NOT NULL,
    to_lat REAL NOT NULL,
    to_lon REAL NOT NULL,
    price INTEGER NOT NULL CHECK (price >= 2000),
    contact TEXT NOT NULL,
    comment TEXT,
    truck_type TEXT NOT NULL CHECK (truck_type IN ('small', 'medium', 'large', 'refrigerator', 'tow')),
    distance_km REAL DEFAULT 0.0,
    status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'matched', 'in_progress', 'completed', 'cancelled')),
    item_photo_path TEXT,
    matched_driver_id INTEGER,
    completed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (matched_driver_id) REFERENCES drivers(id)
);

-- Driver routes table - stores driver route offerings
CREATE TABLE IF NOT EXISTS driver_routes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    driver_id INTEGER NOT NULL,
    telegram_id INTEGER NOT NULL,
    from_address TEXT NOT NULL,
    from_lat REAL NOT NULL,
    from_lon REAL NOT NULL,
    to_address TEXT NOT NULL,
    to_lat REAL NOT NULL,
    to_lon REAL NOT NULL,
    price INTEGER NOT NULL CHECK (price >= 2000),
    truck_type TEXT NOT NULL CHECK (truck_type IN ('small', 'medium', 'large', 'refrigerator', 'tow')),
    max_weight INTEGER DEFAULT 0,
    comment TEXT,
    departure_time DATETIME NOT NULL,
    arrival_time DATETIME,
    distance_km REAL DEFAULT 0.0,
    status TEXT DEFAULT 'active' CHECK (status IN ('active', 'matched', 'completed', 'cancelled')),
    available_seats INTEGER DEFAULT 1 CHECK (available_seats >= 0),
    is_active BOOLEAN DEFAULT true,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (driver_id) REFERENCES drivers(id)
);

-- Driver matches table - stores matches between drivers and delivery requests
CREATE TABLE IF NOT EXISTS driver_matches (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    driver_id INTEGER NOT NULL,
    driver_route_id INTEGER NOT NULL,
    delivery_request_id INTEGER NOT NULL,
    client_telegram_id INTEGER NOT NULL,
    status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected', 'completed')),
    proposed_price INTEGER NOT NULL,
    final_price INTEGER,
    pickup_time DATETIME,
    delivery_time DATETIME,
    driver_comment TEXT,
    client_comment TEXT,
    driver_rating INTEGER CHECK (driver_rating >= 1 AND driver_rating <= 5),
    client_rating INTEGER CHECK (client_rating >= 1 AND client_rating <= 5),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    FOREIGN KEY (driver_id) REFERENCES drivers(id),
    FOREIGN KEY (driver_route_id) REFERENCES driver_routes(id),
    FOREIGN KEY (delivery_request_id) REFERENCES delivery_requests(id)
);

-- Indexes for performance optimization

-- Users indexes
CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id);
CREATE INDEX IF NOT EXISTS idx_users_active ON users(is_active);

-- Drivers indexes
CREATE INDEX IF NOT EXISTS idx_drivers_telegram_id ON drivers(telegram_id);
CREATE INDEX IF NOT EXISTS idx_drivers_status ON drivers(status, is_approved, is_active);
CREATE INDEX IF NOT EXISTS idx_drivers_location ON drivers(latitude, longitude);
CREATE INDEX IF NOT EXISTS idx_drivers_online ON drivers(is_online, is_active);

-- Delivery requests indexes
CREATE INDEX IF NOT EXISTS idx_delivery_requests_telegram_id ON delivery_requests(telegram_id);
CREATE INDEX IF NOT EXISTS idx_delivery_requests_status ON delivery_requests(status);
CREATE INDEX IF NOT EXISTS idx_delivery_requests_location ON delivery_requests(from_lat, from_lon, to_lat, to_lon);
CREATE INDEX IF NOT EXISTS idx_delivery_requests_created ON delivery_requests(created_at);
CREATE INDEX IF NOT EXISTS idx_delivery_requests_truck_type ON delivery_requests(truck_type);

-- Driver routes indexes
CREATE INDEX IF NOT EXISTS idx_driver_routes_driver_id ON driver_routes(driver_id);
CREATE INDEX IF NOT EXISTS idx_driver_routes_telegram_id ON driver_routes(telegram_id);
CREATE INDEX IF NOT EXISTS idx_driver_routes_status ON driver_routes(status, is_active);
CREATE INDEX IF NOT EXISTS idx_driver_routes_departure ON driver_routes(departure_time);
CREATE INDEX IF NOT EXISTS idx_driver_routes_location ON driver_routes(from_lat, from_lon, to_lat, to_lon);
CREATE INDEX IF NOT EXISTS idx_driver_routes_truck_type ON driver_routes(truck_type);

-- Driver matches indexes
CREATE INDEX IF NOT EXISTS idx_driver_matches_driver_id ON driver_matches(driver_id);
CREATE INDEX IF NOT EXISTS idx_driver_matches_request_id ON driver_matches(delivery_request_id);
CREATE INDEX IF NOT EXISTS idx_driver_matches_route_id ON driver_matches(driver_route_id);
CREATE INDEX IF NOT EXISTS idx_driver_matches_status ON driver_matches(status);
CREATE INDEX IF NOT EXISTS idx_driver_matches_client ON driver_matches(client_telegram_id);

-- Triggers for automatic timestamp updates

-- Users updated_at trigger
CREATE TRIGGER IF NOT EXISTS trigger_users_updated_at 
AFTER UPDATE ON users
BEGIN
    UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Drivers updated_at trigger
CREATE TRIGGER IF NOT EXISTS trigger_drivers_updated_at 
AFTER UPDATE ON drivers
BEGIN
    UPDATE drivers SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Delivery requests updated_at trigger
CREATE TRIGGER IF NOT EXISTS trigger_delivery_requests_updated_at 
AFTER UPDATE ON delivery_requests
BEGIN
    UPDATE delivery_requests SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Driver routes updated_at trigger
CREATE TRIGGER IF NOT EXISTS trigger_driver_routes_updated_at 
AFTER UPDATE ON driver_routes
BEGIN
    UPDATE driver_routes SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Driver matches updated_at trigger
CREATE TRIGGER IF NOT EXISTS trigger_driver_matches_updated_at 
AFTER UPDATE ON driver_matches
BEGIN
    UPDATE driver_matches SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Views for common queries

-- Active drivers view
CREATE VIEW IF NOT EXISTS active_drivers AS
SELECT 
    d.*,
    COUNT(dr.id) as active_routes_count
FROM drivers d
LEFT JOIN driver_routes dr ON d.id = dr.driver_id 
    AND dr.status = 'active' 
    AND dr.is_active = true 
    AND dr.departure_time > datetime('now')
WHERE d.is_approved = true 
    AND d.is_active = true
GROUP BY d.id;

-- Available driver routes view
CREATE VIEW IF NOT EXISTS available_routes AS
SELECT 
    dr.*,
    d.first_name,
    d.last_name,
    d.rating,
    d.total_trips,
    d.profile_photo_path,
    d.contact_number,
    d.has_whatsapp,
    d.has_telegram,
    d.telegram_username
FROM driver_routes dr
JOIN drivers d ON dr.driver_id = d.id
WHERE dr.status = 'active' 
    AND dr.is_active = true 
    AND dr.departure_time > datetime('now')
    AND dr.available_seats > 0
    AND d.is_approved = true 
    AND d.is_active = true
ORDER BY dr.departure_time ASC;

-- Pending delivery requests view
CREATE VIEW IF NOT EXISTS pending_requests AS
SELECT 
    req.*,
    u.first_name as client_first_name,
    u.last_name as client_last_name,
    u.telegram_username as client_username
FROM delivery_requests req
LEFT JOIN users u ON req.user_id = u.id
WHERE req.status = 'pending'
ORDER BY req.created_at ASC;

-- Driver statistics view
CREATE VIEW IF NOT EXISTS driver_stats AS
SELECT 
    d.id,
    d.telegram_id,
    d.first_name,
    d.last_name,
    d.rating,
    d.total_trips,
    d.total_earnings,
    COUNT(dr.id) as total_routes_created,
    COUNT(CASE WHEN dr.status = 'completed' THEN 1 END) as completed_routes,
    COUNT(CASE WHEN dr.status = 'cancelled' THEN 1 END) as cancelled_routes,
    ROUND(
        CAST(COUNT(CASE WHEN dr.status = 'cancelled' THEN 1 END) AS FLOAT) / 
        NULLIF(COUNT(dr.id), 0) * 100, 2
    ) as cancellation_rate,
    d.created_at as joined_at,
    d.last_active_at
FROM drivers d
LEFT JOIN driver_routes dr ON d.id = dr.driver_id
GROUP BY d.id;

-- Distance calculation function (using Haversine formula)
-- Note: SQLite doesn't have built-in trigonometric functions, so this is a simplified version
-- For production use, consider implementing this in application code or using SQLite extensions

-- Example queries for common operations:

-- Find drivers within 10km of a pickup location (example coordinates for Almaty)
/*
SELECT d.*, dr.*,
    (6371 * acos(cos(radians(43.238949)) * cos(radians(dr.from_lat)) * 
     cos(radians(dr.from_lon) - radians(76.889709)) + sin(radians(43.238949)) * 
     sin(radians(dr.from_lat)))) AS distance_km
FROM drivers d
JOIN driver_routes dr ON d.id = dr.driver_id
WHERE d.is_approved = true 
    AND d.is_active = true 
    AND dr.status = 'active'
    AND dr.is_active = true
    AND dr.departure_time > datetime('now')
HAVING distance_km <= 10
ORDER BY distance_km ASC, d.rating DESC;
*/

-- Get delivery requests for a specific user
/*
SELECT * FROM delivery_requests 
WHERE telegram_id = ? 
ORDER BY created_at DESC 
LIMIT 20;
*/

-- Get active routes for a driver
/*
SELECT * FROM driver_routes 
WHERE telegram_id = ? 
    AND status = 'active' 
    AND is_active = true 
    AND departure_time > datetime('now')
ORDER BY departure_time ASC;
*/

-- Find matching drivers for a delivery request
/*
SELECT DISTINCT d.id, d.first_name, d.last_name, d.profile_photo_path, 
    d.contact_number, d.rating, d.total_trips, dr.from_address, dr.to_address, 
    dr.price, dr.truck_type, dr.comment, dr.departure_time,
    (6371 * acos(cos(radians(?)) * cos(radians(dr.from_lat)) * 
     cos(radians(dr.from_lon) - radians(?)) + sin(radians(?)) * 
     sin(radians(dr.from_lat)))) AS pickup_distance
FROM drivers d
JOIN driver_routes dr ON d.id = dr.driver_id
WHERE d.is_approved = true 
    AND d.is_active = true 
    AND dr.status = 'active'
    AND dr.is_active = true
    AND dr.departure_time > datetime('now')
    AND dr.truck_type = ?
    AND dr.available_seats > 0
HAVING pickup_distance <= 10
ORDER BY pickup_distance ASC, d.rating DESC
LIMIT 20;
*/

-- Get driver match history
/*
SELECT dm.*, dr.from_address, dr.to_address, 
    req.from_address as req_from, req.to_address as req_to
FROM driver_matches dm
JOIN driver_routes dr ON dm.driver_route_id = dr.id
JOIN delivery_requests req ON dm.delivery_request_id = req.id
WHERE dm.driver_id = ?
ORDER BY dm.created_at DESC;
*/

-- Get pending driver applications
/*
SELECT * FROM drivers 
WHERE status = 'pending' 
    AND is_approved = false
ORDER BY created_at ASC;
*/

-- Update driver rating after completed trip
/*
UPDATE drivers 
SET rating = (
    SELECT AVG(CAST(driver_rating AS FLOAT))
    FROM driver_matches 
    WHERE driver_id = ? 
        AND driver_rating IS NOT NULL
),
total_trips = total_trips + 1,
total_earnings = total_earnings + ?
WHERE id = ?;
*/

-- Cleanup expired routes (should be run periodically)
/*
UPDATE driver_routes 
SET status = 'cancelled', 
    is_active = false, 
    updated_at = CURRENT_TIMESTAMP
WHERE departure_time <= datetime('now', '-1 hour') 
    AND status = 'active' 
    AND is_active = true;
*/

-- Data integrity constraints and business rules:

-- 1. Drivers must be 18+ years old (enforced in application)
-- 2. Minimum price is 2000 (enforced by CHECK constraint)
-- 3. Coordinates must be valid (enforced in application)
-- 4. File paths must exist (enforced in application)
-- 5. Ratings must be between 1-5 (enforced by CHECK constraint)
-- 6. Available seats cannot be negative (enforced by CHECK constraint)
-- 7. Only approved drivers can create routes (enforced in application)
-- 8. Routes cannot be in the past (enforced in application)

-- Performance considerations:
-- 1. Use indexes on frequently queried columns
-- 2. Use prepared statements to prevent SQL injection
-- 3. Implement connection pooling
-- 4. Consider partitioning large tables by date
-- 5. Regular VACUUM and ANALYZE operations for SQLite
-- 6. Monitor query performance and add indexes as needed

-- Security considerations:
-- 1. All user inputs must be validated and sanitized
-- 2. Use parameterized queries to prevent SQL injection
-- 3. Implement proper authentication and authorization
-- 4. Encrypt sensitive data like contact numbers
-- 5. Regular backups and disaster recovery planning
-- 6. Audit trail for sensitive operations

-- Backup and maintenance:
-- 1. Regular database backups using SQLite backup API
-- 2. WAL mode for better concurrency
-- 3. Foreign key constraints enabled
-- 4. Regular PRAGMA integrity_check
-- 5. Monitor database size and performance metrics