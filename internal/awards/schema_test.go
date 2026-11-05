package awards

import (
	"context"
	"testing"
	"time"

	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/db/sqlite"
)

var t0 = time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)

func newStore(t *testing.T) *sqlite.Sqlite {
	t.Helper()
	s, err := sqlite.NewInMemory()
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(context.Background(), s.SQL()); err != nil {
		t.Fatal(err)
	}
	return s
}

func dirty(t *testing.T, s *sqlite.Sqlite) []int64 {
	t.Helper()
	rows, err := s.SQL().Query(`SELECT ts FROM awards_dirty ORDER BY ts`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var ts int64
		if err := rows.Scan(&ts); err != nil {
			t.Fatal(err)
		}
		out = append(out, ts)
	}
	return out
}

func clearDirty(t *testing.T, s *sqlite.Sqlite) {
	t.Helper()
	if err := s.Exec(`DELETE FROM awards_dirty`); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureSchemaIsIdempotent(t *testing.T) {
	s := newStore(t)
	if err := EnsureSchema(context.Background(), s.SQL()); err != nil {
		t.Fatal(err)
	}
}

func TestInsertListenMarksTimestampDirty(t *testing.T) {
	s := newStore(t)
	seed(t, s, ListenSpec{Artists: []string{"A"}, Album: "Al", Track: "T", At: t0})
	got := dirty(t, s)
	if len(got) != 1 || got[0] != t0.Unix() {
		t.Fatalf("dirty = %v, want [%d]", got, t0.Unix())
	}
}

func TestDeleteListenMarksTimestampDirty(t *testing.T) {
	s := newStore(t)
	seed(t, s, ListenSpec{Artists: []string{"A"}, Album: "Al", Track: "T", At: t0})
	clearDirty(t, s)
	var trackID int32
	if err := s.SQL().QueryRow(`SELECT track_id FROM listens`).Scan(&trackID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteListen(context.Background(), trackID, t0); err != nil {
		t.Fatal(err)
	}
	got := dirty(t, s)
	if len(got) != 1 || got[0] != t0.Unix() {
		t.Fatalf("dirty = %v, want [%d]", got, t0.Unix())
	}
}

func TestMergeTracksMarksListensDirty(t *testing.T) {
	s := newStore(t)
	seed(t, s,
		ListenSpec{Artists: []string{"A"}, Album: "Al", Track: "T1", At: t0},
		ListenSpec{Artists: []string{"A"}, Album: "Al", Track: "T2", At: t0.Add(time.Hour)},
	)
	clearDirty(t, s)
	var from, to int32
	if err := s.SQL().QueryRow(`SELECT track_id FROM listens WHERE listened_at = ?`, t0.Unix()).Scan(&from); err != nil {
		t.Fatal(err)
	}
	if err := s.SQL().QueryRow(`SELECT track_id FROM listens WHERE listened_at = ?`, t0.Add(time.Hour).Unix()).Scan(&to); err != nil {
		t.Fatal(err)
	}
	if err := s.MergeTracks(context.Background(), from, to); err != nil {
		t.Fatal(err)
	}
	got := dirty(t, s)
	if len(got) == 0 {
		t.Fatal("merging tracks left awards_dirty empty")
	}
	found := false
	for _, ts := range got {
		if ts == t0.Unix() {
			found = true
		}
	}
	if !found {
		t.Fatalf("dirty = %v, want it to contain %d", got, t0.Unix())
	}
}

func TestMergeArtistsMarksEverythingDirty(t *testing.T) {
	s := newStore(t)
	seed(t, s,
		ListenSpec{Artists: []string{"A"}, Album: "Al", Track: "T1", At: t0},
		ListenSpec{Artists: []string{"B"}, Album: "Bl", Track: "T2", At: t0.Add(time.Hour)},
	)
	clearDirty(t, s)
	var a, b int32
	s.SQL().QueryRow(`SELECT id FROM artists ORDER BY id LIMIT 1`).Scan(&a)
	s.SQL().QueryRow(`SELECT id FROM artists ORDER BY id DESC LIMIT 1`).Scan(&b)
	if err := s.MergeArtists(context.Background(), a, b, false); err != nil {
		t.Fatal(err)
	}
	got := dirty(t, s)
	if len(got) == 0 || got[0] != -1 {
		t.Fatalf("dirty = %v, want a -1 row", got)
	}
}

func TestDeleteTrackMarksDirty(t *testing.T) {
	s := newStore(t)
	seed(t, s, ListenSpec{Artists: []string{"A"}, Album: "Al", Track: "T", At: t0})
	clearDirty(t, s)
	var trackID int32
	if err := s.SQL().QueryRow(`SELECT track_id FROM listens`).Scan(&trackID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTrack(context.Background(), trackID); err != nil {
		t.Fatal(err)
	}
	if len(dirty(t, s)) == 0 {
		t.Fatal("deleting a track with listens left awards_dirty empty")
	}
}

func TestNewTrackWithoutListensDoesNotMarkDirty(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	a, err := s.SaveArtist(ctx, db.SaveArtistOpts{Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	al, err := s.SaveAlbum(ctx, db.SaveAlbumOpts{Title: "Al", ArtistIDs: []int32{a.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveTrack(ctx, db.SaveTrackOpts{Title: "T", AlbumID: al.ID, ArtistIDs: []int32{a.ID}}); err != nil {
		t.Fatal(err)
	}
	if got := dirty(t, s); len(got) != 0 {
		t.Fatalf("dirty = %v, want empty", got)
	}
}
