package repositories

import (
	"battle-wordle/server/models"
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type PlayerRepository struct {
	db *sql.DB
}

func NewPlayerRepository(dbPath string) (*PlayerRepository, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	repo := &PlayerRepository{db: db}

	if err := repo.initializeTable(); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *PlayerRepository) initializeTable() error {
	query := `
        CREATE TABLE IF NOT EXISTS players (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL UNIQUE,
				registered BOOLEAN NOT NULL,
				password_hash TEXT,
				elo INTEGER,
				created_at TIMESTAMP NOT NULL
        );
    `
	_, err := r.db.Exec(query)
	return err
}

func (r *PlayerRepository) GetByID(ctx context.Context, id string) (*models.Player, error) {
	const query = `
        SELECT id, name, registered, password_hash, elo, created_at
        FROM players
        WHERE id = $1
    `

	row := r.db.QueryRowContext(ctx, query, id)

	var player models.Player
	var passwordHash sql.NullString
	var elo sql.NullInt64
	var createdAt time.Time

	err := row.Scan(&player.ID, &player.Name, &player.Registered, &passwordHash, &elo, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("query player by ID failed: %w", err)
	}
	if passwordHash.Valid {
		player.PasswordHash = &passwordHash.String
	}
	if elo.Valid {
		eloInt := int(elo.Int64)
		player.Elo = &eloInt
	}
	player.CreatedAt = createdAt

	return &player, nil
}

func (r *PlayerRepository) CreatePlayer(ctx context.Context, player *models.Player) error {
	const query = `
		INSERT INTO players (id, name, registered, password_hash, elo, created_at) VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, query, player.ID, player.Name, player.Registered, player.PasswordHash, player.Elo, player.CreatedAt)
	if err != nil {
		if sqliteErr, ok := err.(interface{ Error() string }); ok && ( // fallback for sqlite3 driver
		// Check for unique constraint violation (code 2067 for sqlite3)
		// See: https://www.sqlite.org/rescode.html#constraint_unique
		// Unfortunately, mattn/go-sqlite3 does not export error codes, so we check the error string
		// This is brittle but works for now
		// Example error: "UNIQUE constraint failed: players.name"
		len(sqliteErr.Error()) > 0 &&
			(sqliteErr.Error() == "UNIQUE constraint failed: players.name" ||
				(sqliteErr.Error() != "" && ( // fallback for partial match
				len(sqliteErr.Error()) > 0 &&
					(sqliteErr.Error()[:27] == "UNIQUE constraint failed: " &&
						len(sqliteErr.Error()) > 27 &&
						sqliteErr.Error()[27:] == "players.name"))))) {
			return fmt.Errorf("username_taken")
		}
		return fmt.Errorf("failed to insert player: %w", err)
	}
	return nil
}

func (r *PlayerRepository) GetByName(ctx context.Context, name string) (*models.Player, error) {
	const query = `
        SELECT id, name, registered, password_hash, elo, created_at
        FROM players
        WHERE name = $1
    `

	row := r.db.QueryRowContext(ctx, query, name)

	var player models.Player
	var passwordHash sql.NullString
	var elo sql.NullInt64
	var createdAt time.Time

	err := row.Scan(&player.ID, &player.Name, &player.Registered, &passwordHash, &elo, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("query player by name failed: %w", err)
	}
	if passwordHash.Valid {
		player.PasswordHash = &passwordHash.String
	}
	if elo.Valid {
		eloInt := int(elo.Int64)
		player.Elo = &eloInt
	}
	player.CreatedAt = createdAt

	return &player, nil
}

// UpdateGuestToRegistered updates a guest player to a registered player with a new name and password hash.
func (r *PlayerRepository) UpdateGuestToRegistered(ctx context.Context, id string, newName string, passwordHash string) error {
	// Check if the new name is already taken by another player
	const checkNameQuery = `SELECT id FROM players WHERE name = $1 AND id != $2`
	row := r.db.QueryRowContext(ctx, checkNameQuery, newName, id)
	var existingID string
	err := row.Scan(&existingID)
	if err == nil {
		return fmt.Errorf("username_taken")
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("failed to check name: %w", err)
	}

	// Update the guest player to registered
	const updateQuery = `
		UPDATE players
		SET name = $1, registered = 1, password_hash = $2
		WHERE id = $3 AND registered = 0
	`
	res, err := r.db.ExecContext(ctx, updateQuery, newName, passwordHash, id)
	if err != nil {
		return fmt.Errorf("failed to update guest: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("guest_not_found_or_already_registered")
	}
	return nil
}

// SearchByName returns players whose names contain the substring (case-insensitive)
func (r *PlayerRepository) SearchByName(ctx context.Context, name string) ([]*models.Player, error) {
	pattern := "%" + name + "%"
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, registered, elo, created_at FROM players WHERE lower(name) LIKE lower(?)`, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var players []*models.Player
	for rows.Next() {
		var p models.Player
		var elo *int
		var createdAt time.Time
		if err := rows.Scan(&p.ID, &p.Name, &p.Registered, &elo, &createdAt); err != nil {
			return nil, err
		}
		p.Elo = elo
		p.CreatedAt = createdAt
		players = append(players, &p)
	}
	return players, nil
}
