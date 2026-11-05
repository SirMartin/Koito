package awards

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
)

func get(t *testing.T, h http.HandlerFunc, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Get("/awards/{type}/{id}", h)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestAwardsHandlerReturnsSummaryJSON(t *testing.T) {
	s := newStore(t)
	seed(t, s, plays("A", 1, t0)...)
	id := idByName(t, s, Artist, "A")
	insertAward(t, s, Year, "2024", Artist, id, 1)
	insertAward(t, s, Week, "2024-W18", Artist, id, 1)

	rec := get(t, Handler(s), "/awards/artist/"+strconv.Itoa(int(id)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	var sm Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &sm); err != nil {
		t.Fatal(err)
	}
	if !eq(sm.Gold, []int{2024}) || sm.WeeksAtOne != 1 {
		t.Fatalf("got %+v", sm)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q", ct)
	}
}

func TestAwardsHandlerRejectsUnknownType(t *testing.T) {
	s := newStore(t)
	if rec := get(t, Handler(s), "/awards/playlist/1"); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAwardsHandlerRejectsBadID(t *testing.T) {
	s := newStore(t)
	if rec := get(t, Handler(s), "/awards/track/abc"); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAwardsHandlerUnknownItemReturnsEmptySummary(t *testing.T) {
	s := newStore(t)
	rec := get(t, Handler(s), "/awards/album/424242")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	want := `{"gold":[],"silver":[],"bronze":[],"weeks_at_one":0,"certification":""}`
	if got := rec.Body.String(); got != want+"\n" {
		t.Fatalf("body = %q", got)
	}
}
