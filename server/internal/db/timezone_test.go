package database

import (
	"expenser/internal/config"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionUsesTheAppTimezone(t *testing.T) {
	cfg, err := config.LoadConfig(os.Getenv)
	require.NoError(t, err)
	testDB := InitTestDB(cfg)
	defer testDB.Close()

	var tz string
	require.NoError(t, testDB.conn.QueryRow(`SHOW timezone`).Scan(&tz))
	assert.Equal(t, cfg.Location.String(), tz, "EXTRACT and to_char must see the household's days")
}
