package database

import (
	"expenser/internal/config"
	"expenser/internal/models"
	"log"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
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
				err := testDB.CreateUser(user)
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
			targetID: uuid.Nil, // Will be replaced by setup's return
			wantErr:  true,
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
			targetID := userID

			got, err := testDB.GetUserByID(targetID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "user not found")
				tt.validate(t, got)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, got)
			tt.validate(t, got)
			assert.Equal(t, targetID, got.ID) // Confirm the ID matches
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
				err := testDB.CreateUser(user)
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

			got, err := testDB.GetUserByUsername(targetUsername)
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

func TestUpdateUser(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test UpdateUser %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name     string
		setup    func(t *testing.T) *models.User // Returns the user to be updated
		updates  func(user *models.User)         // Function to apply updates to the user model
		wantErr  bool
		validate func(t *testing.T, originalUser *models.User, updatedUser *models.User)
	}

	tests := []testCase{
		{
			name: "Successful Update",
			setup: func(t *testing.T) *models.User {
				ResetTestDB(testDB)
				user := &models.User{
					Username:     "olduser",
					PasswordHash: "oldhash",
				}
				err := testDB.CreateUser(user)
				assert.NoError(t, err)
				return user // Return the created user with its ID
			},
			updates: func(user *models.User) {
				user.Username = "newuser"
				user.PasswordHash = "newhash"
			},
			wantErr: false,
			validate: func(t *testing.T, originalUser *models.User, updatedUser *models.User) {
				assert.Equal(t, originalUser.ID, updatedUser.ID) // ID should remain same
				assert.Equal(t, "newuser", updatedUser.Username)
				assert.Equal(t, "newhash", updatedUser.PasswordHash)
				assert.NotEqual(t, originalUser.UpdatedAt, updatedUser.UpdatedAt) // UpdatedAt should change
				assert.True(t, updatedUser.UpdatedAt.After(originalUser.UpdatedAt))
			},
		},
		{
			name: "Update Non-Existing User",
			setup: func(t *testing.T) *models.User {
				ResetTestDB(testDB)
				// Create a user model with a non-existent ID
				return &models.User{
					ID:           uuid.New(), // A random, non-existent ID
					Username:     "fakeuser",
					PasswordHash: "fakehash",
				}
			},
			updates: func(user *models.User) {
				user.Username = "attempted_update"
			},
			wantErr: true,
			validate: func(t *testing.T, originalUser *models.User, updatedUser *models.User) {
				assert.Nil(t, updatedUser) // No user should be returned if not found
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userToUpdate := tt.setup(t)

			tt.updates(userToUpdate) // Apply the updates to the user model

			err := testDB.UpdateUser(userToUpdate)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "user not found") // Or specific DB error message
				return
			}
			assert.NoError(t, err)

			// Retrieve the user from DB to verify changes
			gotUser, err := testDB.GetUserByID(userToUpdate.ID)
			assert.NoError(t, err)
			tt.validate(t, userToUpdate, gotUser) // Pass original and fetched user
		})
	}
}

func TestDeleteUser(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test DeleteUser %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name     string
		setup    func(t *testing.T) uuid.UUID // Returns the ID of the user to be deleted
		targetID uuid.UUID                    // The ID to pass to DeleteUser
		wantErr  bool
		validate func(t *testing.T, deletedID uuid.UUID)
	}

	tests := []testCase{
		{
			name: "Successful Deletion",
			setup: func(t *testing.T) uuid.UUID {
				ResetTestDB(testDB)
				user := &models.User{
					Username:     "todelete",
					PasswordHash: "deletehash",
				}
				err := testDB.CreateUser(user)
				assert.NoError(t, err)
				return user.ID
			},
			wantErr: false,
			validate: func(t *testing.T, deletedID uuid.UUID) {
				// Verify user is no longer in the DB
				_, err := testDB.GetUserByID(deletedID)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "user not found")
			},
		},
		{
			name: "Delete Non-Existing User",
			setup: func(t *testing.T) uuid.UUID {
				ResetTestDB(testDB)
				return uuid.New() // A random, non-existent ID
			},
			targetID: uuid.Nil, // Will be replaced by setup's return
			wantErr:  true,
			validate: func(t *testing.T, deletedID uuid.UUID) {
				// No specific validation needed beyond error assertion
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := tt.setup(t)
			targetID := userID

			err := testDB.DeleteUser(targetID) // Assuming DeleteUser takes uuid.UUID, not int as in your original snippet
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "user not found")
				return
			}
			assert.NoError(t, err)
			tt.validate(t, targetID)
		})
	}
}

