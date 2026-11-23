package models

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TimeWindow is a daily span of local time, e.g. 09:00-18:00: Start is
// inclusive, End exclusive, both measured from midnight.
type TimeWindow struct {
	Start, End time.Duration
}

// DefaultReminderHours is when reminder emails may be sent.
var DefaultReminderHours = TimeWindow{Start: 9 * time.Hour, End: 18 * time.Hour}

// ParseTimeWindow reads "HH:MM-HH:MM" (hours may have one digit). Start must
// be before End; times run from 00:00 to 24:00.
func ParseTimeWindow(s string) (TimeWindow, error) {
	from, to, ok := strings.Cut(strings.TrimSpace(s), "-")
	if !ok {
		return TimeWindow{}, fmt.Errorf("time window %q: want HH:MM-HH:MM", s)
	}
	start, err := parseClock(from)
	if err != nil {
		return TimeWindow{}, fmt.Errorf("time window %q: %w", s, err)
	}
	end, err := parseClock(to)
	if err != nil {
		return TimeWindow{}, fmt.Errorf("time window %q: %w", s, err)
	}
	if start >= end {
		return TimeWindow{}, fmt.Errorf("time window %q: start must be before end", s)
	}
	return TimeWindow{Start: start, End: end}, nil
}

func parseClock(s string) (time.Duration, error) {
	hh, mm, ok := strings.Cut(strings.TrimSpace(s), ":")
	h, errH := strconv.Atoi(hh)
	m, errM := strconv.Atoi(mm)
	if !ok || errH != nil || errM != nil || len(mm) != 2 || h < 0 || m < 0 || m > 59 || h > 24 || (h == 24 && m != 0) {
		return 0, fmt.Errorf("%q is not a time of day", s)
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute, nil
}

// String is the window as "HH:MM-HH:MM".
func (w TimeWindow) String() string {
	clock := func(d time.Duration) string { return fmt.Sprintf("%02d:%02d", int(d.Hours()), int(d.Minutes())%60) }
	return clock(w.Start) + "-" + clock(w.End)
}

// sinceMidnight is t's wall-clock time of day in its own location.
func sinceMidnight(t time.Time) time.Duration {
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute +
		time.Duration(t.Second())*time.Second + time.Duration(t.Nanosecond())
}

// Contains reports whether t's local time of day is inside the window.
func (w TimeWindow) Contains(t time.Time) bool {
	d := sinceMidnight(t)
	return d >= w.Start && d < w.End
}

// NextOpenAfter is the next time the window opens strictly after t: today's
// start before it, tomorrow's once it has opened. It uses wall clock time,
// so a daylight-saving change doesn't shift the opening hour.
func (w TimeWindow) NextOpenAfter(t time.Time) time.Time {
	day := t
	if sinceMidnight(t) >= w.Start {
		day = t.AddDate(0, 0, 1)
	}
	h, m := int(w.Start/time.Hour), int(w.Start%time.Hour/time.Minute)
	return time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, t.Location())
}
