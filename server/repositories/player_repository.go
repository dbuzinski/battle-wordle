package repositories

import (
	"battle-wordle/server/models"
	"context"
	"database/sql"
	"fmt"

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
					name TEXT NOT NULL
        );
    `
	_, err := r.db.Exec(query)
	return err
}

func (r *PlayerRepository) GetByID(ctx context.Context, id string) (*models.Player, error) {
	const query = `
        SELECT id, name
        FROM players
        WHERE id = $1
    `

	row := r.db.QueryRowContext(ctx, query, id)

	var player models.Player
	err := row.Scan(&player.ID, &player.Name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("query player by ID failed: %w", err)
	}

	return &player, nil
}

func (r *PlayerRepository) CreatePlayer(ctx context.Context, player *models.Player) error {
	const query = `
		INSERT INTO players (id, name) VALUES ($1, $2)
	`
	_, err := r.db.ExecContext(ctx, query, player.ID, player.Name)
	if err != nil {
		return fmt.Errorf("failed to insert player: %w", err)
	}
	return nil
}
