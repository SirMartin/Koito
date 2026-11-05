package awards

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/gabehf/koito/internal/db"
)

// Summary is everything the UI shows for one item.
type Summary struct {
	Gold          []int  `json:"gold"`
	Silver        []int  `json:"silver"`
	Bronze        []int  `json:"bronze"`
	WeeksAtOne    int    `json:"weeks_at_one"`
	Certification string `json:"certification"`
}

var allTime = db.Timeframe{Period: db.PeriodAllTime}

// GetSummary reads stored medals and weeks for an item and derives its
// certification from all-time listen counts. Unknown items give an empty summary.
func GetSummary(ctx context.Context, store db.DB, sqlDB *sql.DB, cfg Config, t ItemType, id int32) (Summary, error) {
	sm := Summary{Gold: []int{}, Silver: []int{}, Bronze: []int{}}

	rows, err := sqlDB.QueryContext(ctx,
		`SELECT period_type, period, rank FROM awards WHERE item_type = ? AND item_id = ?`, string(t), id)
	if err != nil {
		return sm, fmt.Errorf("awards: read awards: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var pt, period string
		var rank int
		if err := rows.Scan(&pt, &period, &rank); err != nil {
			return sm, err
		}
		if PeriodType(pt) == Week {
			if rank == 1 {
				sm.WeeksAtOne++
			}
			continue
		}
		year, err := strconv.Atoi(period)
		if err != nil {
			continue
		}
		switch rank {
		case 1:
			sm.Gold = append(sm.Gold, year)
		case 2:
			sm.Silver = append(sm.Silver, year)
		case 3:
			sm.Bronze = append(sm.Bronze, year)
		}
	}
	if err := rows.Err(); err != nil {
		return sm, err
	}
	rows.Close()
	for _, ys := range [][]int{sm.Gold, sm.Silver, sm.Bronze} {
		sort.Sort(sort.Reverse(sort.IntSlice(ys)))
	}

	cert, err := certification(ctx, store, cfg, t, id)
	if err != nil {
		return sm, err
	}
	sm.Certification = cert
	return sm, nil
}

var certOrder = map[string]int{"": 0, "gold": 1, "platinum": 2, "diamond": 3}

func certification(ctx context.Context, store db.DB, cfg Config, t ItemType, id int32) (string, error) {
	switch t {
	case Track:
		tr, err := store.GetTrack(ctx, db.GetTrackOpts{ID: id})
		if errors.Is(err, db.ErrNotFound) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		return cfg.Track.Certification(tr.ListenCount), nil
	case Album:
		al, err := store.GetAlbum(ctx, db.GetAlbumOpts{ID: id})
		if errors.Is(err, db.ErrNotFound) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		return cfg.Album.Certification(al.ListenCount), nil
	default:
		// An artist is as certified as its best track or best album.
		opts := db.GetItemsOpts{ArtistID: int(id), Limit: 1, Page: 1, Timeframe: allTime}
		best := ""
		tracks, err := store.GetTopTracksPaginated(ctx, opts)
		if err != nil {
			return "", err
		}
		if len(tracks.Items) > 0 {
			best = cfg.Track.Certification(tracks.Items[0].Item.ListenCount)
		}
		albums, err := store.GetTopAlbumsPaginated(ctx, opts)
		if err != nil {
			return "", err
		}
		if len(albums.Items) > 0 {
			if c := cfg.Album.Certification(albums.Items[0].Item.ListenCount); certOrder[c] > certOrder[best] {
				best = c
			}
		}
		return best, nil
	}
}
