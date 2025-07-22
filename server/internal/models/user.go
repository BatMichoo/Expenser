package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system
type User struct {
	ID           uuid.UUID
	Username     string `form:"username" binding:"required,min=3,max=50"`
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UserRegistration represents the data needed for user registration
type UserRegistration struct {
	Username        string `form:"username" binding:"required,min=3,max=50"`
	Password        string `form:"password" binding:"required,min=6"`
	ConfirmPassword string `form:"confirm_password" binding:"required,eqfield=Password"`
}

// UserLogin represents the data needed for user login
type UserLogin struct {
	Username string `form:"username" binding:"required"`
	Password string `form:"password" binding:"required"`
}
