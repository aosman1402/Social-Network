package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Login with GitHub (OAuth 2.0 authorization-code flow), built on the
// standard library only - no OAuth client package.
//
//  1. GET /api/auth/github/login - sets a random `state` cookie and
//     redirects the browser to GitHub's consent page.
//  2. GitHub redirects back to GET /api/auth/github/callback with a
//     `code` and the same `state`. The state is checked against the cookie
//     (so a login can't be forged from another site), the code is swapped
//     for an access token, and the GitHub profile + verified emails are read.
//  3. The GitHub account is then matched to a local user:
//     - already linked -> logged in;
//     - a local account with the same (GitHub-verified) email -> linked
//       and logged in;
//     - otherwise -> a short-lived pending sign-up, and the browser is sent
//       to the frontend's /register/github page to add the fields GitHub
//       doesn't provide (date of birth is required), which then calls
//       POST /api/auth/oauth/complete.
//
// Logging in creates exactly the same session as password login
// (startSession).

const (
	githubProvider       = "github"
	oauthStateCookie     = "oauth_state"
	oauthStateTTL        = 10 * time.Minute
	oauthPendingTTL      = 15 * time.Minute
	oauthHTTPTimeout     = 10 * time.Second
	defaultBackendURL    = "http://localhost:8081"
	defaultFrontendURL   = "http://localhost:3000"
	githubAuthorizeURL   = "https://github.com/login/oauth/authorize"
	githubTokenURL       = "https://github.com/login/oauth/access_token"
	githubAPIURL         = "https://api.github.com"
	githubAvatarURLStart = "https://avatars.githubusercontent.com/"
)

// GitHubOAuth holds the GitHub OAuth app settings. The three GitHub URLs
// are fields (rather than constants) only so tests can point them at a
// fake GitHub server.
type GitHubOAuth struct {
	DB           *sql.DB
	ClientID     string
	ClientSecret string
	BackendURL   string // where GitHub sends the browser back to (this server)
	FrontendURL  string // where the browser goes after logging in
	AuthorizeURL string
	TokenURL     string
	APIURL       string
	HTTPClient   *http.Client
}

// NewGitHubOAuthFromEnv reads GITHUB_CLIENT_ID / GITHUB_CLIENT_SECRET (and
// optionally BACKEND_URL / FRONTEND_URL). Without the two GitHub values the
// feature is simply off: the login page shows the button as unavailable.
func NewGitHubOAuthFromEnv(db *sql.DB) *GitHubOAuth {
	return &GitHubOAuth{
		DB:           db,
		ClientID:     strings.TrimSpace(os.Getenv("GITHUB_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(os.Getenv("GITHUB_CLIENT_SECRET")),
		BackendURL:   envOr("BACKEND_URL", defaultBackendURL),
		FrontendURL:  envOr("FRONTEND_URL", defaultFrontendURL),
		AuthorizeURL: githubAuthorizeURL,
		TokenURL:     githubTokenURL,
		APIURL:       githubAPIURL,
		HTTPClient:   &http.Client{Timeout: oauthHTTPTimeout},
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimRight(strings.TrimSpace(os.Getenv(key)), "/"); v != "" {
		return v
	}
	return fallback
}

// Enabled reports whether GitHub login is configured.
func (g *GitHubOAuth) Enabled() bool {
	return g.ClientID != "" && g.ClientSecret != ""
}

func (g *GitHubOAuth) callbackURL() string {
	return g.BackendURL + "/api/auth/github/callback"
}

// redirectToFrontend sends the browser to a frontend path.
func (g *GitHubOAuth) redirectToFrontend(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, g.FrontendURL+path, http.StatusFound)
}

// fail ends the flow on the login page with a short error code the page
// turns into a message.
func (g *GitHubOAuth) fail(w http.ResponseWriter, r *http.Request, code string) {
	g.redirectToFrontend(w, r, "/login?oauth_error="+url.QueryEscape(code))
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Providers - GET /api/auth/providers: which external logins are available.
func (g *GitHubOAuth) Providers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"github": g.Enabled()})
}

