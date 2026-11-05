package awards

import (
	"context"
	"fmt"

	"github.com/gabehf/koito/internal/db"
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

// tieMargin is how many ranked items are fetched per type. Ranks use RANK(),
// so ties share a rank; more than this many items tied at rank 1 in a single
// period is not a realistic case.
const tieMargin = 50

// ComputePeriod ranks artists, albums and tracks within p using upstream's own
// top queries. Years keep ranks 1-3, weeks keep rank 1.
func ComputePeriod(ctx context.Context, store db.DB, p Period) ([]Award, error) {
	maxRank := int64(1)
	if p.Type == Year {
		maxRank = 3
	}
	opts := db.GetItemsOpts{
		Page:      1,
		Limit:     tieMargin,
		Timeframe: db.Timeframe{From: p.From, To: p.To, Timezone: p.From.Location()},
	}

	var out []Award
	add := func(it ItemType, id int32, rank int64) {
		if rank <= maxRank {
			out = append(out, Award{ItemType: it, ItemID: id, Rank: int(rank)})
		}
	}

	artists, err := store.GetTopArtistsPaginated(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("awards: top artists for %s: %w", p.Key, err)
	}
	for _, r := range artists.Items {
		add(Artist, r.Item.ID, r.Rank)
	}
	albums, err := store.GetTopAlbumsPaginated(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("awards: top albums for %s: %w", p.Key, err)
	}
	for _, r := range albums.Items {
		add(Album, r.Item.ID, r.Rank)
	}
	tracks, err := store.GetTopTracksPaginated(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("awards: top tracks for %s: %w", p.Key, err)
	}
	for _, r := range tracks.Items {
		add(Track, r.Item.ID, r.Rank)
	}
	return out, nil
}
