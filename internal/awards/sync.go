package awards

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gabehf/koito/internal/db"
)

// Syncer keeps the awards table in step with the listens. Closed periods are
// immutable, so each is computed once; triggers mark the ones that change.
type Syncer struct {
	store db.DB
	sqlDB *sql.DB
	cfg   Config
	now   func() time.Time
	mu    sync.Mutex
}

func NewSyncer(store db.DB, sqlDB *sql.DB, cfg Config, now func() time.Time) *Syncer {
	return &Syncer{store: store, sqlDB: sqlDB, cfg: cfg, now: now}
}

// Sync invalidates the periods touched since the last run, then computes every
// closed period that is not stored yet. Concurrent calls are serialised.
func (s *Syncer) Sync(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	maxDirty, err := s.invalidate(ctx)
	if err != nil {
		return err
	}
	if err := s.computeMissing(ctx); err != nil {
		return err
	}
	// Rows added while computing have a higher rowid and wait for the next run.
	if _, err := s.sqlDB.ExecContext(ctx, `DELETE FROM awards_dirty WHERE rowid <= ?`, maxDirty); err != nil {
		return fmt.Errorf("awards: clear dirty: %w", err)
	}
	return nil
}

// invalidate drops stored results for dirty periods and returns the highest
// awards_dirty rowid it accounted for.
func (s *Syncer) invalidate(ctx context.Context) (int64, error) {
	rows, err := s.sqlDB.QueryContext(ctx, `SELECT rowid, ts FROM awards_dirty`)
	if err != nil {
		return 0, fmt.Errorf("awards: read dirty: %w", err)
	}
	var maxRowid int64
	all := false
	keys := map[[2]string]struct{}{}
	for rows.Next() {
		var rowid, ts int64
		if err := rows.Scan(&rowid, &ts); err != nil {
			rows.Close()
			return 0, err
		}
		maxRowid = max(maxRowid, rowid)
		if ts < 0 {
			all = true
			continue
		}
		for _, pt := range []PeriodType{Year, Week} {
			keys[[2]string{string(pt), PeriodOf(pt, ts, s.cfg.Location).Key}] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	if maxRowid == 0 {
		return 0, nil
	}

	tx, err := s.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if all {
		for _, q := range []string{`DELETE FROM awards`, `DELETE FROM awards_done`} {
			if _, err := tx.ExecContext(ctx, q); err != nil {
				return 0, fmt.Errorf("awards: rebuild all: %w", err)
			}
		}
	} else {
		for k := range keys {
			for _, q := range []string{
				`DELETE FROM awards WHERE period_type = ? AND period = ?`,
				`DELETE FROM awards_done WHERE period_type = ? AND period = ?`,
			} {
				if _, err := tx.ExecContext(ctx, q, k[0], k[1]); err != nil {
					return 0, fmt.Errorf("awards: invalidate %s: %w", k[1], err)
				}
			}
		}
	}
	return maxRowid, tx.Commit()
}

func (s *Syncer) computeMissing(ctx context.Context) error {
	first, err := s.store.GetFirstListenUnix(ctx)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && first == 0) {
		return nil // empty database
	}
	if err != nil {
		return err
	}

	done, err := s.doneSet(ctx)
	if err != nil {
		return err
	}
	for _, pt := range []PeriodType{Year, Week} {
		for _, p := range ClosedPeriods(pt, first, s.now(), s.cfg.Location) {
			if _, ok := done[[2]string{string(pt), p.Key}]; ok {
				continue
			}
			if err := s.computeOne(ctx, p); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Syncer) doneSet(ctx context.Context) (map[[2]string]struct{}, error) {
	rows, err := s.sqlDB.QueryContext(ctx, `SELECT period_type, period FROM awards_done`)
	if err != nil {
		return nil, fmt.Errorf("awards: read done: %w", err)
	}
	defer rows.Close()
	done := map[[2]string]struct{}{}
	for rows.Next() {
		var k [2]string
		if err := rows.Scan(&k[0], &k[1]); err != nil {
			return nil, err
		}
		done[k] = struct{}{}
	}
	return done, rows.Err()
}

func (s *Syncer) computeOne(ctx context.Context, p Period) error {
	awards, err := ComputePeriod(ctx, s.store, p)
	if err != nil {
		return err
	}
	tx, err := s.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, a := range awards {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO awards (period_type, period, item_type, item_id, rank) VALUES (?,?,?,?,?)`,
			string(p.Type), p.Key, string(a.ItemType), a.ItemID, a.Rank); err != nil {
			return fmt.Errorf("awards: store %s: %w", p.Key, err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO awards_done (period_type, period) VALUES (?,?)`, string(p.Type), p.Key); err != nil {
		return err
	}
	return tx.Commit()
}
