package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/example/be_jwt_golang/backend/internal/user"
)

// fakeRepo is an in-memory user.Repository.
type fakeRepo struct {
	mu    sync.Mutex
	users map[string]user.User
}

func (f *fakeRepo) Create(_ context.Context, email, hash string) (user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.users[email]; ok {
		return user.User{}, user.ErrEmailTaken
	}
	u := user.User{ID: "id-" + strconv.Itoa(len(f.users)+1), Email: email, PasswordHash: hash}
	f.users[email] = u
	return u, nil
}

func (f *fakeRepo) GetByEmail(_ context.Context, email string) (user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[email]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return u, nil
}

func newTestServer() (http.Handler, *TokenManager) {
	tokens := NewTokenManager(testSecret, 15*time.Minute)
	h := NewHandler(&fakeRepo{users: map[string]user.User{}}, tokens, bcrypt.MinCost)
	mux := http.NewServeMux()
	h.Routes(mux)
	return mux, tokens
}

func do(t *testing.T, srv http.Handler, method, path, token string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func TestFullAuthFlow(t *testing.T) {
	srv, tokens := newTestServer()

	rec, _ := do(t, srv, "POST", "/api/auth/register", "", map[string]string{
		"email": "Alice@Example.com", "password": "password1", "confirmPassword": "password1",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: got %d %s", rec.Code, rec.Body)
	}

	rec, _ = do(t, srv, "POST", "/api/auth/register", "", map[string]string{
		"email": "alice@example.com", "password": "password1", "confirmPassword": "password1",
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate register: got %d", rec.Code)
	}

	rec, _ = do(t, srv, "POST", "/api/auth/login", "", map[string]string{"email": "alice@example.com", "password": "wrong-pass"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad password: got %d", rec.Code)
	}

	rec, _ = do(t, srv, "POST", "/api/auth/login", "", map[string]string{"email": "nobody@example.com", "password": "password1"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown user: got %d", rec.Code)
	}

	rec, body := do(t, srv, "POST", "/api/auth/login", "", map[string]string{"email": "alice@example.com", "password": "password1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("login: got %d %s", rec.Code, rec.Body)
	}
	token, _ := body["token"].(string)
	claims, err := tokens.Parse(token)
	if err != nil {
		t.Fatalf("login token invalid: %v", err)
	}
	if claims.Email != "alice@example.com" || claims.Subject == "" {
		t.Fatalf("unexpected claims %+v", claims)
	}

	rec, body = do(t, srv, "GET", "/api/me", token, nil)
	if rec.Code != http.StatusOK || body["message"] != "Hello alice@example.com, welcome back" {
		t.Fatalf("me: got %d %v", rec.Code, body)
	}

	rec, body = do(t, srv, "POST", "/api/auth/refresh", token, nil)
	if rec.Code != http.StatusOK || body["token"] == "" {
		t.Fatalf("refresh: got %d %v", rec.Code, body)
	}
}

func TestRegisterValidationErrors(t *testing.T) {
	srv, _ := newTestServer()
	rec, body := do(t, srv, "POST", "/api/auth/register", "", map[string]string{
		"email": "bad", "password": "123", "confirmPassword": "456",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d", rec.Code)
	}
	errs, _ := body["errors"].(map[string]any)
	for _, f := range []string{"email", "password", "confirmPassword"} {
		if _, ok := errs[f]; !ok {
			t.Errorf("missing field error %q in %v", f, errs)
		}
	}
}

func TestProtectedRouteRejectsMissingOrBadToken(t *testing.T) {
	srv, _ := newTestServer()
	for _, tok := range []string{"", "garbage"} {
		rec, _ := do(t, srv, "GET", "/api/me", tok, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("token %q: got %d", tok, rec.Code)
		}
	}
}

func TestProtectedRouteRejectsExpiredToken(t *testing.T) {
	srv, _ := newTestServer()
	old := NewTokenManager(testSecret, 15*time.Minute)
	old.now = func() time.Time { return time.Now().Add(-16 * time.Minute) }
	token, _, _ := old.Generate("id-1", "a@b.com")
	rec, _ := do(t, srv, "GET", "/api/me", token, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expired token: got %d", rec.Code)
	}
}
