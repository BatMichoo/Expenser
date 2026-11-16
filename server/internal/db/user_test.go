package database

import (
	"context"
	"expenser/internal/config"
	"expenser/internal/models"
	"log"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserByID(t *testing.T) {
	cfg, err := config.LoadConfig(os.Getenv)
	if err != nil {
		log.Panicf("Couldn't load config in test GetUserByID %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name     string
		setup    func(t *testing.T) uuid.UUID
		targetID uuid.UUID
		wantErr  bool
		validate func(t *testing.T, got *models.User)
	}

	tests := []testCase{
		{
			name: "Existing User",
			setup: func(t *testing.T) uuid.UUID {
				ResetTestDB(testDB)
				user := &models.User{DisplayName: "findbyid"}
				err := testDB.CreateUser(context.Background(), user)
				assert.NoError(t, err)
				return user.ID // Return the ID of the created user
			},
			wantErr: false,
			validate: func(t *testing.T, got *models.User) {
				assert.NotNil(t, got)
				assert.Equal(t, "findbyid", got.DisplayName)
				assert.NotZero(t, got.CreatedAt)
				assert.NotZero(t, got.UpdatedAt)
			},
		},
		{
			name: "Non-Existing User",
			setup: func(t *testing.T) uuid.UUID {
				ResetTestDB(testDB)
				return uuid.New() // Return a random, non-existent UUID
			},
			wantErr: true,
			validate: func(t *testing.T, got *models.User) {
				assert.Nil(t, got)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := tt.setup(t)
			// For "Non-Existing User" case, targetID comes from setup.
			// For "Existing User", setup creates and returns the ID.
			// We need to pass the ID obtained from setup to GetUserByID.

			got, err := testDB.GetUserByID(context.Background(), userID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "user not found")
				tt.validate(t, got)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, got)
			tt.validate(t, got)
			assert.Equal(t, userID, got.ID) // Confirm the ID matches
		})
	}
}

func TestEnsureUser(t *testing.T) {
	cfg, err := config.LoadConfig(os.Getenv)
	require.NoError(t, err)
	testDB := InitTestDB(cfg)
	defer testDB.Close()
	ResetTestDB(testDB)
	ctx := context.Background()

	// Someone new from the home server: a row with their id, no username.
	id := uuid.New()
	require.NoError(t, testDB.EnsureUser(ctx, id, "Miro", "bg"))
	u, err := testDB.GetUserByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "Miro", u.DisplayName)
	assert.Equal(t, "bg", u.PreferredLanguage)
	assert.Empty(t, u.Username)
	assert.Equal(t, models.DefaultReminderDefaults(), u.ReminderDefaults)

	// Unchanged: nothing written.
	require.NoError(t, testDB.EnsureUser(ctx, id, "Miro", "bg"))
	again, err := testDB.GetUserByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, u.UpdatedAt, again.UpdatedAt)

	// A new name or language is kept; an empty name clears it.
	require.NoError(t, testDB.EnsureUser(ctx, id, "", "en"))
	u, err = testDB.GetUserByID(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, u.DisplayName)
	assert.Equal(t, "en", u.PreferredLanguage)
}
