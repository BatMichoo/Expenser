package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTimeWindow(t *testing.T) {
	w, err := ParseTimeWindow("09:00-18:00")
	require.NoError(t, err)
	assert.Equal(t, TimeWindow{9 * time.Hour, 18 * time.Hour}, w)

	w, err = ParseTimeWindow("8:30-20:00")
	require.NoError(t, err)
	assert.Equal(t, TimeWindow{8*time.Hour + 30*time.Minute, 20 * time.Hour}, w)

	w, err = ParseTimeWindow(" 07:00 - 24:00 ")
	require.NoError(t, err)
	assert.Equal(t, TimeWindow{7 * time.Hour, 24 * time.Hour}, w)

	for _, bad := range []string{"", "garbage", "18:00-09:00", "09:00-09:00", "25:00-26:00", "09:60-10:00", "09:00", "-1:00-10:00", "24:30-25:00"} {
		_, err := ParseTimeWindow(bad)
		assert.Error(t, err, bad)
	}
}

func sofia(t *testing.T) *time.Location {
	loc, err := time.LoadLocation("Europe/Sofia")
	require.NoError(t, err)
	return loc
}

func TestTimeWindowContains(t *testing.T) {
	loc := sofia(t)
	at := func(h, m int) time.Time { return time.Date(2026, 12, 12, h, m, 0, 0, loc) }
	w := DefaultReminderHours
	assert.False(t, w.Contains(at(8, 59)))
	assert.True(t, w.Contains(at(9, 0)))
	assert.True(t, w.Contains(at(17, 59)))
	assert.False(t, w.Contains(at(18, 0)))
	assert.False(t, w.Contains(at(3, 0)))
}

func TestTimeWindowNextOpen(t *testing.T) {
	loc := sofia(t)
	w := DefaultReminderHours
	at := func(d, h, m int) time.Time { return time.Date(2026, 12, d, h, m, 0, 0, loc) }
	assert.Equal(t, at(12, 9, 0), w.NextOpen(at(12, 3, 0)))
	assert.Equal(t, at(12, 10, 0), w.NextOpen(at(12, 10, 0)))
	assert.Equal(t, at(13, 9, 0), w.NextOpen(at(12, 18, 30)))

	// Clocks go forward at 03:00 on 29 March 2026 in Sofia.
	dst := time.Date(2026, 3, 29, 1, 0, 0, 0, loc)
	assert.Equal(t, time.Date(2026, 3, 29, 9, 0, 0, 0, loc), w.NextOpen(dst))
	assert.Equal(t, 9, w.NextOpen(dst).Hour())
}

func TestTimeWindowString(t *testing.T) {
	assert.Equal(t, "09:00-18:00", DefaultReminderHours.String())
}
