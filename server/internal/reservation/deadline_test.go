package reservation

import (
	"testing"
	"time"
)

func TestEventEndUsesAuthorsTimeZone(t *testing.T) {
	end, err := EventEnd("2026-03-08", "America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 3, 9, 4, 0, 0, 0, time.UTC)
	if !end.Equal(want) {
		t.Fatalf("end = %v, want %v", end, want)
	}
	for _, tc := range []struct{ date, zone string }{
		{"2026-02-29", "UTC"},
		{"2026-03-08", "Local"},
		{"2026-03-08", "invalid/zone"},
		{"2026-03-08", ""},
	} {
		if _, err := EventEnd(tc.date, tc.zone); err == nil {
			t.Fatalf("expected invalid date or zone for %+v", tc)
		}
	}
}

func TestInitialDeadline(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		days  int
		event bool
		want  int
	}{
		{"without event", 0, false, 14},
		{"far event", 30, true, 14},
		{"between 5 and 19 days", 10, true, 5},
		{"exactly 5 days", 5, true, 5},
		{"less than 5 days", 3, true, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var event *time.Time
			if tc.event {
				value := now.AddDate(0, 0, tc.days)
				event = &value
			}
			want := now.Add(time.Duration(tc.want) * 24 * time.Hour)
			if got := InitialDeadline(now, event); !got.Equal(want) {
				t.Fatalf("InitialDeadline() = %v, want %v", got, want)
			}
		})
	}
}

func TestInitialDeadlineForCalendarEvent(t *testing.T) {
	event, err := EventEnd("2026-12-20", "Europe/Chisinau")
	if err != nil {
		t.Fatal(err)
	}
	location := event.Location()
	for _, tc := range []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"more than 14 days", time.Date(2026, 12, 1, 12, 0, 0, 0, location), time.Date(2026, 12, 15, 12, 0, 0, 0, location)},
		{"five day cutoff", time.Date(2026, 12, 10, 12, 0, 0, 0, location), time.Date(2026, 12, 16, 0, 0, 0, 0, location)},
		{"close to event", time.Date(2026, 12, 18, 12, 0, 0, 0, location), event},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := InitialDeadline(tc.now, &event); !got.Equal(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExtendedDeadline(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for _, days := range []int{3, 14, 30} {
		event := now.AddDate(0, 0, days)
		want := now.Add(14 * 24 * time.Hour)
		if days < 14 {
			want = event
		}
		if got := ExtendedDeadline(now, &event); !got.Equal(want) {
			t.Fatalf("event in %d days: got %v, want %v", days, got, want)
		}
	}
	if got := ExtendedDeadline(now, nil); !got.Equal(now.Add(14 * 24 * time.Hour)) {
		t.Fatalf("no event: got %v", got)
	}
}

func TestShortenedDeadline(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	deadline := now.Add(14 * 24 * time.Hour)
	earlier := now.Add(5 * 24 * time.Hour)
	later := now.Add(20 * 24 * time.Hour)
	for _, tc := range []struct {
		name  string
		event *time.Time
		want  time.Time
	}{
		{"event added earlier", &earlier, earlier},
		{"event moved later", &later, deadline},
		{"event removed", nil, deadline},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShortenedDeadline(deadline, tc.event); !got.Equal(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestInitialDeadlineUsesCalendarDaysAcrossDST(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("timezone data unavailable: %v", err)
	}
	event := time.Date(2026, 3, 11, 23, 59, 0, 0, location)
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, location)
	want := time.Date(2026, 3, 6, 23, 59, 0, 0, location)
	if got := InitialDeadline(now, &event); !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
