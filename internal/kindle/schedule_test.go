package kindle

import (
	"testing"
	"time"
)

func TestDefaultSchedule(t *testing.T) {
	s := DefaultSchedule()
	for _, tc := range []struct{ from, want string }{
		{"2026-10-19T17:59:59+08:00", "2026-10-19T18:00:00+08:00"},
		{"2026-10-19T18:00:00+08:00", "2026-10-19T18:30:00+08:00"},
		{"2026-10-19T18:31:11+08:00", "2026-10-19T19:00:00+08:00"},
		{"2026-10-19T23:30:00+08:00", "2026-10-20T18:00:00+08:00"},
		{"2026-10-23T23:30:00+08:00", "2026-10-24T09:00:00+08:00"},
		{"2026-10-25T23:30:00+08:00", "2026-10-26T18:00:00+08:00"},
		{"2026-10-19T10:00:00Z", "2026-10-19T18:30:00+08:00"},
	} {
		from, _ := time.Parse(time.RFC3339, tc.from)
		got, err := s.Next(from)
		if err != nil || got.Format(time.RFC3339) != tc.want {
			t.Errorf("%s: got %v (%v), want %s", tc.from, got, err, tc.want)
		}
	}
}
func TestSlotCounts(t *testing.T) {
	s := DefaultSchedule()
	for _, tc := range []struct {
		date  string
		count int
	}{{"2026-10-19", 12}, {"2026-10-24", 30}} {
		start, _ := time.Parse(time.RFC3339, tc.date+"T00:00:00+08:00")
		cursor := start
		count := 0
		for {
			next, err := s.Next(cursor)
			if err != nil {
				t.Fatal(err)
			}
			if !next.Before(start.AddDate(0, 0, 1)) {
				break
			}
			count++
			cursor = next
		}
		if count != tc.count {
			t.Fatalf("got %d want %d", count, tc.count)
		}
	}
}
func TestDisabledAndValidation(t *testing.T) {
	s := DefaultSchedule()
	s.Enabled = false
	next, err := s.Next(time.Now())
	if err != nil || !next.IsZero() {
		t.Fatal(next, err)
	}
	for _, mutate := range []func(*Schedule){
		func(s *Schedule) { s.Timezone = "invalid" },
		func(s *Schedule) { s.Windows[0].Every = 0 },
		func(s *Schedule) { s.Windows[0].Days = []int{0} },
		func(s *Schedule) { s.Windows[0].End = 1441 },
		func(s *Schedule) { s.Windows[0].Start = s.Windows[0].End },
		func(s *Schedule) { s.Windows = nil },
	} {
		s := DefaultSchedule()
		mutate(&s)
		if s.Validate() == nil {
			t.Fatal("accepted invalid schedule")
		}
	}
}
