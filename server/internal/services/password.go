package services

import (
	"context"
	"errors"
	"fmt"

	"expenser/internal/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ErrWrongPassword means the current password given was not the user's.
var ErrWrongPassword = errors.New("the current password is wrong")

// PasswordStore is what changing a password needs from the database.
type PasswordStore interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	UpdateUserPassword(ctx context.Context, id uuid.UUID, hash string) error
}

// ChangePassword replaces a user's password after checking the new one and
// the current one. Logins on other devices end (see UpdateUserPassword).
func ChangePassword(ctx context.Context, store PasswordStore, userID uuid.UUID, current, next, confirm string) error {
	if err := models.ValidateNewPassword(current, next, confirm); err != nil {
		return err
	}
	user, err := store.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("change password: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)); err != nil {
		return ErrWrongPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := store.UpdateUserPassword(ctx, userID, string(hash)); err != nil {
		return fmt.Errorf("change password: %w", err)
	}
	return nil
}
