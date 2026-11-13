package services

import (
	"context"
	"errors"
	"testing"

	"expenser/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// fakeAccountStore keeps users by username.
type fakeAccountStore struct {
	fakePasswordStore
	users     map[string]*models.User
	lookupErr error
}

func (f *fakeAccountStore) UserExists(_ context.Context, username string) (bool, error) {
	_, ok := f.users[username]
	return ok, f.lookupErr
}

func (f *fakeAccountStore) CreateUser(_ context.Context, u *models.User) error {
	if _, ok := f.users[u.Username]; ok {
		return models.ErrUsernameTaken
	}
	u.ID = uuid.New()
	f.users[u.Username] = u
	return nil
}

func (f *fakeAccountStore) GetUserByUsername(_ context.Context, username string) (*models.User, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	u, ok := f.users[username]
	if !ok {
		return nil, models.ErrUserNotFound
	}
	return u, nil
}

func newAccounts() (*AccountService, *fakeAccountStore) {
	store := &fakeAccountStore{users: map[string]*models.User{}}
	s := NewAccountService(store)
	s.cost = bcrypt.MinCost
	return s, store
}

func TestRegisterAndLogin(t *testing.T) {
	s, _ := newAccounts()
	ctx := context.Background()

	user, err := s.Register(ctx, "ana", "secret-1")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, user.ID)
	assert.NotEqual(t, "secret-1", user.PasswordHash, "stored hashed")

	_, err = s.Register(ctx, "ana", "other-pw")
	assert.ErrorIs(t, err, models.ErrUsernameTaken)

	got, err := s.Login(ctx, "ana", "secret-1")
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)

	_, err = s.Login(ctx, "ana", "wrong")
	assert.ErrorIs(t, err, ErrInvalidCredentials)
	_, err = s.Login(ctx, "nobody", "secret-1")
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestLoginDatabaseFailureIsNotWrongPassword(t *testing.T) {
	s, store := newAccounts()
	store.lookupErr = errors.New("connection refused")
	_, err := s.Login(context.Background(), "ana", "secret-1")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidCredentials)

	_, err = s.Register(context.Background(), "ana", "secret-1")
	assert.ErrorContains(t, err, "connection refused")
}
