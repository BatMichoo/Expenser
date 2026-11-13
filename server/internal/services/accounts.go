package services

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"expenser/internal/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidCredentials means the username or the password is wrong. Which
// one is not said, so usernames can't be probed.
var ErrInvalidCredentials = errors.New("invalid username or password")

// AccountStore is what registering, logging in and changing a password
// need from the database. *database.DB implements it.
type AccountStore interface {
	PasswordStore
	UserExists(ctx context.Context, username string) (bool, error)
	CreateUser(ctx context.Context, user *models.User) error
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
}

// AccountService registers users and checks their passwords.
type AccountService struct {
	store AccountStore
	cost  int // bcrypt cost
}

func NewAccountService(store AccountStore) *AccountService {
	return &AccountService{store: store, cost: bcrypt.DefaultCost}
}

// Register creates a user with the password. It returns
// models.ErrUsernameTaken when the username is in use.
func (s *AccountService) Register(ctx context.Context, username, password string) (*models.User, error) {
	// Checked first to skip hashing; CreateUser still catches a race.
	exists, err := s.store.UserExists(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("check username: %w", err)
	}
	if exists {
		return nil, models.ErrUsernameTaken
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.cost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	user := &models.User{Username: username, PasswordHash: string(hash)}
	if err := s.store.CreateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// Login returns the user when the password is theirs, ErrInvalidCredentials
// (wrapping the cause) when the username or password is wrong, and any
// other error when the check itself failed.
func (s *AccountService) Login(ctx context.Context, username, password string) (*models.User, error) {
	user, err := s.store.GetUserByUsername(ctx, username)
	if errors.Is(err, models.ErrUserNotFound) {
		// Hash anyway, so an unknown username takes as long to refuse as a
		// wrong password and doesn't reveal which usernames exist.
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash(), []byte(password))
		return nil, fmt.Errorf("%w: %w", ErrInvalidCredentials, err)
	}
	if err != nil {
		return nil, fmt.Errorf("look up user: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidCredentials, err)
	}
	return user, nil
}

// ChangePassword is the package's ChangePassword with this service's store.
func (s *AccountService) ChangePassword(ctx context.Context, userID uuid.UUID, current, next, confirm string) error {
	return ChangePassword(ctx, s.store, userID, current, next, confirm)
}

// dummyPasswordHash is checked against when the username is unknown; it is
// made once, on first use.
var dummyPasswordHash = sync.OnceValue(func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("not a real password"), bcrypt.DefaultCost)
	if err != nil {
		panic(fmt.Sprintf("dummy password hash: %v", err))
	}
	return hash
})
