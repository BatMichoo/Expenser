package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"expenser/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

type fakePasswordStore struct {
	user    models.User
	updated string
	saveErr error
}

func (f *fakePasswordStore) GetUserByID(id uuid.UUID) (*models.User, error) {
	if id != f.user.ID {
		return nil, errors.New("user not found")
	}
	u := f.user
	return &u, nil
}

func (f *fakePasswordStore) UpdateUserPassword(_ context.Context, id uuid.UUID, hash string) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.updated = hash
	return nil
}

func newPasswordStore(t *testing.T, current string) *fakePasswordStore {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(current), bcrypt.MinCost)
	require.NoError(t, err)
	return &fakePasswordStore{user: models.User{ID: uuid.New(), PasswordHash: string(hash)}}
}

func TestChangePassword(t *testing.T) {
	store := newPasswordStore(t, "old-secret")
	err := ChangePassword(context.Background(), store, store.user.ID, "old-secret", "new-secret", "new-secret")
	require.NoError(t, err)
	require.NotEmpty(t, store.updated)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(store.updated), []byte("new-secret")), "stores a hash of the new password")
	assert.NotContains(t, store.updated, "new-secret")
}

func TestChangePasswordRefusals(t *testing.T) {
	for name, tc := range map[string]struct {
		current, next, confirm string
		want                   error
	}{
		"wrong current":   {"nope", "new-secret", "new-secret", ErrWrongPassword},
		"too short":       {"old-secret", "12345", "12345", models.ErrPasswordTooShort},
		"too long":        {"old-secret", strings.Repeat("x", 73), strings.Repeat("x", 73), models.ErrPasswordTooLong},
		"mismatch":        {"old-secret", "new-secret", "new-secreT", models.ErrPasswordMismatch},
		"same as current": {"old-secret", "old-secret", "old-secret", models.ErrPasswordUnchanged},
	} {
		t.Run(name, func(t *testing.T) {
			store := newPasswordStore(t, "old-secret")
			err := ChangePassword(context.Background(), store, store.user.ID, tc.current, tc.next, tc.confirm)
			assert.ErrorIs(t, err, tc.want)
			assert.Empty(t, store.updated, "nothing saved")
		})
	}
}

func TestChangePasswordSaveError(t *testing.T) {
	store := newPasswordStore(t, "old-secret")
	store.saveErr = errors.New("db down")
	err := ChangePassword(context.Background(), store, store.user.ID, "old-secret", "new-secret", "new-secret")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrWrongPassword)
}
