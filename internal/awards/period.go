package awards

import (
	"fmt"
	"time"
)

type PeriodType string

const (
	Year PeriodType = "year"
	Week PeriodType = "week"
)

// Period is a calendar year or ISO week. To is the last second of the period
// because upstream's ranking queries use an inclusive BETWEEN.
type Period struct {
	Type     PeriodType
	Key      string // "2025" or "2025-W01"
	From, To time.Time
}

// PeriodOf returns the period of type t that contains the unix timestamp ts,
// bucketed in loc.
func PeriodOf(t PeriodType, ts int64, loc *time.Location) Period {
	at := time.Unix(ts, 0).In(loc)
	if t == Year {
		from := time.Date(at.Year(), 1, 1, 0, 0, 0, 0, loc)
		return Period{Type: Year, Key: fmt.Sprintf("%d", at.Year()), From: from, To: endOf(from.AddDate(1, 0, 0))}
	}
	// ISO weeks start on Monday. Date arithmetic (not +24h) keeps DST safe.
	offset := (int(at.Weekday()) + 6) % 7
	from := time.Date(at.Year(), at.Month(), at.Day()-offset, 0, 0, 0, 0, loc)
	y, w := at.ISOWeek()
	return Period{Type: Week, Key: fmt.Sprintf("%d-W%02d", y, w), From: from, To: endOf(from.AddDate(0, 0, 7))}
}

// endOf returns the last second before next, the start of the following period.
func endOf(next time.Time) time.Time { return next.Add(-time.Second) }

// ClosedPeriods lists every period from the one containing firstTS up to, but
// excluding, the one containing now, in ascending order.
func ClosedPeriods(t PeriodType, firstTS int64, now time.Time, loc *time.Location) []Period {
	current := PeriodOf(t, now.Unix(), loc)
	var out []Period
	for p := PeriodOf(t, firstTS, loc); p.From.Before(current.From); {
		out = append(out, p)
		p = PeriodOf(t, p.To.Add(time.Second).Unix(), loc)
	}
	return out
}