func TestListUsers(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test ListUsers %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name          string
		setup         func(t *testing.T) uuid.UUID // Returns ID of the user who is "active" or for context (not strictly needed for List)
		limit         int
		offset        int
		expectedCount int // Expected number of users in the result slice
		wantErr       bool
		validate      func(t *testing.T, got []*models.User)
	}

	tests := []testCase{
		{
			name: "List with multiple users (limit 2, offset 0)",
			setup: func(t *testing.T) uuid.UUID {
				ResetTestDB(testDB)
				// Create a few users to list
				user1 := &models.User{Username: "listuser1", PasswordHash: "h1"}
				user2 := &models.User{Username: "listuser2", PasswordHash: "h2"}
				user3 := &models.User{Username: "listuser3", PasswordHash: "h3"}
				testDB.CreateUser(user1)
				testDB.CreateUser(user2)
				testDB.CreateUser(user3)
				return user1.ID // Return one ID for consistency, not directly used by ListUsers
			},
			limit:         2,
			offset:        0,
			expectedCount: 2,
			wantErr:       false,
			validate: func(t *testing.T, got []*models.User) {
				assert.Len(t, got, 2)
				// Assert specific properties if needed, respecting ORDER BY (created_at DESC in your func)
				assert.Equal(t, "listuser3", got[0].Username) // Assuming user3 was created last
				assert.Equal(t, "listuser2", got[1].Username)
			},
		},
		{
			name: "List with offset",
			setup: func(t *testing.T) uuid.UUID {
				ResetTestDB(testDB)
				user1 := &models.User{Username: "offuser1", PasswordHash: "h1"}
				user2 := &models.User{Username: "offuser2", PasswordHash: "h2"}
				user3 := &models.User{Username: "offuser3", PasswordHash: "h3"}
				testDB.CreateUser(user1)
				testDB.CreateUser(user2)
				testDB.CreateUser(user3)
				return user1.ID
			},
			limit:         2,
			offset:        1,
			expectedCount: 2,
			wantErr:       false,
			validate: func(t *testing.T, got []*models.User) {
				assert.Len(t, got, 2)
				// Order should be by created_at DESC, so offset 1 skips user3 (newest)
				assert.Equal(t, "offuser2", got[0].Username)
				assert.Equal(t, "offuser1", got[1].Username)
			},
		},
		{
			name: "No users in DB",
			setup: func(t *testing.T) uuid.UUID {
				ResetTestDB(testDB) // Ensure DB is empty
				return uuid.New()
			},
			limit:         10,
			offset:        0,
			expectedCount: 0,
			wantErr:       false,
			validate: func(t *testing.T, got []*models.User) {
				assert.Empty(t, got)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t) // Perform setup, creating users as needed

			got, err := testDB.ListUsers(tt.limit, tt.offset)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Len(t, got, tt.expectedCount)
			tt.validate(t, got)
		})
	}
}

func TestGetUserCount(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicf("Couldn't load config in test GetUserCount %v", err)
		t.FailNow()
	}

	testDB := InitTestDB(cfg)
	defer testDB.Close()

	type testCase struct {
		name          string
		setup         func(t *testing.T) // Function to populate DB
		expectedCount int
		wantErr       bool
	}

	tests := []testCase{
		{
			name: "Count with multiple users",
			setup: func(t *testing.T) {
				ResetTestDB(testDB)
				testDB.CreateUser(&models.User{Username: "c1", PasswordHash: "h"})
				testDB.CreateUser(&models.User{Username: "c2", PasswordHash: "h"})
				testDB.CreateUser(&models.User{Username: "c3", PasswordHash: "h"})
			},
			expectedCount: 3,
			wantErr:       false,
		},
		{
			name: "Count with zero users",
			setup: func(t *testing.T) {
				ResetTestDB(testDB) // Ensure DB is empty
			},
			expectedCount: 0,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)

			count, err := testDB.GetUserCount()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCount, count)
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
				testDB.CreateUser(&models.User{Username: "existuser", PasswordHash: "h"})
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

			exists, err := testDB.UserExists(tt.username)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedExists, exists)
		})
	}
}
