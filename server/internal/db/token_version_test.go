package database

import (
	"context"
	"database/sql"
	"expenser/internal/config"
	"expenser/internal/models"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasswordChangeRaisesTokenVersion(t *testing.T) {
	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	testDB := InitTestDB(cfg)
	defer testDB.Close()
	ResetTestDB(testDB)

	user := &models.User{Username: "versioned", PasswordHash: "h"}
	require.NoError(t, testDB.CreateUser(user))
	ctx := context.Background()

	v, err := testDB.TokenVersion(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, v)

	require.NoError(t, testDB.UpdateUserPassword(ctx, user.ID, "h2"))
	v, err = testDB.TokenVersion(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, v)

	got, err := testDB.GetUserByID(user.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, got.TokenVersion)
	got, err = testDB.GetUserByUsername("versioned")
	require.NoError(t, err)
	assert.Equal(t, 1, got.TokenVersion)

	_, err = testDB.TokenVersion(ctx, uuid.New())
	assert.ErrorIs(t, err, sql.ErrNoRows)
}
