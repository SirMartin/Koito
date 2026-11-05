package awards

import (
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Location.String() != "UTC" {
		t.Errorf("location = %s, want UTC", cfg.Location)
	}
	if cfg.Track != (Thresholds{250, 500, 1000}) {
		t.Errorf("track thresholds = %+v", cfg.Track)
	}
	if cfg.Album != (Thresholds{500, 750, 1500}) {
		t.Errorf("album thresholds = %+v", cfg.Album)
	}
}

func TestLoadConfigTimezonePrecedence(t *testing.T) {
	cfg, err := LoadConfig(env(map[string]string{
		"KOITO_AWARDS_TZ": "Europe/Madrid",
		"KOITO_FORCE_TZ":  "America/New_York",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Location.String() != "Europe/Madrid" {
		t.Errorf("AWARDS_TZ should win, got %s", cfg.Location)
	}

	cfg, err = LoadConfig(env(map[string]string{"KOITO_FORCE_TZ": "America/New_York"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Location.String() != "America/New_York" {
		t.Errorf("FORCE_TZ should beat UTC, got %s", cfg.Location)
	}
}

func TestLoadConfigRejectsBadTimezone(t *testing.T) {
	if _, err := LoadConfig(env(map[string]string{"KOITO_AWARDS_TZ": "Nope/Nowhere"})); err == nil {
		t.Fatal("expected error for bad timezone")
	}
}

func TestLoadConfigRejectsUnorderedThresholds(t *testing.T) {
	_, err := LoadConfig(env(map[string]string{
		"KOITO_CERT_GOLD":     "600",
		"KOITO_CERT_PLATINUM": "500",
	}))
	if err == nil {
		t.Fatal("expected error when gold > platinum")
	}
}

func TestCertificationBoundaries(t *testing.T) {
	th := Thresholds{250, 500, 1000}
	cases := map[int64]string{
		249: "", 250: "gold", 499: "gold", 500: "platinum", 999: "platinum", 1000: "diamond",
	}
	for n, want := range cases {
		if got := th.Certification(n); got != want {
			t.Errorf("Certification(%d) = %q, want %q", n, got, want)
		}
	}
}
