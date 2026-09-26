package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type fakeGitHubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// newFakeGitHub stands in for github.com + api.github.com: it accepts the
// code "good-code" and serves the given profile and email list.
func newFakeGitHub(t *testing.T, profile map[string]interface{}, emails []fakeGitHubEmail) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost || r.Form.Get("client_id") != "cid" ||
			r.Form.Get("client_secret") != "secret" || r.Form.Get("code") != "good-code" {
			json.NewEncoder(w).Encode(map[string]string{"error": "bad_verification_code"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"access_token": "gho_test", "token_type": "bearer"})
	})
	authed := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer gho_test" || r.Header.Get("User-Agent") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next(w, r)
		}
	}
	mux.HandleFunc("/user", authed(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(profile)
	}))
	mux.HandleFunc("/user/emails", authed(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(emails)
	}))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func newTestGitHubOAuth(t *testing.T, fake *httptest.Server) *GitHubOAuth {
	t.Helper()
	return &GitHubOAuth{
		DB:           newTestDB(t),
		ClientID:     "cid",
		ClientSecret: "secret",
		BackendURL:   "http://backend.test",
		FrontendURL:  "http://frontend.test",
		AuthorizeURL: fake.URL + "/login/oauth/authorize",
		TokenURL:     fake.URL + "/login/oauth/access_token",
		APIURL:       fake.URL,
		HTTPClient:   fake.Client(),
	}
}

var octocat = map[string]interface{}{
	"id": 583231, "login": "octocat", "name": "Mona Lisa Octocat",
	"avatar_url": "https://avatars.githubusercontent.com/u/583231?v=4",
}

// startGitHubLogin hits /api/auth/github/login and returns the state
// cookie and the GitHub URL the browser was sent to.
func startGitHubLogin(t *testing.T, g *GitHubOAuth) (*http.Cookie, *url.URL) {
	t.Helper()
	w := httptest.NewRecorder()
	g.Login(w, httptest.NewRequest(http.MethodGet, "/api/auth/github/login", nil))
	if w.Code != http.StatusFound {
		t.Fatalf("login: status = %d, want 302", w.Code)
	}
	loc, _ := url.Parse(w.Header().Get("Location"))
	for _, c := range w.Result().Cookies() {
		if c.Name == oauthStateCookie {
			return c, loc
		}
	}
	t.Fatal("login did not set the state cookie")
	return nil, nil
}

// githubCallback simulates GitHub redirecting the browser back.
func githubCallback(t *testing.T, g *GitHubOAuth, stateCookie *http.Cookie, query string) (*url.URL, *http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/github/callback?"+query, nil)
	if stateCookie != nil {
		req.AddCookie(stateCookie)
	}
	w := httptest.NewRecorder()
	g.Callback(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("callback: status = %d, want 302 (body %s)", w.Code, w.Body.String())
	}
	loc, _ := url.Parse(w.Header().Get("Location"))
	for _, c := range w.Result().Cookies() {
		if c.Name == "session_token" {
			return loc, c
		}
	}
	return loc, nil
}

// whoAmI calls /api/check-auth with the session cookie.
func whoAmI(t *testing.T, g *GitHubOAuth, session *http.Cookie) (bool, int64, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/check-auth", nil)
	req.AddCookie(session)
	w := httptest.NewRecorder()
	(&AuthHandler{DB: g.DB}).CheckAuth(w, req)
	var body struct {
		Authenticated bool   `json:"authenticated"`
		ID            int64  `json:"id"`
		Email         string `json:"email"`
	}
	json.Unmarshal(w.Body.Bytes(), &body)
	return body.Authenticated, body.ID, body.Email
}

func TestGitHubLoginRedirectsToGitHubWithState(t *testing.T) {
	fake := newFakeGitHub(t, octocat, nil)
	g := newTestGitHubOAuth(t, fake)

	state, loc := startGitHubLogin(t, g)
	if !strings.HasPrefix(loc.String(), fake.URL+"/login/oauth/authorize?") {
		t.Fatalf("redirected to %s, want GitHub's authorize page", loc)
	}
	q := loc.Query()
	if q.Get("client_id") != "cid" || q.Get("state") != state.Value || len(state.Value) < 32 ||
		q.Get("redirect_uri") != "http://backend.test/api/auth/github/callback" ||
		!strings.Contains(q.Get("scope"), "user:email") {
		t.Errorf("authorize query = %v (state cookie %q)", q, state.Value)
	}
	if !state.HttpOnly || state.SameSite != http.SameSiteLaxMode {
		t.Errorf("state cookie must be HttpOnly + SameSite=Lax: %+v", state)
	}
}