// Login - GET /api/auth/github/login: start the flow.
func (g *GitHubOAuth) Login(w http.ResponseWriter, r *http.Request) {
	if !g.Enabled() {
		g.fail(w, r, "not_configured")
		return
	}
	state, err := randomToken()
	if err != nil {
		g.fail(w, r, "server_error")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/api/auth/github",
		MaxAge:   int(oauthStateTTL.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		// Lax, not Strict: the callback is a top-level redirect *from
		// github.com*, and Strict cookies aren't sent on cross-site
		// navigations.
		SameSite: http.SameSiteLaxMode,
	})

	q := url.Values{}
	q.Set("client_id", g.ClientID)
	q.Set("redirect_uri", g.callbackURL())
	q.Set("scope", "read:user user:email")
	q.Set("state", state)
	q.Set("allow_signup", "true")
	http.Redirect(w, r, g.AuthorizeURL+"?"+q.Encode(), http.StatusFound)
}

// githubProfile is what we use from the GitHub API.
type githubProfile struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Email     string // primary verified email, from /user/emails
}

// Callback - GET /api/auth/github/callback: GitHub sends the browser here.
func (g *GitHubOAuth) Callback(w http.ResponseWriter, r *http.Request) {
	if !g.Enabled() {
		g.fail(w, r, "not_configured")
		return
	}

	// The state cookie is single-use - clear it whatever happens next.
	cookie, cookieErr := r.Cookie(oauthStateCookie)
	http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Value: "", Path: "/api/auth/github", MaxAge: -1, HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode})

	q := r.URL.Query()
	if q.Get("error") != "" { // e.g. the user clicked "Cancel" on GitHub
		g.fail(w, r, "denied")
		return
	}
	state := q.Get("state")
	if cookieErr != nil || state == "" ||
		subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(state)) != 1 {
		g.fail(w, r, "invalid_state")
		return
	}
	code := q.Get("code")
	if code == "" {
		g.fail(w, r, "github_error")
		return
	}

	token, err := g.exchangeCode(code)
	if err != nil {
		log.Printf("GitHub OAuth: code exchange failed: %v", err)
		g.fail(w, r, "github_error")
		return
	}
	profile, err := g.fetchProfile(token)
	if err != nil {
		log.Printf("GitHub OAuth: reading profile failed: %v", err)
		g.fail(w, r, "github_error")
		return
	}
	if profile.Email == "" {
		g.fail(w, r, "no_verified_email")
		return
	}
	providerUserID := strconv.FormatInt(profile.ID, 10)

	// 1. Already linked.
	var userID int64
	err = g.DB.QueryRow(
		"SELECT user_id FROM oauth_accounts WHERE provider = ? AND provider_user_id = ?",
		githubProvider, providerUserID,
	).Scan(&userID)
	if err == nil {
		g.logIn(w, r, userID)
		return
	}
	if err != sql.ErrNoRows {
		g.fail(w, r, "server_error")
		return
	}

	// 2. A local account with the same email - GitHub has verified the
	// user owns that address, so link it.
	err = g.DB.QueryRow("SELECT id FROM users WHERE email = ? COLLATE NOCASE", profile.Email).Scan(&userID)
	if err == nil {
		if _, err := g.DB.Exec(
			"INSERT OR IGNORE INTO oauth_accounts (user_id, provider, provider_user_id) VALUES (?, ?, ?)",
			userID, githubProvider, providerUserID,
		); err != nil {
			g.fail(w, r, "server_error")
			return
		}
		g.logIn(w, r, userID)
		return
	}
	if err != sql.ErrNoRows {
		g.fail(w, r, "server_error")
		return
	}

	// 3. New user - finish signing up on the frontend.
	pendingToken, err := g.createPendingSignup(profile, providerUserID)
	if err != nil {
		log.Printf("GitHub OAuth: saving pending sign-up failed: %v", err)
		g.fail(w, r, "server_error")
		return
	}
	g.redirectToFrontend(w, r, "/register/github?token="+url.QueryEscape(pendingToken))
}

func (g *GitHubOAuth) logIn(w http.ResponseWriter, r *http.Request, userID int64) {
	if err := startSession(g.DB, w, r, userID); err != nil {
		g.fail(w, r, "server_error")
		return
	}
	g.redirectToFrontend(w, r, "/")
}

