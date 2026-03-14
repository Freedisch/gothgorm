package gothgorm

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"gorm.io/gorm"
)

type Auth struct {
	db     *gorm.DB
	config Config
}

func New(cfg Config) (*Auth, error) {
	if cfg.DB == nil {
		return nil, fmt.Errorf("gothgorm: DB is required")
	}

	if cfg.SessionSecret == "" {
		return nil, fmt.Errorf("gothgorm: SessionSecret is required")

	}
	if len(cfg.Providers) == 0 {
		return nil, fmt.Errorf("gothgorm: at least one provider is required")
	}
	if cfg.TokenPrefix == "" {
		cfg.TokenPrefix = "gt_"
	}

	if err := cfg.DB.AutoMigrate(&User{}); err != nil {
		return nil, fmt.Errorf("gothgorm: auto migrate: %w", err)
	}

	store := sessions.NewCookieStore([]byte(cfg.SessionSecret))
	store.MaxAge(300)
	store.Options.HttpOnly = true
	store.Options.Secure = true
	gothic.Store = store

	goth.UseProviders(cfg.Providers...)
	return &Auth{db: cfg.DB, config: cfg}, nil
}

func (a *Auth) findOrCreate(r *http.Request, gothUser goth.User) (*User, bool, error) {
	var user User
	var isNew bool

	result := a.db.WithContext(r.Context()).Where("oauth_provider = ? AND oauth_id = ?", gothUser.Provider, gothUser.UserID).First(&user)
	if result.Error == gorm.ErrRecordNotFound {
		user = User{
			Email:         gothUser.Email,
			OAuthProvider: gothUser.Provider,
			OAuthID:       gothUser.UserID,
			DisplayName:   gothUser.Name,
			AvatarURL:     gothUser.AvatarURL,
		}
		if err := a.db.WithContext(r.Context()).Create(&user).Error; err != nil {
			return nil, false, fmt.Errorf("create user: %w", err)
		}
		isNew = true
	} else if result.Error != nil {
		return nil, false, fmt.Errorf("find user: %w", result.Error)

	} else {
		a.db.WithContext(r.Context()).Model(&user).Updates(map[string]any{
			"display_name": gothUser.Name,
			"avatar_url":   gothUser.AvatarURL,
			"last_seen_at": time.Now(),
		})
	}
	return &user, isNew, nil

}

func (a *Auth) issueToken(r *http.Request, user *User) (string, error) {
	raw, hash, prefix, err := generateToken(a.config.TokenPrefix)
	if err != nil {
		return "", err
	}

	if err := a.db.WithContext(r.Context()).Model(user).Updates(map[string]any{
		"token_hash":   hash,
		"token_prefix": prefix,
	}).Error; err != nil {
		return "", fmt.Errorf("save token: %w", err)
	}
	user.TokenHash = hash
	user.TokenPrefix = prefix
	return raw, nil
}
