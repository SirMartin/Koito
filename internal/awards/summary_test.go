package awards

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gabehf/koito/internal/db/sqlite"
)

// testCfg uses small thresholds so tests seed a handful of listens.
func testCfg() Config {
	return Config{Location: time.UTC, Track: Thresholds{2, 3, 4}, Album: Thresholds{5, 6, 7}}
}

func summary(t *testing.T, s *sqlite.Sqlite, it ItemType, id int32) Summary {
	t.Helper()
	sm, err := GetSummary(context.Background(), s, s.SQL(), testCfg(), it, id)
	if err != nil {
		t.Fatal(err)
	}
	return sm
}

func insertAward(t *testing.T, s *sqlite.Sqlite, pt PeriodType, period string, it ItemType, id int32, rank int) {
	t.Helper()
	if err := s.Exec(`INSERT INTO awards VALUES (?,?,?,?,?)`, string(pt), period, string(it), id, rank); err != nil {
		t.Fatal(err)
	}
}

func eq(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSummaryYearlyMedalsSortedDescending(t *testing.T) {
	s := newStore(t)
	for year, rank := range map[string]int{"2021": 1, "2023": 1, "2022": 2, "2020": 3, "2019": 3} {
		insertAward(t, s, Year, year, Artist, 7, rank)
	}
	insertAward(t, s, Year, "2018", Artist, 8, 1) // another artist
	sm := summary(t, s, Artist, 7)
	if !eq(sm.Gold, []int{2023, 2021}) || !eq(sm.Silver, []int{2022}) || !eq(sm.Bronze, []int{2020, 2019}) {
		t.Fatalf("got %+v", sm)
	}
}

func TestSummaryWeeksAtOneCount(t *testing.T) {
	s := newStore(t)
	for _, w := range []string{"2024-W01", "2024-W02", "2024-W10"} {
		insertAward(t, s, Week, w, Track, 3, 1)
	}
	insertAward(t, s, Week, "2024-W03", Album, 3, 1) // same id, different type
	if got := summary(t, s, Track, 3).WeeksAtOne; got != 3 {
		t.Fatalf("weeks = %d, want 3", got)
	}
}

func TestSummaryTrackCertification(t *testing.T) {
	s := newStore(t)
	seedAll(t, s, plays("A", 3, t0), plays("B", 1, t0))
	if got := summary(t, s, Track, idByName(t, s, Track, "T-A")).Certification; got != "platinum" {
		t.Errorf("3 listens = %q, want platinum", got)
	}
	if got := summary(t, s, Track, idByName(t, s, Track, "T-B")).Certification; got != "" {
		t.Errorf("1 listen = %q, want none", got)
	}
}

func TestSummaryAlbumUsesAlbumThresholds(t *testing.T) {
	s := newStore(t)
	seed(t, s, plays("A", 5, t0)...)
	// 5 listens: gold for an album (5/6/7) but diamond by track thresholds (2/3/4).
	if got := summary(t, s, Album, idByName(t, s, Album, "Al-A")).Certification; got != "gold" {
		t.Fatalf("got %q, want gold", got)
	}
}

func TestSummaryArtistInheritsBestOfTrackAndAlbum(t *testing.T) {
	s := newStore(t)
	// X: one track with 3 listens → platinum by track, album 3 → none. Artist: platinum.
	seed(t, s, plays("X", 3, t0)...)
	if got := summary(t, s, Artist, idByName(t, s, Artist, "X")).Certification; got != "platinum" {
		t.Errorf("X = %q, want platinum", got)
	}
	// Y: three tracks with 2 listens each (best track: gold) on one album of 6 (platinum).
	for _, tr := range []string{"y1", "y2", "y3"} {
		for i := 0; i < 2; i++ {
			seed(t, s, ListenSpec{Artists: []string{"Y"}, Album: "Al-Y", Track: tr, At: t0.Add(time.Duration(i) * time.Minute)})
		}
	}
	if got := summary(t, s, Artist, idByName(t, s, Artist, "Y")).Certification; got != "platinum" {
		t.Errorf("Y = %q, want platinum from the album", got)
	}
}

func TestSummaryUnknownItemIsEmptyNotError(t *testing.T) {
	s := newStore(t)
	for _, it := range []ItemType{Artist, Album, Track} {
		sm := summary(t, s, it, 99999)
		if len(sm.Gold)+len(sm.Silver)+len(sm.Bronze) != 0 || sm.WeeksAtOne != 0 || sm.Certification != "" {
			t.Errorf("%s: got %+v", it, sm)
		}
	}
}

func TestSummaryJSONUsesEmptyArraysNotNull(t *testing.T) {
	s := newStore(t)
	b, err := json.Marshal(summary(t, s, Track, 1))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"gold":[],"silver":[],"bronze":[],"weeks_at_one":0,"certification":""}`
	if string(b) != want {
		t.Fatalf("got %s", b)
	}
}
