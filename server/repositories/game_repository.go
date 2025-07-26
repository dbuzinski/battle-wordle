package repositories

import (
	"battle-wordle/server/models"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type GameRepository struct {
	db *sql.DB
}

func NewGameRepository(dbPath string) (*GameRepository, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	repo := &GameRepository{db: db}

	if err := repo.initializeTable(); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *GameRepository) initializeTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS games (
			id TEXT PRIMARY KEY,
			solution TEXT NOT NULL,
			first_player TEXT NOT NULL,
			second_player TEXT NOT NULL,
			current_player TEXT,
			result TEXT,
			guesses TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (first_player) REFERENCES players(id),
			FOREIGN KEY (second_player) REFERENCES players(id)
		)
	`
	_, err := r.db.Exec(query)
	return err
}

func (r *GameRepository) GetByID(ctx context.Context, id string) (*models.Game, error) {
	const query = `
			SELECT id, solution, created_at, updated_at, first_player, second_player, current_player, result, guesses
			FROM games
			WHERE id = $1
	`

	row := r.db.QueryRowContext(ctx, query, id)

	var game models.Game
	var guessesJSON []byte

	err := row.Scan(
		&game.ID,
		&game.Solution,
		&game.CreatedAt,
		&game.UpdatedAt,
		&game.FirstPlayer,
		&game.SecondPlayer,
		&game.CurrentPlayer,
		&game.Result,
		&guessesJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("query game by ID failed: %w", err)
	}

	if err := json.Unmarshal(guessesJSON, &game.Guesses); err != nil {
		return nil, fmt.Errorf("failed to unmarshal guesses: %w", err)
	}

	return &game, nil
}

func (r *GameRepository) GetByPlayer(ctx context.Context, playerID string) ([]*models.Game, error) {
	const query = `
			SELECT 
					id, solution, created_at, updated_at, 
					first_player, second_player, current_player, 
					result, guesses
			FROM games
			WHERE first_player = $1 OR second_player = $1
			ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, playerID)
	if err != nil {
		return nil, fmt.Errorf("query games by player failed: %w", err)
	}
	defer rows.Close()

	var games []*models.Game

	for rows.Next() {
		var game models.Game
		var guessesJSON []byte

		err := rows.Scan(
			&game.ID,
			&game.Solution,
			&game.CreatedAt,
			&game.UpdatedAt,
			&game.FirstPlayer,
			&game.SecondPlayer,
			&game.CurrentPlayer,
			&game.Result,
			&guessesJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("scan game row failed: %w", err)
		}

		if err := json.Unmarshal(guessesJSON, &game.Guesses); err != nil {
			return nil, fmt.Errorf("unmarshal guesses failed: %w", err)
		}

		games = append(games, &game)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return games, nil
}

func (r *GameRepository) CreateGame(ctx context.Context, game *models.Game) error {
	guessesJSON, err := json.Marshal(game.Guesses)
	if err != nil {
		return fmt.Errorf("failed to marshal guesses: %w", err)
	}

	const query = `
		INSERT INTO games (
			id, solution, first_player, second_player, current_player, result, guesses, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
	`
	_, err = r.db.ExecContext(
		ctx,
		query,
		game.ID,
		game.Solution,
		game.FirstPlayer,
		game.SecondPlayer,
		game.CurrentPlayer,
		game.Result,
		guessesJSON,
		game.CreatedAt,
		game.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert game: %w", err)
	}
	return nil
}

func (r *GameRepository) UpdateGame(ctx context.Context, game *models.Game) error {
	guessesJSON, err := json.Marshal(game.Guesses)
	if err != nil {
		return fmt.Errorf("failed to marshal guesses: %w", err)
	}

	const query = `
		UPDATE games SET
			solution = $1,
			first_player = $2,
			second_player = $3,
			current_player = $4,
			result = $5,
			guesses = $6,
			updated_at = $7
		WHERE id = $8
	`
	_, err = r.db.ExecContext(
		ctx,
		query,
		game.Solution,
		game.FirstPlayer,
		game.SecondPlayer,
		game.CurrentPlayer,
		game.Result,
		guessesJSON,
		game.UpdatedAt,
		game.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update game: %w", err)
	}
	return nil
}

type GameRepositoryI interface {
	CreateGame(ctx context.Context, game *models.Game) error
	GetByID(ctx context.Context, id string) (*models.Game, error)
	GetByPlayer(ctx context.Context, playerID string) ([]*models.Game, error)
	UpdateGame(ctx context.Context, game *models.Game) error
}
