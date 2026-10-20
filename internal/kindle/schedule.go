// Package kindle implements device-independent Kindle scheduling.
package kindle

import (
	"fmt"
	"time"
	_ "time/tzdata"
)

// Window uses ISO weekdays (Monday=1, Sunday=7), local clock minutes,
// and a half-open [start,end) interval. Overnight windows must be split.
type Window struct {
	Days  []int `json:"days"`
	Start int   `json:"start_minute"`
	End   int   `json:"end_minute"`
	Every int   `json:"interval_minutes"`
}
type Schedule struct {
	ShowStatus  bool     `json:"show_status"`
	Orientation string   `json:"orientation"`
	Timezone    string   `json:"timezone"`
	Enabled     bool     `json:"enabled"`
	Windows     []Window `json:"windows"`
}

func DefaultSchedule() Schedule {
	return Schedule{ShowStatus: true, Orientation: "portrait", Timezone: "Asia/Shanghai", Enabled: true, Windows: []Window{
		{[]int{1, 2, 3, 4, 5}, 1080, 1440, 30},
		{[]int{6, 7}, 540, 1440, 30},
	}}
}
func (s Schedule) Validate() error {
	if s.Orientation != "" && s.Orientation != "portrait" && s.Orientation != "landscape_cw" && s.Orientation != "landscape_ccw" {
		return fmt.Errorf("invalid orientation")
	}
	if _, err := time.LoadLocation(s.Timezone); err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}
	if len(s.Windows) > 32 {
		return fmt.Errorf("too many windows")
	}
	if s.Enabled && len(s.Windows) == 0 {
		return fmt.Errorf("enabled schedule needs a window")
	}
	for _, w := range s.Windows {
		if len(w.Days) == 0 || w.Start < 0 || w.End > 1440 || w.Start >= w.End || w.Every < 1 || w.Every > 1440 {
			return fmt.Errorf("invalid refresh window")
		}
		for _, d := range w.Days {
			if d < 1 || d > 7 {
				return fmt.Errorf("invalid weekday")
			}
		}
	}
	return nil
}

// Next returns the first slot strictly after after. A disabled schedule returns
// zero. Slots are aligned to each window's start, not the last download time.
func (s Schedule) Next(after time.Time) (time.Time, error) {
	if err := s.Validate(); err != nil {
		return time.Time{}, err
	}
	if !s.Enabled {
		return time.Time{}, nil
	}
	loc, _ := time.LoadLocation(s.Timezone)
	local := after.In(loc)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	var best time.Time
	for n := 0; n <= 7; n++ {
		day := midnight.AddDate(0, 0, n)
		dow := int(day.Weekday())
		if dow == 0 {
			dow = 7
		}
		for _, w := range s.Windows {
			matches := false
			for _, d := range w.Days {
				if d == dow {
					matches = true
				}
			}
			if !matches {
				continue
			}
			for minute := w.Start; minute < w.End; minute += w.Every {
				slot := time.Date(day.Year(), day.Month(), day.Day(), minute/60, minute%60, 0, 0, loc)
				// Skip local clock times normalized across a DST gap.
				if slot.Hour() != minute/60 || slot.Minute() != minute%60 {
					continue
				}
				if slot.After(after) && (best.IsZero() || slot.Before(best)) {
					best = slot
				}
			}
		}
	}
	return best, nil
}
