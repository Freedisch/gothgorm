package gothgorm

import (
	"net/http"

	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/google"
	"gorm.io/gorm"
)

// Provider is an alias for goth.Provider so callers don't need to import goth directly.
type Provider = goth.Provider

type Config struct {
	DB               *gorm.DB
	SessionSecret    string
	Providers        []goth.Provider
	OnLogin          func(user *User, isNew bool, rawToken string)
	TokenPrefix      string
	ResponseHanlders func(w http.ResponseWriter, r *http.Request, user *User, rawToken string)
}

func Google(clientID, clientSecret, callbackURL string, scopes ...string) goth.Provider {
	if len(scopes) == 0 {
		scopes = []string{"user:email"}
	}
	return google.New(clientID, clientSecret, callbackURL, scopes...)
}

func Github(clientID, clientSecret, callbackURL string, scopes ...string) goth.Provider {
	if len(scopes) == 0 {
		scopes = []string{"user:email"}

	}
	return github.New(clientID, clientSecret, callbackURL, scopes...)
}
