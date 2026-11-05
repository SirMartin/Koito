package awards

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gabehf/koito/internal/db/sqlite"
)

// plays returns n listens of a one-track, one-album artist, one minute apart from base.
func plays(artist string, n int, base time.Time) []ListenSpec {
	var out []ListenSpec
	for i := 0; i < n; i++ {
		out = append(out, ListenSpec{
			Artists: []string{artist}, Album: "Al-" + artist, Track: "T-" + artist,
			At: base.Add(time.Duration(i) * time.Minute),
		})
	}
	return out
}

func idByName(t *testing.T, s *sqlite.Sqlite, it ItemType, name string) int32 {
	t.Helper()
	q := map[ItemType]string{
		Artist: `SELECT id FROM artists_with_name WHERE name = ?`,
		Album:  `SELECT id FROM releases_with_title WHERE title = ?`,
		Track:  `SELECT id FROM tracks_with_title WHERE title = ?`,
	}[it]
	var id int32
	if err := s.SQL().QueryRow(q, name).Scan(&id); err != nil {
		t.Fatalf("lookup %s %q: %v", it, name, err)
	}
	return id
}

func rankOf(t *testing.T, s *sqlite.Sqlite, as []Award, it ItemType, name string) int {
	t.Helper()
	id := idByName(t, s, it, name)
	for _, a := range as {
		if a.ItemType == it && a.ItemID == id {
			return a.Rank
		}
	}
	return 0
}

func seedAll(t *testing.T, s *sqlite.Sqlite, groups ...[]ListenSpec) {
	t.Helper()
	for _, g := range groups {
		seed(t, s, g...)
	}
}

func year2024(t *testing.T) Period {
	return PeriodOf(Year, t0.Unix(), time.UTC)
}

func TestComputeYearReturnsTopThreeForEachType(t *testing.T) {
	s := newStore(t)
	seedAll(t, s, plays("A", 4, t0), plays("B", 3, t0), plays("C", 2, t0), plays("D", 1, t0))
	as, err := ComputePeriod(context.Background(), s, year2024(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(as) != 9 {
		t.Fatalf("got %d awards, want 9: %+v", len(as), as)
	}
	for _, it := range []ItemType{Artist, Album, Track} {
		for i, name := range []string{"A", "B", "C"} {
			prefix := map[ItemType]string{Artist: "", Album: "Al-", Track: "T-"}[it]
			if got := rankOf(t, s, as, it, prefix+name); got != i+1 {
				t.Errorf("%s %s rank = %d, want %d", it, name, got, i+1)
			}
		}
	}
}

func TestComputeYearIncludesTiedItems(t *testing.T) {
	s := newStore(t)
	seedAll(t, s, plays("A", 2, t0), plays("B", 2, t0), plays("C", 1, t0), plays("D", 1, t0))
	as, err := ComputePeriod(context.Background(), s, year2024(t))
	if err != nil {
		t.Fatal(err)
	}
	// Upstream uses RANK(): ties share a rank and the next rank skips (1,1,3,3).
	want := map[string]int{"A": 1, "B": 1, "C": 3, "D": 3}
	for name, r := range want {
		if got := rankOf(t, s, as, Artist, name); got != r {
			t.Errorf("artist %s rank = %d, want %d", name, got, r)
		}
	}
}

func TestComputeWeekReturnsOnlyRankOne(t *testing.T) {
	s := newStore(t)
	seedAll(t, s, plays("A", 3, t0), plays("B", 2, t0))
	as, err := ComputePeriod(context.Background(), s, PeriodOf(Week, t0.Unix(), time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(as) != 3 { // artist, album and track of A
		t.Fatalf("got %+v, want only A's three awards", as)
	}
	for _, a := range as {
		if a.Rank != 1 {
			t.Errorf("rank = %d", a.Rank)
		}
	}
	if rankOf(t, s, as, Artist, "B") != 0 {
		t.Error("B must not win a week award")
	}
}

func TestComputePeriodWithNoListensReturnsEmpty(t *testing.T) {
	s := newStore(t)
	as, err := ComputePeriod(context.Background(), s, year2024(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(as) != 0 {
		t.Fatalf("got %+v", as)
	}
}

func TestComputeCountsFeaturedArtists(t *testing.T) {
	s := newStore(t)
	seed(t, s, ListenSpec{Artists: []string{"Main", "Guest"}, Album: "Al", Track: "T", At: t0})
	as, err := ComputePeriod(context.Background(), s, year2024(t))
	if err != nil {
		t.Fatal(err)
	}
	if rankOf(t, s, as, Artist, "Main") != 1 || rankOf(t, s, as, Artist, "Guest") != 1 {
		t.Fatalf("both credited artists should rank 1: %+v", as)
	}
}

func TestComputeIgnoresListensOutsidePeriod(t *testing.T) {
	s := newStore(t)
	seedAll(t, s,
		plays("A", 1, t0),
		plays("B", 5, t0.AddDate(-1, 0, 0)),
		plays("B", 5, t0.AddDate(1, 0, 0)),
	)
	as, err := ComputePeriod(context.Background(), s, year2024(t))
	if err != nil {
		t.Fatal(err)
	}
	if rankOf(t, s, as, Artist, "A") != 1 {
		t.Fatalf("A should win 2024: %+v", as)
	}
	if rankOf(t, s, as, Artist, "B") != 0 {
		t.Fatal("B has no listens in 2024")
	}
}

func TestComputeWeekKeepsAllTiedItemsBeyondOnePage(t *testing.T) {
	s := newStore(t)
	// 60 tracks listened once each: all tie at rank 1, more than one page of 50.
	for i := 0; i < 60; i++ {
		seed(t, s, plays(fmt.Sprintf("Art%02d", i), 1, t0)...)
	}
	as, err := ComputePeriod(context.Background(), s, PeriodOf(Week, t0.Unix(), time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	counts := map[ItemType]int{}
	for _, a := range as {
		counts[a.ItemType]++
	}
	for _, it := range []ItemType{Artist, Album, Track} {
		if counts[it] != 60 {
			t.Errorf("%s awards = %d, want 60", it, counts[it])
		}
	}
}
