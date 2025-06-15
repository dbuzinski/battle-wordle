package database

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB represents the database connection
type DB struct {
	*sql.DB
}

// New creates a new database connection
func New(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_busy_timeout=5000&_txlock=deferred")
	if err != nil {
		return nil, err
	}

	// Set connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Create tables
	if err := createTables(db); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

// createTables creates the necessary database tables
func createTables(db *sql.DB) error {
	// Create players table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS players (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			wins INTEGER DEFAULT 0,
			losses INTEGER DEFAULT 0,
			draws INTEGER DEFAULT 0
		)
	`)
	if err != nil {
		return err
	}

	// Create games table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS games (
			id TEXT PRIMARY KEY,
			solution TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			loser_id TEXT,
			current_player TEXT,
			game_over BOOLEAN DEFAULT FALSE,
			guesses TEXT,
			rematch_game_id TEXT,
			FOREIGN KEY (loser_id) REFERENCES players(id),
			FOREIGN KEY (rematch_game_id) REFERENCES games(id)
		)
	`)
	if err != nil {
		return err
	}

	// Create game_players table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS game_players (
			game_id TEXT,
			player_id TEXT,
			PRIMARY KEY (game_id, player_id),
			FOREIGN KEY (game_id) REFERENCES games(id),
			FOREIGN KEY (player_id) REFERENCES players(id)
		)
	`)
	return err
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}
