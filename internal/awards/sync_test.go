package awards

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gabehf/koito/internal/db/sqlite"
)

var nowFixed = time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

func newSyncer(t *testing.T, s *sqlite.Sqlite, now time.Time) *Syncer {
	t.Helper()
	cfg, err := LoadConfig(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	return NewSyncer(s, s.SQL(), cfg, func() time.Time { return now })
}

func mustSync(t *testing.T, sy *Syncer) {
	t.Helper()
	if err := sy.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func countRows(t *testing.T, s *sqlite.Sqlite, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.SQL().QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// storedRank returns the stored rank of the named item for a period, or 0.
func storedRank(t *testing.T, s *sqlite.Sqlite, pt PeriodType, period string, it ItemType, name string) int {
	t.Helper()
	id := idByName(t, s, it, name)
	return countRowsOr0(s, `SELECT rank FROM awards WHERE period_type=? AND period=? AND item_type=? AND item_id=?`,
		string(pt), period, string(it), id)
}

func countRowsOr0(s *sqlite.Sqlite, query string, args ...any) int {
	var n int
	if err := s.SQL().QueryRow(query, args...).Scan(&n); err != nil {
		return 0
	}
	return n
}

func TestSyncComputesClosedPeriodsOnly(t *testing.T) {
	s := newStore(t)
	seedAll(t, s,
		plays("A", 1, time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC)),
		plays("B", 1, time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)),
		plays("C", 1, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)),
	)
	mustSync(t, newSyncer(t, s, nowFixed))
	if got := storedRank(t, s, Year, "2023", Artist, "A"); got != 1 {
		t.Errorf("2023 A rank = %d", got)
	}
	if got := storedRank(t, s, Year, "2024", Artist, "B"); got != 1 {
		t.Errorf("2024 B rank = %d", got)
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM awards WHERE period_type='year' AND period='2026'`); n != 0 {
		t.Errorf("current year has %d rows, want 0", n)
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM awards_done WHERE period_type='year'`); n != 3 {
		t.Errorf("done years = %d, want 3 (2023-2025)", n)
	}
}

func TestSyncIsIncremental(t *testing.T) {
	s := newStore(t)
	seed(t, s, plays("A", 1, time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC))...)
	sy := newSyncer(t, s, nowFixed)
	mustSync(t, sy)
	if err := s.Exec(`INSERT INTO awards VALUES ('year','2023','artist',9999,1)`); err != nil {
		t.Fatal(err)
	}
	mustSync(t, sy)
	if n := countRows(t, s, `SELECT COUNT(*) FROM awards WHERE item_id=9999`); n != 1 {
		t.Fatalf("sentinel was recomputed away (count %d)", n)
	}
}

func TestSyncPicksUpNewlyClosedYear(t *testing.T) {
	s := newStore(t)
	seedAll(t, s,
		plays("A", 1, time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)),
		plays("B", 1, time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)),
	)
	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	mustSync(t, newSyncer(t, s, now))
	if n := countRows(t, s, `SELECT COUNT(*) FROM awards WHERE period_type='year' AND period='2025'`); n != 0 {
		t.Fatalf("2025 is still open but has %d rows", n)
	}
	mustSync(t, newSyncer(t, s, time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)))
	if got := storedRank(t, s, Year, "2025", Artist, "B"); got != 1 {
		t.Fatalf("2025 B rank = %d, want 1", got)
	}
}

func TestSyncRecomputesAfterLateListen(t *testing.T) {
	s := newStore(t)
	base := time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC)
	seedAll(t, s, plays("A", 2, base), plays("B", 1, base))
	sy := newSyncer(t, s, nowFixed)
	mustSync(t, sy)
	if got := storedRank(t, s, Year, "2023", Artist, "A"); got != 1 {
		t.Fatalf("A rank = %d, want 1", got)
	}
	seed(t, s, plays("B", 3, base.Add(time.Hour))...)
	mustSync(t, sy)
	if got := storedRank(t, s, Year, "2023", Artist, "B"); got != 1 {
		t.Errorf("B rank = %d, want 1", got)
	}
	if got := storedRank(t, s, Year, "2023", Artist, "A"); got != 2 {
		t.Errorf("A rank = %d, want 2", got)
	}
}

func TestSyncRebuildsAfterArtistMerge(t *testing.T) {
	s := newStore(t)
	base := time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC)
	seedAll(t, s, plays("A", 2, base), plays("B", 1, base))
	sy := newSyncer(t, s, nowFixed)
	mustSync(t, sy)
	a, b := idByName(t, s, Artist, "A"), idByName(t, s, Artist, "B")
	if err := s.MergeArtists(context.Background(), a, b, false); err != nil {
		t.Fatal(err)
	}
	mustSync(t, sy)
	if n := countRows(t, s, `SELECT COUNT(*) FROM awards WHERE item_type='artist' AND item_id=?`, a); n != 0 {
		t.Errorf("merged-away artist still has %d award rows", n)
	}
	if got := countRowsOr0(s, `SELECT rank FROM awards WHERE item_type='artist' AND item_id=? AND period='2023'`, b); got != 1 {
		t.Errorf("merged artist rank = %d, want 1", got)
	}
}

func TestSyncRemovesAwardsOfDeletedTrack(t *testing.T) {
	s := newStore(t)
	seed(t, s, plays("A", 1, time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC))...)
	sy := newSyncer(t, s, nowFixed)
	mustSync(t, sy)
	tr := idByName(t, s, Track, "T-A")
	if err := s.DeleteTrack(context.Background(), tr); err != nil {
		t.Fatal(err)
	}
	mustSync(t, sy)
	if n := countRows(t, s, `SELECT COUNT(*) FROM awards WHERE item_type='track' AND item_id=?`, tr); n != 0 {
		t.Fatalf("deleted track still has %d award rows", n)
	}
}

func TestSyncOnEmptyDatabase(t *testing.T) {
	s := newStore(t)
	mustSync(t, newSyncer(t, s, nowFixed))
	if n := countRows(t, s, `SELECT COUNT(*) FROM awards`); n != 0 {
		t.Fatalf("got %d rows", n)
	}
}

func TestSyncConcurrentCallsDoNotDuplicate(t *testing.T) {
	build := func() *sqlite.Sqlite {
		s := newStore(t)
		seedAll(t, s,
			plays("A", 2, time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC)),
			plays("B", 1, time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)),
		)
		return s
	}
	single := build()
	mustSync(t, newSyncer(t, single, nowFixed))
	want := countRows(t, single, `SELECT COUNT(*) FROM awards`)

	s := build()
	sy := newSyncer(t, s, nowFixed)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- sy.Sync(context.Background())
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := countRows(t, s, `SELECT COUNT(*) FROM awards`); got != want {
		t.Fatalf("rows = %d, want %d", got, want)
	}
}
