package awards

import (
	"testing"
	"time"
)

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestPeriodOfYearKey(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC).Unix()
	p := PeriodOf(Year, ts, time.UTC)
	if p.Key != "2024" || p.Type != Year {
		t.Fatalf("got %+v", p)
	}
	if !p.From.Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) ||
		!p.To.Equal(time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)) {
		t.Fatalf("bounds = %v .. %v", p.From, p.To)
	}
}

func TestPeriodOfWeekKeyISO(t *testing.T) {
	cases := map[string]time.Time{
		"2025-W01": time.Date(2024, 12, 30, 12, 0, 0, 0, time.UTC),
		"2020-W53": time.Date(2021, 1, 3, 12, 0, 0, 0, time.UTC),
		"2024-W05": time.Date(2024, 1, 31, 12, 0, 0, 0, time.UTC),
	}
	for want, at := range cases {
		if got := PeriodOf(Week, at.Unix(), time.UTC).Key; got != want {
			t.Errorf("%v: key = %s, want %s", at, got, want)
		}
	}
}

func TestPeriodOfRespectsTimezone(t *testing.T) {
	madrid := mustLoc(t, "Europe/Madrid")
	ts := time.Date(2024, 12, 31, 23, 30, 0, 0, time.UTC).Unix() // 00:30 on 2025-01-01 in Madrid
	if got := PeriodOf(Year, ts, madrid).Key; got != "2025" {
		t.Errorf("year = %s, want 2025", got)
	}
	if got := PeriodOf(Week, ts, madrid).Key; got != "2025-W01" {
		t.Errorf("week = %s, want 2025-W01", got)
	}
	if got := PeriodOf(Year, ts, time.UTC).Key; got != "2024" {
		t.Errorf("UTC year = %s, want 2024", got)
	}
}

func TestWeekBoundsAreMondayToSunday(t *testing.T) {
	ts := time.Date(2024, 5, 8, 15, 0, 0, 0, time.UTC).Unix() // a Wednesday
	p := PeriodOf(Week, ts, time.UTC)
	if p.From.Weekday() != time.Monday || p.From.Hour() != 0 {
		t.Errorf("from = %v", p.From)
	}
	if p.To.Weekday() != time.Sunday || p.To.Hour() != 23 || p.To.Minute() != 59 || p.To.Second() != 59 {
		t.Errorf("to = %v", p.To)
	}
}

func TestWeekAcrossDSTChange(t *testing.T) {
	madrid := mustLoc(t, "Europe/Madrid")
	// Clocks went forward on Sunday 2025-03-30.
	ts := time.Date(2025, 3, 31, 12, 0, 0, 0, madrid).Unix()
	p := PeriodOf(Week, ts, madrid)
	if p.To.Hour() != 23 || p.To.Minute() != 59 || p.To.Second() != 59 || p.To.Weekday() != time.Sunday {
		t.Errorf("to = %v", p.To)
	}
	prev := PeriodOf(Week, time.Date(2025, 3, 29, 12, 0, 0, 0, madrid).Unix(), madrid)
	if prev.To.Hour() != 23 || prev.To.Minute() != 59 {
		t.Errorf("previous week (contains the DST change) to = %v", prev.To)
	}
}

func TestClosedPeriodsExcludesCurrent(t *testing.T) {
	first := time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC).Unix()
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	ps := ClosedPeriods(Year, first, now, time.UTC)
	if len(ps) != 3 || ps[0].Key != "2023" || ps[2].Key != "2025" {
		t.Fatalf("got %+v", ps)
	}
}

func TestClosedPeriodsEmptyWhenFirstListenIsCurrentPeriod(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	for _, pt := range []PeriodType{Year, Week} {
		if ps := ClosedPeriods(pt, now.Unix(), now, time.UTC); len(ps) != 0 {
			t.Errorf("%s: got %+v, want none", pt, ps)
		}
	}
}
