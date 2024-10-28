package db

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var (
	db   *sql.DB
	once sync.Once
)

// Player represents a player in the database
type Player struct {
	ID            string
	Nickname      string
	PurePoints    float64
	EvilPoints    float64
	LastRequestID string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Initialize sets up the database connection
func Initialize(dbPath string) error {
	var err error
	once.Do(func() {
		// Check if database file exists
		_, err = os.Stat(dbPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("Database file does not exist at %s, creating new database", dbPath)
				// Ensure the directory exists
				if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
					return
				}
			} else {
				return
			}
		} else {
			log.Printf("Using existing database at %s", dbPath)
		}

		// Open database connection
		db, err = sql.Open("sqlite3", dbPath)
		if err != nil {
			return
		}

		// Test the connection
		if err = db.Ping(); err != nil {
			return
		}

		// Enable WAL mode for better concurrency
		if _, err = db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return
		}

		// Create tables if they don't exist
		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS players (
				id TEXT PRIMARY KEY,
				nickname TEXT NOT NULL,
				pure_points REAL DEFAULT 0,
				evil_points REAL DEFAULT 0,
				last_request_id TEXT,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`)
	})
	return err
}

// CreatePlayer inserts a new player into the database
func CreatePlayer(id, nickname string) error {
	_, err := db.Exec(`
		INSERT INTO players (id, nickname)
		VALUES (?, ?)
	`, id, nickname)
	return err
}

// GetPlayer retrieves a player from the database
func GetPlayer(id string) (*Player, error) {
	var p Player
	var lastRequestID sql.NullString
	err := db.QueryRow(`
		SELECT id, nickname, pure_points, evil_points, last_request_id, created_at, updated_at
		FROM players
		WHERE id = ?
	`, id).Scan(&p.ID, &p.Nickname, &p.PurePoints, &p.EvilPoints, &lastRequestID, &p.CreatedAt, &p.UpdatedAt)
	if err == nil {
		if lastRequestID.Valid {
			p.LastRequestID = lastRequestID.String
		} else {
			p.LastRequestID = ""
		}
	}
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

// UpdatePlayerPoints updates a player's points
func UpdatePlayerPoints(id string, pureDelta, evilDelta float64) error {
	_, err := db.Exec(`
		UPDATE players
		SET pure_points = pure_points + ?,
			evil_points = evil_points + ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, pureDelta, evilDelta, id)
	return err
}

// UpdatePlayerRequest updates a player's last assigned request
func UpdatePlayerRequest(id, requestID string) error {
	_, err := db.Exec(`
		UPDATE players
		SET last_request_id = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, requestID, id)
	return err
}

// GetLeaderboard returns all players sorted by net alignment
func GetLeaderboard() ([]Player, error) {
	rows, err := db.Query(`
		SELECT id, nickname, pure_points, evil_points, last_request_id, created_at, updated_at
		FROM players
		ORDER BY (pure_points - evil_points) DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []Player
	for rows.Next() {
		var p Player
		var lastRequestID sql.NullString
		err := rows.Scan(&p.ID, &p.Nickname, &p.PurePoints, &p.EvilPoints, &lastRequestID, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		// Convert NULL to empty string if needed
		if lastRequestID.Valid {
			p.LastRequestID = lastRequestID.String
		} else {
			p.LastRequestID = ""
		}
		if err != nil {
			return nil, err
		}
		players = append(players, p)
	}
	return players, rows.Err()
}

// Close closes the database connection
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
