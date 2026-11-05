// Package awards computes Maloja-style awards (yearly medals, weeks at #1 and
// listen-count certifications) on top of upstream's ranking queries.
package awards

import (
	"fmt"
	"strconv"
	"time"
)

// Thresholds are the listen counts needed for each certification level.
type Thresholds struct {
	Gold, Platinum, Diamond int
}

// Config holds the awards settings read from the environment.
type Config struct {
	Location *time.Location
	Track    Thresholds
	Album    Thresholds
}

// Certification returns "", "gold", "platinum" or "diamond" for a listen count.
func (t Thresholds) Certification(listens int64) string {
	switch {
	case listens >= int64(t.Diamond):
		return "diamond"
	case listens >= int64(t.Platinum):
		return "platinum"
	case listens >= int64(t.Gold):
		return "gold"
	}
	return ""
}

// LoadConfig reads the configuration through getenv so tests do not touch the
// process environment.
func LoadConfig(getenv func(string) string) (Config, error) {
	tz := getenv("KOITO_AWARDS_TZ")
	if tz == "" {
		tz = getenv("KOITO_FORCE_TZ")
	}
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return Config{}, fmt.Errorf("awards: invalid timezone %q: %w", tz, err)
	}

	track, err := loadThresholds(getenv, "", Thresholds{250, 500, 1000})
	if err != nil {
		return Config{}, err
	}
	album, err := loadThresholds(getenv, "_ALBUM", Thresholds{500, 750, 1500})
	if err != nil {
		return Config{}, err
	}
	return Config{Location: loc, Track: track, Album: album}, nil
}

func loadThresholds(getenv func(string) string, suffix string, def Thresholds) (Thresholds, error) {
	t := def
	for _, f := range []struct {
		name string
		dst  *int
	}{
		{"KOITO_CERT_GOLD" + suffix, &t.Gold},
		{"KOITO_CERT_PLATINUM" + suffix, &t.Platinum},
		{"KOITO_CERT_DIAMOND" + suffix, &t.Diamond},
	} {
		v := getenv(f.name)
		if v == "" {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return Thresholds{}, fmt.Errorf("awards: invalid %s=%q", f.name, v)
		}
		*f.dst = n
	}
	if !(t.Gold <= t.Platinum && t.Platinum <= t.Diamond) {
		return Thresholds{}, fmt.Errorf("awards: thresholds%s must satisfy gold <= platinum <= diamond", suffix)
	}
	return t, nil
}
