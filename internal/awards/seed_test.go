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
			id := existing(s, `SELECT id FROM artists_with_name WHERE name = ?`, name)
			if id == 0 {
				a, err := s.SaveArtist(ctx, db.SaveArtistOpts{Name: name})
				if err != nil {
					t.Fatal(err)
				}
				id = a.ID
			}
			artistIDs = append(artistIDs, id)
		}
		albumID := existing(s, `SELECT id FROM releases_with_title WHERE title = ?`, sp.Album)
		if albumID == 0 {
			al, err := s.SaveAlbum(ctx, db.SaveAlbumOpts{Title: sp.Album, ArtistIDs: artistIDs})
			if err != nil {
				t.Fatal(err)
			}
			albumID = al.ID
		}
		trackID := existing(s, `SELECT id FROM tracks_with_title WHERE title = ?`, sp.Track)
		if trackID == 0 {
			tr, err := s.SaveTrack(ctx, db.SaveTrackOpts{Title: sp.Track, AlbumID: albumID, ArtistIDs: artistIDs})
			if err != nil {
				t.Fatal(err)
			}
			trackID = tr.ID
		}
		if err := s.SaveListen(ctx, db.SaveListenOpts{TrackID: trackID, Time: sp.At, UserID: 1}); err != nil {
			t.Fatal(err)
		}
	}
}

// existing returns the id selected by query for name, or 0 if there is none.
// The save functions do not deduplicate by name, so seed looks first.
func existing(s *sqlite.Sqlite, query, name string) int32 {
	var id int32
	if err := s.SQL().QueryRow(query, name).Scan(&id); err != nil {
		return 0
	}
	return id
}