func TestGitHubCallbackRejectsForgedState(t *testing.T) {
	fake := newFakeGitHub(t, octocat, []fakeGitHubEmail{{"mona@example.com", true, true}})
	g := newTestGitHubOAuth(t, fake)
	state, _ := startGitHubLogin(t, g)

	for name, tc := range map[string]struct {
		cookie *http.Cookie
		query  string
	}{
		"wrong state":     {state, "code=good-code&state=attacker"},
		"no state cookie": {nil, "code=good-code&state=" + state.Value},
		"no state param":  {state, "code=good-code"},
	} {
		loc, session := githubCallback(t, g, tc.cookie, tc.query)
		if loc.String() != "http://frontend.test/login?oauth_error=invalid_state" || session != nil {
			t.Errorf("%s: redirected to %s (session %v), want invalid_state and no session", name, loc, session != nil)
		}
	}
}

// TestGitHubNewUserSignUpAndReturningLogin - the full path for someone
// new: GitHub -> finish sign-up (date of birth) -> logged in; and the next
// time they're logged straight in.
func TestGitHubNewUserSignUpAndReturningLogin(t *testing.T) {
	fake := newFakeGitHub(t, octocat, []fakeGitHubEmail{
		{"unverified@example.com", false, false},
		{"mona@example.com", true, true},
	})
	g := newTestGitHubOAuth(t, fake)

	state, _ := startGitHubLogin(t, g)
	loc, session := githubCallback(t, g, state, "code=good-code&state="+state.Value)
	if session != nil || loc.Host != "frontend.test" || loc.Path != "/register/github" || loc.Query().Get("token") == "" {
		t.Fatalf("new user: redirected to %s, want /register/github?token=... and no session yet", loc)
	}
	token := loc.Query().Get("token")

	// The finish-sign-up page is pre-filled from GitHub.
	w := httptest.NewRecorder()
	g.Pending(w, httptest.NewRequest(http.MethodGet, "/api/auth/oauth/pending?token="+token, nil))
	var pending map[string]string
	json.Unmarshal(w.Body.Bytes(), &pending)
	if w.Code != http.StatusOK || pending["email"] != "mona@example.com" ||
		pending["first_name"] != "Mona" || pending["last_name"] != "Lisa Octocat" || pending["username"] != "octocat" {
		t.Fatalf("pending = %d %v", w.Code, pending)
	}

	complete := func(body map[string]interface{}) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		g.Complete(w, jsonRequest(t, http.MethodPost, "/api/auth/oauth/complete", body))
		return w
	}
	if w := complete(map[string]interface{}{"token": token, "first_name": "Mona", "last_name": "Octocat"}); w.Code != http.StatusBadRequest {
		t.Errorf("missing date of birth: status = %d, want 400", w.Code)
	}
	if w := complete(map[string]interface{}{"token": token, "first_name": "Mona", "last_name": "Octocat", "date_of_birth": "2999-01-01"}); w.Code != http.StatusBadRequest {
		t.Errorf("future date of birth: status = %d, want 400", w.Code)
	}
	if w := complete(map[string]interface{}{"token": token, "first_name": "Mona", "last_name": "Octocat", "date_of_birth": "1990-05-05", "nickname": "no"}); w.Code != http.StatusBadRequest {
		t.Errorf("invalid nickname: status = %d, want 400", w.Code)
	}

	w = complete(map[string]interface{}{
		"token": token, "first_name": "Mona", "last_name": "Octocat", "date_of_birth": "1990-05-05",
		"nickname": "monalisa", "about_me": "Hi", "use_github_avatar": true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("complete: status = %d, body = %s", w.Code, w.Body.String())
	}
	var newSession *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "session_token" {
			newSession = c
		}
	}
	if newSession == nil {
		t.Fatal("complete did not log the user in")
	}
	ok, userID, email := whoAmI(t, g, newSession)
	if !ok || email != "mona@example.com" {
		t.Fatalf("check-auth after sign-up = %v %d %q", ok, userID, email)
	}
	var avatar, nickname string
	g.DB.QueryRow("SELECT COALESCE(avatar_path,''), COALESCE(nickname,'') FROM users WHERE id = ?", userID).Scan(&avatar, &nickname)
	if avatar != "https://avatars.githubusercontent.com/u/583231?v=4" || nickname != "monalisa" {
		t.Errorf("saved avatar %q nickname %q", avatar, nickname)
	}

	// The token is single-use.
	if w := complete(map[string]interface{}{"token": token, "first_name": "M", "last_name": "O", "date_of_birth": "1990-05-05"}); w.Code != http.StatusNotFound {
		t.Errorf("reused token: status = %d, want 404", w.Code)
	}

	// Next time: straight in, same account.
	state, _ = startGitHubLogin(t, g)
	loc, session = githubCallback(t, g, state, "code=good-code&state="+state.Value)
	if loc.String() != "http://frontend.test/" || session == nil {
		t.Fatalf("returning user: redirected to %s (session %v), want home with a session", loc, session != nil)
	}
	if ok, again, _ := whoAmI(t, g, session); !ok || again != userID {
		t.Errorf("returning user logged in as %d, want %d", again, userID)
	}

	// Still exactly one account for this email.
	var n int
	g.DB.QueryRow("SELECT COUNT(*) FROM users WHERE email = 'mona@example.com'").Scan(&n)
	if n != 1 {
		t.Errorf("%d accounts for the email, want 1", n)
	}
}

