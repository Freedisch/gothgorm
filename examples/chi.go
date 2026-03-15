package examples

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/freedisch/gothgorm"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main(){
	db, _ := gorm.Open(postgres.Open(os.Getenv("DATABASE_URL")), &gorm.Config{})
	auth, err := gothgorm.New(gothgorm.Config{
		DB: db,
		SessionSecret: os.Getenv("SESSION_SECRET"),
        TokenPrefix:   "myapp_",
        Providers: []gothgorm.Provider{
            gothgorm.Google(
                os.Getenv("GOOGLE_CLIENT_ID"),
                os.Getenv("GOOGLE_CLIENT_SECRET"),
                os.Getenv("APP_URL")+"/auth/google/callback",
            ),
            gothgorm.Github(
                os.Getenv("GITHUB_CLIENT_ID"),
                os.Getenv("GITHUB_CLIENT_SECRET"),
                os.Getenv("APP_URL")+"/auth/github/callback",
            ),
        },
        OnLogin: func(user *gothgorm.User, isNew bool, _ string) {
            if isNew {
                log.Printf("new user: %s", user.Email)
            }
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    // Public
    r.Get("/auth/{provider}",          auth.Begin)
    r.Get("/auth/{provider}/callback", auth.CallBack)

    // Protected
    r.Group(func(r chi.Router) {
        r.Use(auth.Middleware)

        r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
            user := gothgorm.UserFromContext(r)
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(user)
        })
    })

    log.Println("listening on :8080")
    http.ListenAndServe(":8080", r)

}