// exchangeCode swaps the one-time code for an access token.
func (g *GitHubOAuth) exchangeCode(code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", g.callbackURL())

	req, err := http.NewRequest(http.MethodPost, g.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var body struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("token response: %w", err)
	}
	if body.AccessToken == "" {
		return "", fmt.Errorf("no access token (status %d): %s %s", resp.StatusCode, body.Error, body.Description)
	}
	return body.AccessToken, nil
}

func (g *GitHubOAuth) githubGet(token, path string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, g.APIURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "social-network-oauth") // GitHub requires one

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// fetchProfile reads the GitHub user and their primary verified email
// (falling back to any verified one). Unverified emails are never used.
func (g *GitHubOAuth) fetchProfile(token string) (*githubProfile, error) {
	var p githubProfile
	if err := g.githubGet(token, "/user", &p); err != nil {
		return nil, err
	}
	if p.ID == 0 {
		return nil, fmt.Errorf("GitHub user has no id")
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := g.githubGet(token, "/user/emails", &emails); err != nil {
		return nil, err
	}
	for _, e := range emails {
		if e.Verified && e.Primary {
			p.Email = e.Email
			break
		}
	}
	if p.Email == "" {
		for _, e := range emails {
			if e.Verified {
				p.Email = e.Email
				break
			}
		}
	}
	return &p, nil
}

// splitName turns GitHub's single "name" field into first / last name,
// falling back to the username.
func splitName(name, login string) (string, string) {
	parts := strings.Fields(name)
	switch len(parts) {
	case 0:
		return login, ""
	case 1:
		return parts[0], ""
	default:
		return parts[0], strings.Join(parts[1:], " ")
	}
}

func (g *GitHubOAuth) createPendingSignup(p *githubProfile, providerUserID string) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	// Housekeeping: drop sign-ups that were never finished.
	g.DB.Exec("DELETE FROM oauth_pending_signups WHERE expires_at < ?", now)

	first, last := splitName(p.Name, p.Login)
	avatar := ""
	if strings.HasPrefix(p.AvatarURL, githubAvatarURLStart) {
		avatar = p.AvatarURL
	}
	_, err = g.DB.Exec(
		`INSERT INTO oauth_pending_signups
             (token, provider, provider_user_id, email, first_name, last_name, avatar_url, username, expires_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		token, githubProvider, providerUserID, p.Email, first, last, avatar, p.Login, now.Add(oauthPendingTTL),
	)
	return token, err
}

type pendingSignup struct {
	Provider       string
	ProviderUserID string
	Email          string
	FirstName      string
	LastName       string
	AvatarURL      string
	Username       string
	ExpiresAt      time.Time
}

// loadPending returns the pending sign-up for token, or sql.ErrNoRows if
// there is none or it has expired.
func loadPending(db *sql.DB, token string) (*pendingSignup, error) {
	if token == "" {
		return nil, sql.ErrNoRows
	}
	var p pendingSignup
	var first, last, avatar, username sql.NullString
	err := db.QueryRow(
		`SELECT provider, provider_user_id, email, first_name, last_name, avatar_url, username, expires_at
         FROM oauth_pending_signups WHERE token = ?`, token,
	).Scan(&p.Provider, &p.ProviderUserID, &p.Email, &first, &last, &avatar, &username, &p.ExpiresAt)
	if err != nil {
		return nil, err
	}
	if time.Now().After(p.ExpiresAt) {
		db.Exec("DELETE FROM oauth_pending_signups WHERE token = ?", token)
		return nil, sql.ErrNoRows
	}
	p.FirstName, p.LastName, p.AvatarURL, p.Username = first.String, last.String, avatar.String, username.String
	return &p, nil
}

// Pending - GET /api/auth/oauth/pending?token=...: what GitHub told us, to
// pre-fill the "finish signing up" form.
func (g *GitHubOAuth) Pending(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	p, err := loadPending(g.DB, r.URL.Query().Get("token"))
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "This sign-up link has expired - please continue with GitHub again")
		return
	}
	json.NewEncoder(w).Encode(map[string]string{
		"provider":   p.Provider,
		"email":      p.Email,
		"first_name": p.FirstName,
		"last_name":  p.LastName,
		"avatar_url": p.AvatarURL,
		"username":   p.Username,
	})
}

// Complete - POST /api/auth/oauth/complete: create the account from a
// pending sign-up plus the fields GitHub doesn't provide, then log in.
func (g *GitHubOAuth) Complete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Token           string  `json:"token"`
		FirstName       string  `json:"first_name"`
		LastName        string  `json:"last_name"`
		DateOfBirth     string  `json:"date_of_birth"`
		Nickname        *string `json:"nickname"`
		AboutMe         *string `json:"about_me"`
		UseGitHubAvatar bool    `json:"use_github_avatar"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	p, err := loadPending(g.DB, req.Token)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "This sign-up link has expired - please continue with GitHub again")
		return
	}

	// Same rules as the normal registration form.
	req.FirstName, req.LastName = strings.TrimSpace(req.FirstName), strings.TrimSpace(req.LastName)
	if req.FirstName == "" || req.LastName == "" || req.DateOfBirth == "" {
		writeJSONError(w, http.StatusBadRequest, "First name, last name and date of birth are required")
		return
	}
	dob, err := time.Parse("2006-01-02", req.DateOfBirth)
	if err != nil || dob.After(time.Now()) {
		writeJSONError(w, http.StatusBadRequest, "Invalid date of birth")
		return
	}
	if req.Nickname != nil {
		trimmed := strings.TrimSpace(*req.Nickname)
		req.Nickname = &trimmed
		if trimmed == "" {
			req.Nickname = nil
		}
	}
	ah := &AuthHandler{DB: g.DB}
	if req.Nickname != nil {
		if !isValidNicknameFormat(*req.Nickname) {
			writeJSONError(w, http.StatusBadRequest, "Nickname must be 3-14 characters using only letters and numbers")
			return
		}
		if taken, err := ah.nicknameTaken(*req.Nickname); err != nil || taken {
			writeJSONError(w, http.StatusConflict, "Nickname is already taken")
			return
		}
	}
	if req.AboutMe != nil && strings.TrimSpace(*req.AboutMe) == "" {
		req.AboutMe = nil
	}
	p.Email = normalizeEmail(p.Email)
	if taken, err := ah.emailTaken(p.Email); err != nil || taken {
		writeJSONError(w, http.StatusConflict, "An account with this email already exists - log in with your password")
		return
	}

	// The account has no password of its own: it's a random one nobody
	// knows, so it can only be signed into through GitHub.
	secret, err := randomToken()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create account")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create account")
		return
	}
	var avatar *string
	if req.UseGitHubAvatar && p.AvatarURL != "" {
		avatar = &p.AvatarURL
	}

	tx, err := g.DB.Begin()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create account")
		return
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		`INSERT INTO users (email, password_hash, first_name, last_name, date_of_birth, avatar_path, nickname, about_me)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Email, hash, req.FirstName, req.LastName, dob, avatar, req.Nickname, req.AboutMe,
	)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "users.email"):
			writeJSONError(w, http.StatusConflict, "An account with this email already exists - log in with your password")
		case strings.Contains(msg, "users.nickname"):
			writeJSONError(w, http.StatusConflict, "Nickname is already taken")
		default:
			writeJSONError(w, http.StatusInternalServerError, "Failed to create account")
		}
		return
	}
	userID, _ := result.LastInsertId()
	if _, err := tx.Exec(
		"INSERT INTO oauth_accounts (user_id, provider, provider_user_id) VALUES (?, ?, ?)",
		userID, p.Provider, p.ProviderUserID,
	); err != nil {
		writeJSONError(w, http.StatusConflict, "This GitHub account is already linked to another user")
		return
	}
	if _, err := tx.Exec("DELETE FROM oauth_pending_signups WHERE token = ?", req.Token); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create account")
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create account")
		return
	}

	if err := startSession(g.DB, w, r, userID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Account created, but logging in failed - continue with GitHub again")
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Account created",
		"user_id": userID,
	})
}
