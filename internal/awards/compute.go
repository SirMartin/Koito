package awards

import (
	"context"
	"fmt"

	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/models"
)

type ItemType string

const (
	Artist ItemType = "artist"
	Album  ItemType = "album"
	Track  ItemType = "track"
)

type Award struct {
	ItemType ItemType
	ItemID   int32
	Rank     int
}

// pageSize is how many ranked items are fetched per request. Ranks use RANK(),
// so ties share a rank and can spill over several pages; topRanked keeps
// paging until it passes maxRank.
const pageSize = 50

// topRanked walks a paginated top query, visiting every item with rank <= maxRank.
func topRanked[T any](
	fetch func(db.GetItemsOpts) (*db.PaginatedResponse[db.RankedItem[T]], error),
	opts db.GetItemsOpts, maxRank int64, visit func(T, int64),
) error {
	for page := 1; ; page++ {
		opts.Page = page
		resp, err := fetch(opts)
		if err != nil {
			return err
		}
		for _, r := range resp.Items {
			if r.Rank <= maxRank {
				visit(r.Item, r.Rank)
			}
		}
		if len(resp.Items) < opts.Limit || resp.Items[len(resp.Items)-1].Rank > maxRank {
			return nil
		}
	}
}

// ComputePeriod ranks artists, albums and tracks within p using upstream's own
// top queries. Years keep ranks 1-3, weeks keep rank 1.
func ComputePeriod(ctx context.Context, store db.DB, p Period) ([]Award, error) {
	maxRank := int64(1)
	if p.Type == Year {
		maxRank = 3
	}
	opts := db.GetItemsOpts{
		Limit:     pageSize,
		Timeframe: db.Timeframe{From: p.From, To: p.To, Timezone: p.From.Location()},
	}

	var out []Award
	add := func(it ItemType, id int32, rank int64) {
		out = append(out, Award{ItemType: it, ItemID: id, Rank: int(rank)})
	}

	err := topRanked(func(o db.GetItemsOpts) (*db.PaginatedResponse[db.RankedItem[*models.Artist]], error) {
		return store.GetTopArtistsPaginated(ctx, o)
	}, opts, maxRank, func(a *models.Artist, r int64) { add(Artist, a.ID, r) })
	if err != nil {
		return nil, fmt.Errorf("awards: top artists for %s: %w", p.Key, err)
	}
	err = topRanked(func(o db.GetItemsOpts) (*db.PaginatedResponse[db.RankedItem[*models.Album]], error) {
		return store.GetTopAlbumsPaginated(ctx, o)
	}, opts, maxRank, func(a *models.Album, r int64) { add(Album, a.ID, r) })
	if err != nil {
		return nil, fmt.Errorf("awards: top albums for %s: %w", p.Key, err)
	}
	err = topRanked(func(o db.GetItemsOpts) (*db.PaginatedResponse[db.RankedItem[*models.Track]], error) {
		return store.GetTopTracksPaginated(ctx, o)
	}, opts, maxRank, func(tr *models.Track, r int64) { add(Track, tr.ID, r) })
	if err != nil {
		return nil, fmt.Errorf("awards: top tracks for %s: %w", p.Key, err)
	}
	return out, nil
}
