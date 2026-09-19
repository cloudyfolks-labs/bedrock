package maintenance

import (
	"fmt"
	"strings"
	"time"
)

type Window struct {
	Days  []time.Weekday
	Start int
	End   int
}

var dayNames = map[string]time.Weekday{"Mon": time.Monday, "Tue": time.Tuesday, "Wed": time.Wednesday, "Thu": time.Thursday, "Fri": time.Friday, "Sat": time.Saturday, "Sun": time.Sunday}

func Parse(s string) (Window, error) {
	days, span, ok := strings.Cut(strings.TrimSpace(s), " ")
	if !ok {
		return Window{}, fmt.Errorf("maintenance window %q: want <Days> HH:MM-HH:MM", s)
	}
	var window Window
	for _, name := range strings.Split(days, ",") {
		day, known := dayNames[name]
		if !known {
			return Window{}, fmt.Errorf("maintenance window %q: unknown day %q", s, name)
		}
		window.Days = append(window.Days, day)
	}
	from, to, ok := strings.Cut(span, "-")
	if !ok {
		return Window{}, fmt.Errorf("maintenance window %q: want HH:MM-HH:MM", s)
	}
	start, err := minutes(from)
	if err != nil {
		return Window{}, fmt.Errorf("maintenance window %q: %w", s, err)
	}
	end, err := minutes(to)
	if err != nil {
		return Window{}, fmt.Errorf("maintenance window %q: %w", s, err)
	}
	if end <= start {
		return Window{}, fmt.Errorf("maintenance window %q: end must be after start on the same day", s)
	}
	window.Start, window.End = start, end
	return window, nil
}

func minutes(clock string) (int, error) {
	var hour, minute int
	if _, err := fmt.Sscanf(clock, "%d:%d", &hour, &minute); err != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, fmt.Errorf("bad time %q", clock)
	}
	return hour*60 + minute, nil
}

func (w Window) Open(now time.Time) bool {
	minute := now.Hour()*60 + now.Minute()
	for _, day := range w.Days {
		if now.Weekday() == day && minute >= w.Start && minute < w.End {
			return true
		}
	}
	return false
}
