package awards

import (
	"context"
	"database/sql"
	"fmt"
)

// The awards tables and triggers are created here instead of in a goose
// migration: a migration number would collide with upstream's next one.
//
// awards_dirty.ts holds the listened_at of a changed listen, or -1 meaning
// "rebuild everything" (merges and deletes that can move many listens).
var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS awards (
		period_type TEXT NOT NULL,
		period      TEXT NOT NULL,
		item_type   TEXT NOT NULL,
		item_id     INTEGER NOT NULL,
		rank        INTEGER NOT NULL,
		PRIMARY KEY (period_type, period, item_type, item_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_awards_item ON awards(item_type, item_id)`,
	`CREATE TABLE IF NOT EXISTS awards_done (
		period_type TEXT NOT NULL,
		period      TEXT NOT NULL,
		PRIMARY KEY (period_type, period)
	)`,
	`CREATE TABLE IF NOT EXISTS awards_dirty (ts INTEGER NOT NULL)`,

	`CREATE TRIGGER IF NOT EXISTS awards_listens_ai AFTER INSERT ON listens
	BEGIN INSERT INTO awards_dirty (ts) VALUES (NEW.listened_at); END`,
	`CREATE TRIGGER IF NOT EXISTS awards_listens_ad AFTER DELETE ON listens
	BEGIN INSERT INTO awards_dirty (ts) VALUES (OLD.listened_at); END`,
	`CREATE TRIGGER IF NOT EXISTS awards_listens_au AFTER UPDATE OF track_id ON listens
	BEGIN INSERT INTO awards_dirty (ts) VALUES (NEW.listened_at); END`,

	`CREATE TRIGGER IF NOT EXISTS awards_artist_tracks_au AFTER UPDATE ON artist_tracks
	BEGIN INSERT INTO awards_dirty (ts) VALUES (-1); END`,
	`CREATE TRIGGER IF NOT EXISTS awards_artist_tracks_ad AFTER DELETE ON artist_tracks
	BEGIN INSERT INTO awards_dirty (ts) VALUES (-1); END`,
	// Album merges move tracks to another release.
	`CREATE TRIGGER IF NOT EXISTS awards_tracks_au AFTER UPDATE OF release_id ON tracks
	BEGIN INSERT INTO awards_dirty (ts) VALUES (-1); END`,
}

// EnsureSchema creates the awards tables and triggers. It is idempotent.
func EnsureSchema(ctx context.Context, sqlDB *sql.DB) error {
	for _, stmt := range schemaStatements {
		if _, err := sqlDB.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("awards: ensure schema: %w", err)
		}
	}
	return nil
}
