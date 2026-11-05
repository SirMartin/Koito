package awards

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"strconv"

	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/logger"
	"github.com/gabehf/koito/internal/utils"
	"github.com/go-chi/chi/v5"
)

// sqlProvider is implemented by the SQLite store (see sqlite/awards_access.go).
type sqlProvider interface{ SQL() *sql.DB }

// Handler serves GET /apis/web/v1/awards/{type}/{id}.
func Handler(store db.DB) http.HandlerFunc {
	l := logger.FromContext(context.Background())
	cfg, err := LoadConfig(os.Getenv)
	if err != nil {
		l.Err(err).Msg("Awards: invalid configuration, falling back to defaults")
		cfg, _ = LoadConfig(func(string) string { return "" })
	}
	sp, ok := store.(sqlProvider)
	if ok {
		// Idempotent; makes the tables exist before the first request.
		if err := EnsureSchema(context.Background(), sp.SQL()); err != nil {
			l.Err(err).Msg("Awards: failed to create schema")
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var it ItemType
		switch ItemType(chi.URLParam(r, "type")) {
		case Artist:
			it = Artist
		case Album:
			it = Album
		case Track:
			it = Track
		default:
			utils.WriteError(w, "invalid item type", http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
		if err != nil {
			utils.WriteError(w, "invalid id", http.StatusBadRequest)
			return
		}
		if !ok {
			utils.WriteJSON(w, http.StatusOK, Summary{Gold: []int{}, Silver: []int{}, Bronze: []int{}})
			return
		}
		sm, err := GetSummary(r.Context(), store, sp.SQL(), cfg, it, int32(id))
		if err != nil {
			logger.FromContext(r.Context()).Err(err).Msg("Awards: failed to build summary")
			utils.WriteError(w, "internal server error", http.StatusInternalServerError)
			return
		}
		utils.WriteJSON(w, http.StatusOK, sm)
	}
}
