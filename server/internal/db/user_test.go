package database

import (
	"context"
	"expenser/internal/config"
	"expenser/internal/models"
	"log"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserByID(t *testing.T) {
	cfg, err := config.LoadConfig()
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
				user := &models.User{
					Username:     "findbyid",
					PasswordHash: "hash123",
				}
				err := testDB.CreateUser(context.Background(), user)
				assert.NoError(t, err)
				return user.ID // Return the ID of the created user
			},
			wantErr: false,
			validate: func(t *testing.T, got *models.User) {
				assert.NotNil(t, got)
				assert.Equal(t, "findbyid", got.Username)
				assert.Equal(t, "hash123", got.PasswordHash)
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

func TestGetUserByUsername(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test GetUserByUsername %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name           string
		setup          func(t *testing.T) string // Returns the username to look for
		targetUsername string                    // The username to pass to GetUserByUsername
		wantErr        bool
		validate       func(t *testing.T, got *models.User)
	}

	tests := []testCase{
		{
			name: "Existing User",
			setup: func(t *testing.T) string {
				ResetTestDB(testDB)
				user := &models.User{
					Username:     "userbyname",
					PasswordHash: "hash456",
				}
				err := testDB.CreateUser(context.Background(), user)
				assert.NoError(t, err)
				return user.Username
			},
			wantErr: false,
			validate: func(t *testing.T, got *models.User) {
				assert.NotNil(t, got)
				assert.Equal(t, "userbyname", got.Username)
			},
		},
		{
			name: "Non-Existing User",
			setup: func(t *testing.T) string {
				ResetTestDB(testDB)
				return "nonexistentusername" // Return a username that won't exist
			},
			targetUsername: "nonexistentusername",
			wantErr:        true,
			validate: func(t *testing.T, got *models.User) {
				assert.Nil(t, got)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			username := tt.setup(t)
			targetUsername := username

			got, err := testDB.GetUserByUsername(context.Background(), targetUsername)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "user not found")
				tt.validate(t, got)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, got)
			tt.validate(t, got)
			assert.Equal(t, targetUsername, got.Username)
		})
	}
}

func TestUserExists(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test UserExists %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name           string
		setup          func(t *testing.T) // Function to populate DB
		username       string
		expectedExists bool
		wantErr        bool
	}

	tests := []testCase{
		{
			name: "User exists by username",
			setup: func(t *testing.T) {
				ResetTestDB(testDB)
				testDB.CreateUser(context.Background(), &models.User{Username: "existuser", PasswordHash: "h"})
			},
			username:       "existuser",
			expectedExists: true,
			wantErr:        false,
		},
		{
			name: "User does not exist",
			setup: func(t *testing.T) {
				ResetTestDB(testDB) // Empty DB
			},
			username:       "nonexistent",
			expectedExists: false,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)

			exists, err := testDB.UserExists(context.Background(), tt.username)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedExists, exists)
		})
	}
}

func TestCreateUserTakenUsername(t *testing.T) {
	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	testDB := InitTestDB(cfg)
	defer testDB.Close()
	ResetTestDB(testDB)

	require.NoError(t, testDB.CreateUser(context.Background(), &models.User{Username: "twin", PasswordHash: "h"}))
	err = testDB.CreateUser(context.Background(), &models.User{Username: "twin", PasswordHash: "h2"})
	assert.ErrorIs(t, err, ErrUsernameTaken)
}
