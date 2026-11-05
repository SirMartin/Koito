package awards

import (
	"context"
	"os"
	"time"

	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/logger"
)

const syncInterval = 10 * time.Minute

// Start runs Sync once immediately and then every syncInterval until ctx is
// done. Errors are logged, never fatal. Stores without a SQL() handle (a
// future non-SQLite backend) are skipped so the server still boots.
func Start(ctx context.Context, store db.DB) {
	l := logger.FromContext(ctx)
	sp, ok := store.(sqlProvider)
	if !ok {
		l.Warn().Msg("Awards: store does not expose a SQL handle, awards disabled")
		return
	}
	cfg, err := LoadConfig(os.Getenv)
	if err != nil {
		l.Err(err).Msg("Awards: invalid configuration, awards disabled")
		return
	}
	if err := EnsureSchema(ctx, sp.SQL()); err != nil {
		l.Err(err).Msg("Awards: failed to create schema, awards disabled")
		return
	}

	syncer := NewSyncer(store, sp.SQL(), cfg, time.Now)
	run := func() {
		if err := syncer.Sync(ctx); err != nil && ctx.Err() == nil {
			l.Err(err).Msg("Awards: sync failed")
		}
	}
	go run()

	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
