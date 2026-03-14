package gothgorm

import (
	"context"
	"net/http"
	"strings"

	"gorm.io/gorm"
)

type contextKey string
const ContextKeyUser contextKey = "gothgorm_user"

func (a * Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer")
		if raw == ""{
			http.Error(w, `{"error":"no token provided","code":"missing_token"}`, http.StatusUnauthorized)
            return
		}

		hash := hashToken(raw)
		var user User
		result := a.db.WithContext(r.Context()).Where("token_hash = ?", hash).First(&user)

		if result.Error != nil {
            http.Error(w, `{"error":"invalid token","code":"invalid_token"}`, http.StatusUnauthorized)
            return
        }

		go a.db.WithContext(context.Background()).Model(&user).Update("last_seen_at", gorm.Expr("now()"))
		ctx := context.WithValue(r.Context(), ContextKeyUser, &user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserFromContext retrieves the authenticated user from the request context.
// Returns nil if no user is present (unauthenticated request).
func UserFromContext(r *http.Request) *User{
	u, _ := r.Context().Value(ContextKeyUser).(*User)
	return u
}