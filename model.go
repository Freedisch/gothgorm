package gothgorm

import "time"

// User is the GORM model managed by this library.
// Developers can embed it in their own user struct if they need
// to add custom fields.
type User struct {
	ID            string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Email         string `gorm:"uniqueIndex;not null"`
	OAuthProvider string `gorm:"not null;index"`
	OAuthID       string `gorm:"not null"`
	DisplayName   string
	AvatarURL     string
	TokenHash     string `gorm:"uniqueIndex"`
	TokenPrefix   string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