// TestGitHubLinksExistingAccountByVerifiedEmail - someone who registered
// with a password and then uses "Continue with GitHub" (same verified
// email) gets their existing account, not a second one.
func TestGitHubLinksExistingAccountByVerifiedEmail(t *testing.T) {
	fake := newFakeGitHub(t, octocat, []fakeGitHubEmail{{"existing@example.com", true, true}})
	g := newTestGitHubOAuth(t, fake)
	existingID := registerTestUser(t, g.DB, "existing@example.com")

	state, _ := startGitHubLogin(t, g)
	loc, session := githubCallback(t, g, state, "code=good-code&state="+state.Value)
	if loc.String() != "http://frontend.test/" || session == nil {
		t.Fatalf("redirected to %s (session %v), want home with a session", loc, session != nil)
	}
	if ok, id, _ := whoAmI(t, g, session); !ok || id != existingID {
		t.Errorf("logged in as %d, want the existing account %d", id, existingID)
	}
	var linked int64
	g.DB.QueryRow("SELECT user_id FROM oauth_accounts WHERE provider = 'github' AND provider_user_id = '583231'").Scan(&linked)
	if linked != existingID {
		t.Errorf("GitHub account linked to %d, want %d", linked, existingID)
	}
}

// TestGitHubNeverUsesUnverifiedEmail - an unverified GitHub email must not
// sign anyone into the local account that owns that address.
func TestGitHubNeverUsesUnverifiedEmail(t *testing.T) {
	fake := newFakeGitHub(t, octocat, []fakeGitHubEmail{{"victim@example.com", true, false}})
	g := newTestGitHubOAuth(t, fake)
	registerTestUser(t, g.DB, "victim@example.com")

	state, _ := startGitHubLogin(t, g)
	loc, session := githubCallback(t, g, state, "code=good-code&state="+state.Value)
	if loc.String() != "http://frontend.test/login?oauth_error=no_verified_email" || session != nil {
		t.Errorf("redirected to %s (session %v), want no_verified_email and no session", loc, session != nil)
	}
}

func TestGitHubCallbackErrors(t *testing.T) {
	fake := newFakeGitHub(t, octocat, []fakeGitHubEmail{{"mona@example.com", true, true}})
	g := newTestGitHubOAuth(t, fake)

	state, _ := startGitHubLogin(t, g)
	if loc, _ := githubCallback(t, g, state, "error=access_denied&state="+state.Value); loc.String() != "http://frontend.test/login?oauth_error=denied" {
		t.Errorf("user cancelled on GitHub: redirected to %s", loc)
	}
	state, _ = startGitHubLogin(t, g)
	if loc, _ := githubCallback(t, g, state, "code=bad-code&state="+state.Value); loc.String() != "http://frontend.test/login?oauth_error=github_error" {
		t.Errorf("bad code: redirected to %s", loc)
	}
	w := httptest.NewRecorder()
	g.Pending(w, httptest.NewRequest(http.MethodGet, "/api/auth/oauth/pending?token=nope", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("unknown pending token: status = %d, want 404", w.Code)
	}
}

// TestGitHubLoginDisabledWithoutKeys - with no GitHub app configured the
// feature reports itself off and never redirects to GitHub.
func TestGitHubLoginDisabledWithoutKeys(t *testing.T) {
	g := &GitHubOAuth{DB: newTestDB(t), FrontendURL: "http://frontend.test"}

	w := httptest.NewRecorder()
	g.Providers(w, httptest.NewRequest(http.MethodGet, "/api/auth/providers", nil))
	if strings.TrimSpace(w.Body.String()) != `{"github":false}` {
		t.Errorf("providers = %s, want github:false", w.Body.String())
	}
	w = httptest.NewRecorder()
	g.Login(w, httptest.NewRequest(http.MethodGet, "/api/auth/github/login", nil))
	if w.Header().Get("Location") != "http://frontend.test/login?oauth_error=not_configured" {
		t.Errorf("login without keys redirected to %q", w.Header().Get("Location"))
	}
}
