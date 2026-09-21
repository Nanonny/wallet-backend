package domain

import "time"

// User is an authenticated wallet console user.
// PasswordHash is a bcrypt digest and must never be serialized to clients.
type User struct {
	ID           string     `json:"id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}
