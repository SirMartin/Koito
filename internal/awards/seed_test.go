package awards

import (
	"context"
	"testing"
	"time"

	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/db/sqlite"
)

// ListenSpec describes one listen to seed; missing artists/albums/tracks are created.
type ListenSpec struct {
	Artists      []string
	Album, Track string
	At           time.Time
}

// seed creates the listens in spec, reusing artists, albums and tracks by name.
func seed(t *testing.T, s *sqlite.Sqlite, spec ...ListenSpec) {
	t.Helper()
	ctx := context.Background()
	// listens.user_id has a foreign key and the fresh schema has no users.
	if err := s.Exec(`INSERT OR IGNORE INTO users (id, username, role, password) VALUES (1, 'u', 'user', x'00')`); err != nil {
		t.Fatal(err)
	}
	for _, sp := range spec {
		var artistIDs []int32
		for _, name := range sp.Artists {
			a, err := s.SaveArtist(ctx, db.SaveArtistOpts{Name: name})
			if err != nil {
				t.Fatal(err)
			}
			artistIDs = append(artistIDs, a.ID)
		}
		al, err := s.SaveAlbum(ctx, db.SaveAlbumOpts{Title: sp.Album, ArtistIDs: artistIDs})
		if err != nil {
			t.Fatal(err)
		}
		tr, err := s.SaveTrack(ctx, db.SaveTrackOpts{Title: sp.Track, AlbumID: al.ID, ArtistIDs: artistIDs})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SaveListen(ctx, db.SaveListenOpts{TrackID: tr.ID, Time: sp.At, UserID: 1}); err != nil {
			t.Fatal(err)
		}
	}
}
