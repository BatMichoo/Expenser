package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseReminderCheckInterval(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"", DefaultReminderCheckInterval},
		{"garbage", DefaultReminderCheckInterval},
		{"-5m", DefaultReminderCheckInterval},
		{"0s", DefaultReminderCheckInterval},
		{"1ns", MinReminderCheckInterval},
		{"1m", MinReminderCheckInterval},
		{"29m59s", MinReminderCheckInterval},
		{"30m", 30 * time.Minute},
		{"45m", 45 * time.Minute},
		{"48h", 48 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, parseReminderCheckInterval(tt.in))
		})
	}
}
