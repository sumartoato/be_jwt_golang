package auth

import (
	"net/mail"
	"strings"
)

const (
	minPasswordLen = 8
	maxPasswordLen = 72 // bcrypt limit
)

type RegisterRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// FieldErrors maps a field name to a human readable message.
type FieldErrors map[string]string

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	if len(email) > 254 {
		return false
	}
	addr, err := mail.ParseAddress(email)
	// Reject display-name forms like "Bob <bob@x.com>" and require a dot in the domain.
	if err != nil || addr.Address != email {
		return false
	}
	at := strings.LastIndex(email, "@")
	return at > 0 && strings.Contains(email[at+1:], ".")
}

func (r RegisterRequest) Validate() FieldErrors {
	errs := FieldErrors{}
	email := NormalizeEmail(r.Email)
	switch {
	case email == "":
		errs["email"] = "Email is required"
	case !validEmail(email):
		errs["email"] = "Email is not valid"
	}
	switch {
	case r.Password == "":
		errs["password"] = "Password is required"
	case len(r.Password) < minPasswordLen:
		errs["password"] = "Password must be at least 8 characters"
	case len(r.Password) > maxPasswordLen:
		errs["password"] = "Password must be at most 72 characters"
	}
	switch {
	case r.ConfirmPassword == "":
		errs["confirmPassword"] = "Please confirm your password"
	case r.ConfirmPassword != r.Password:
		errs["confirmPassword"] = "Passwords do not match"
	}
	return errs
}

func (r LoginRequest) Validate() FieldErrors {
	errs := FieldErrors{}
	if NormalizeEmail(r.Email) == "" {
		errs["email"] = "Email is required"
	}
	if r.Password == "" {
		errs["password"] = "Password is required"
	}
	return errs
}
