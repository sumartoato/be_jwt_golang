package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/example/be_jwt_golang/backend/internal/user"
)

type Handler struct {
	users      user.Repository
	tokens     *TokenManager
	bcryptCost int
	// dummyHash is compared against when the user does not exist, so login
	// timing does not reveal whether an email is registered.
	dummyHash []byte
}

func NewHandler(users user.Repository, tokens *TokenManager, bcryptCost int) *Handler {
	dummy, _ := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcryptCost)
	return &Handler{users: users, tokens: tokens, bcryptCost: bcryptCost, dummyHash: dummy}
}

// Routes registers all auth endpoints on mux.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/register", h.Register)
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.Handle("POST /api/auth/refresh", h.tokens.Middleware(http.HandlerFunc(h.Refresh)))
	mux.Handle("GET /api/me", h.tokens.Middleware(http.HandlerFunc(h.Me)))
}

type tokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if errs := req.Validate(); len(errs) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "validation failed", "errors": errs})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), h.bcryptCost)
	if err != nil {
		h.internalError(w, err)
		return
	}
	u, err := h.users.Create(r.Context(), NormalizeEmail(req.Email), string(hash))
	if errors.Is(err, user.ErrEmailTaken) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":  "email already registered",
			"errors": FieldErrors{"email": "Email is already registered"},
		})
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": u.ID, "email": u.Email})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if errs := req.Validate(); len(errs) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "validation failed", "errors": errs})
		return
	}
	u, err := h.users.GetByEmail(r.Context(), NormalizeEmail(req.Email))
	if err != nil && !errors.Is(err, user.ErrNotFound) {
		h.internalError(w, err)
		return
	}
	found := err == nil
	hash := h.dummyHash
	if found {
		hash = []byte(u.PasswordHash)
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(req.Password)) != nil || !found {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	h.issueToken(w, u.ID, u.Email)
}

// Refresh issues a new token for a still-valid one (sliding expiration).
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	id, _ := IdentityFrom(r.Context())
	h.issueToken(w, id.UserID, id.Email)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	id, _ := IdentityFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]string{
		"id":      id.UserID,
		"email":   id.Email,
		"message": fmt.Sprintf("Hello %s, welcome back", id.Email),
	})
}

func (h *Handler) issueToken(w http.ResponseWriter, userID, email string) {
	token, exp, err := h.tokens.Generate(userID, email)
	if err != nil {
		h.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tokenResponse{Token: token, ExpiresAt: exp})
}

func (h *Handler) internalError(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
