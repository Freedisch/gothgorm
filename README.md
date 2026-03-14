# gothgorm

**OAuth authentication for Go — powered by [Goth](https://github.com/markbates/goth) and [GORM](https://gorm.io).**

gothgorm wraps the OAuth dance, user persistence, API token generation, and Bearer token middleware into a single package. Drop it into any Go project and have working authentication in minutes.

---

## Features

- Google and GitHub OAuth out of the box (more providers via Goth)
- Automatic user creation and login via GORM — no raw SQL
- Secure API token generation (SHA-256 hashed, never stored in plaintext)
- Bearer token middleware for protecting routes
- `OnLogin` hook for custom post-auth logic
- Custom response handler support
- Works with any Chi-compatible router

---

## Installation

```bash
go get github.com/freedisch/gothgorm
```

**Dependencies installed automatically:**

| Package                       | Purpose                       |
| ----------------------------- | ----------------------------- |
| `gorm.io/gorm`                | ORM for user persistence      |
| `gorm.io/driver/postgres`     | PostgreSQL driver             |
| `github.com/markbates/goth`   | OAuth provider abstraction    |
| `github.com/gorilla/sessions` | Session store for OAuth state |

---

## Quickstart

```go
package main

import (
    "log"
    "net/http"
    "os"

    "github.com/go-chi/chi/v5"
    "github.com/freedisch/gothgorm"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

func main() {
    // 1. Connect to your database
    db, err := gorm.Open(postgres.Open(os.Getenv("DATABASE_URL")), &gorm.Config{})
    if err != nil {
        log.Fatal(err)
    }

    // 2. Set up gothgorm
    auth, err := gothgorm.New(gothgorm.Config{
        DB:            db,
        SessionSecret: os.Getenv("SESSION_SECRET"),
        Providers: []gothgorm.Provider{
            gothgorm.Google(
                os.Getenv("GOOGLE_CLIENT_ID"),
                os.Getenv("GOOGLE_CLIENT_SECRET"),
                "http://localhost:8080/auth/google/callback",
            ),
            gothgorm.GitHub(
                os.Getenv("GITHUB_CLIENT_ID"),
                os.Getenv("GITHUB_CLIENT_SECRET"),
                "http://localhost:8080/auth/github/callback",
            ),
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // 3. Register routes
    r := chi.NewRouter()

    r.Get("/auth/{provider}",          auth.Begin)
    r.Get("/auth/{provider}/callback", auth.Callback)

    r.Group(func(r chi.Router) {
        r.Use(auth.Middleware)

        r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
            user := gothgorm.UserFromContext(r)
            w.Write([]byte("Hello, " + user.DisplayName))
        })
    })

    http.ListenAndServe(":8080", r)
}
```

That's everything. gothgorm handles the OAuth redirect, the callback exchange, user creation, token generation, and Bearer token validation automatically.

---

## How It Works

### Authentication Flow

```
User clicks "Sign in with Google / GitHub"
        |
        v
GET /auth/{provider}          → gothgorm.Begin
        |                        Redirects to provider consent screen
        v
User approves access
        |
        v
GET /auth/{provider}/callback → gothgorm.Callback
        |                        Exchanges code for profile
        |                        FindOrCreate user in database
        |                        Generates API token
        |                        Returns JSON response
        v
{ "token": "gt_abc123...", "user_id": "...", "email": "..." }
        |
        v
Client sends token on every request:
Authorization: Bearer gt_abc123...
        |
        v
auth.Middleware validates token → attaches user to context
```

### Token Security

gothgorm never stores raw tokens. When a token is generated:

1. A 32-byte cryptographically random value is created
2. The raw token is returned to the client once — this is the only time it appears in plaintext
3. A SHA-256 hash of the token is stored in the database
4. On every request, the incoming token is hashed and the hash is looked up — the raw token never touches your database

This means a database breach exposes hashed tokens, not usable credentials.

---

## Configuration

```go
gothgorm.Config{
    // Required
    DB            *gorm.DB        // your GORM database connection
    SessionSecret string          // random secret for OAuth session cookies (min 32 chars)
    Providers     []goth.Provider // at least one provider required

    // Optional
    TokenPrefix     string         // prefix for generated tokens (default: "gt_")
    OnLogin         func(user *gothgorm.User, isNew bool, rawToken string)
    ResponseHandler func(w http.ResponseWriter, r *http.Request, user *gothgorm.User, rawToken string)
}
```

### TokenPrefix

By default tokens look like `gt_3f2a1b...`. Override the prefix to match your product:

```go
gothgorm.Config{
    TokenPrefix: "memo_",
    // tokens will look like: memo_3f2a1b...
}
```

### OnLogin Hook

Run custom logic after every successful login without touching the auth flow:

```go
gothgorm.Config{
    OnLogin: func(user *gothgorm.User, isNew bool, rawToken string) {
        if isNew {
            // Send welcome email, provision resources, emit analytics event
            sendWelcomeEmail(user.Email)
        }
        log.Printf("login: %s via %s (new=%v)", user.Email, user.OAuthProvider, isNew)
    },
}
```

### ResponseHandler

Override the default JSON response to control exactly what gets sent back to the client:

```go
gothgorm.Config{
    ResponseHandler: func(w http.ResponseWriter, r *http.Request, user *gothgorm.User, rawToken string) {
        // Redirect to a frontend URL with the token as a query param
        http.Redirect(w, r, "https://app.example.com/auth?token="+rawToken, http.StatusFound)
    },
}
```

If `ResponseHandler` is not set, the default JSON response is:

```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "user@example.com",
  "display_name": "Jane Smith",
  "avatar_url": "https://...",
  "is_new": true,
  "token": "gt_3f2a1b...",
  "token_prefix": "gt_3f2a1b"
}
```

---

## The User Model

gothgorm automatically creates and migrates the `users` table via GORM's `AutoMigrate`. The model has these fields:

```go
type User struct {
    ID            string     // UUID, primary key
    Email         string     // unique
    OAuthProvider string     // "google" | "github"
    OAuthID       string     // provider's stable user ID
    DisplayName   string     // full name from provider
    AvatarURL     string     // profile picture URL
    TokenHash     string     // SHA-256 hash of current API token
    TokenPrefix   string     // first ~13 chars, safe to display in UI
    CreatedAt     time.Time
    UpdatedAt     time.Time
    LastSeenAt    *time.Time // updated on every authenticated request
}
```

### Extending the User Model

If you need additional fields on your users, embed `gothgorm.User` in your own struct and run `AutoMigrate` on it:

```go
type AppUser struct {
    gothgorm.User
    Plan        string
    CompanyName string
    OnboardedAt *time.Time
}

// Tell GORM to use your extended struct
db.AutoMigrate(&AppUser{})
```

---

## Providers

### Built-in Helpers

```go
// Google — default scopes: email, profile
gothgorm.Google(clientID, clientSecret, callbackURL)

// Google — custom scopes
gothgorm.Google(clientID, clientSecret, callbackURL, "email", "profile", "openid")

// GitHub — default scope: user:email
gothgorm.GitHub(clientID, clientSecret, callbackURL)

// GitHub — custom scopes
gothgorm.GitHub(clientID, clientSecret, callbackURL, "user:email", "read:org")
```

### Other Goth Providers

gothgorm accepts any `goth.Provider` directly. This means you can use any of Goth's 70+ providers:

```go
import "github.com/markbates/goth/providers/twitter"
import "github.com/markbates/goth/providers/discord"

gothgorm.Config{
    Providers: []goth.Provider{
        twitter.New(clientID, clientSecret, callbackURL),
        discord.New(clientID, clientSecret, callbackURL),
    },
}
```

---

## Middleware

### Protecting Routes

```go
r.Group(func(r chi.Router) {
    r.Use(auth.Middleware)

    r.Get("/dashboard", dashboardHandler)
    r.Post("/api/data",  dataHandler)
})
```

### Reading the User in a Handler

```go
func myHandler(w http.ResponseWriter, r *http.Request) {
    user := gothgorm.UserFromContext(r)
    if user == nil {
        // Should not happen on a protected route — middleware would have blocked it
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    fmt.Fprintf(w, "Hello %s, your provider is %s", user.DisplayName, user.OAuthProvider)
}
```

---

## Environment Variables

| Variable               | Description                                                     |
| ---------------------- | --------------------------------------------------------------- |
| `DATABASE_URL`         | PostgreSQL connection string                                    |
| `SESSION_SECRET`       | Random secret for OAuth session cookies — minimum 32 characters |
| `GOOGLE_CLIENT_ID`     | From Google Cloud Console                                       |
| `GOOGLE_CLIENT_SECRET` | From Google Cloud Console                                       |
| `GITHUB_CLIENT_ID`     | From GitHub OAuth Apps settings                                 |
| `GITHUB_CLIENT_SECRET` | From GitHub OAuth Apps settings                                 |

### Generating a Session Secret

```bash
openssl rand -hex 32
```

---

## Setting Up OAuth Credentials

### Google

1. Go to [Google Cloud Console](https://console.cloud.google.com)
2. Create a project → APIs & Services → Credentials
3. Create OAuth 2.0 Client ID (Web application)
4. Add your callback URL to Authorized redirect URIs:
   ```
   http://localhost:8080/auth/google/callback   ← development
   https://api.yourdomain.com/auth/google/callback  ← production
   ```

### GitHub

1. Go to GitHub → Settings → Developer settings → OAuth Apps
2. New OAuth App
3. Set Authorization callback URL:
   ```
   http://localhost:8080/auth/github/callback   ← development
   https://api.yourdomain.com/auth/github/callback  ← production
   ```

---

## Full Example with Chi

```go
package main

import (
    "encoding/json"
    "log"
    "net/http"
    "os"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/freedisch/gothgorm"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

func main() {
    db, _ := gorm.Open(postgres.Open(os.Getenv("DATABASE_URL")), &gorm.Config{})

    auth, err := gothgorm.New(gothgorm.Config{
        DB:            db,
        SessionSecret: os.Getenv("SESSION_SECRET"),
        TokenPrefix:   "myapp_",
        Providers: []gothgorm.Provider{
            gothgorm.Google(
                os.Getenv("GOOGLE_CLIENT_ID"),
                os.Getenv("GOOGLE_CLIENT_SECRET"),
                os.Getenv("APP_URL")+"/auth/google/callback",
            ),
            gothgorm.GitHub(
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
    r.Get("/auth/{provider}/callback", auth.Callback)

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
```

---

## Security Notes

- `SESSION_SECRET` must be at least 32 characters. Use `openssl rand -hex 32` to generate one.
- Session cookies are set with `HttpOnly: true` and `Secure: true`. In local development over HTTP, set `Secure: false` on the cookie store.
- Tokens are regenerated on every login. There is no way to recover a lost token — the user must log in again.
- `LastSeenAt` is updated on every authenticated request via a non-blocking background goroutine so it never adds latency to your responses.
- gothgorm makes no external network calls except through the Goth provider during the OAuth exchange.

---

## License

MIT
