package gothgorm

import (
	"encoding/json"
	"net/http"

	"github.com/markbates/goth/gothic"
)

func (a *Auth) Begin(w http.ResponseWriter, r *http.Request) {
	gothic.BeginAuthHandler(w, r)
}

func (a *Auth) CallBack(w http.ResponseWriter, r *http.Request) {

	gothUser, err := gothic.CompleteUserAuth(w, r)
	if err != nil {
		http.Error(w, `{"error": "oauth failed", "code":"oauth_error"}`, http.StatusUnauthorized)
		return

	}

	user, isNew, err := a.findOrCreate(r, gothUser)
	if err != nil {
		http.Error(w, `{"error":"auth failed","code":"internal_error"}`, http.StatusInternalServerError)
		return
	}

	rawToken, err := a.issueToken(r, user)
	if err != nil {
		http.Error(w, `{"error":"token generation failed","code":"internal_error"}`, http.StatusInternalServerError)
		return
	}

	if a.config.OnLogin != nil {
		a.config.OnLogin(user, isNew, rawToken)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"user_id":      user.ID,
		"email":        user.Email,
		"display_name": user.DisplayName,
		"avatar_url":   user.AvatarURL,
		"is_new":       isNew,
		"token":        rawToken,
		"token_prefix": user.TokenPrefix,
	})

}
