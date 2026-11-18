package auth

import (
	"strings"
	"testing"
	"time"
)

const testSecret = "test-secret-that-is-at-least-32-characters"

func TestGenerateAndParse(t *testing.T) {
	m := NewTokenManager(testSecret, 15*time.Minute)
	token, exp, err := m.Generate("user-123", "a@b.com")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if d := time.Until(exp); d < 14*time.Minute || d > 15*time.Minute {
		t.Fatalf("unexpected expiry in %v", d)
	}
	claims, err := m.Parse(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.Subject != "user-123" || claims.Email != "a@b.com" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestParseExpired(t *testing.T) {
	m := NewTokenManager(testSecret, 15*time.Minute)
	issued := time.Now().Add(-16 * time.Minute)
	m.now = func() time.Time { return issued }
	token, _, err := m.Generate("u", "a@b.com")
	if err != nil {
		t.Fatal(err)
	}
	m.now = time.Now
	if _, err := m.Parse(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestParseWrongSecret(t *testing.T) {
	token, _, _ := NewTokenManager(testSecret, time.Minute).Generate("u", "a@b.com")
	other := NewTokenManager(strings.Repeat("x", 32), time.Minute)
	if _, err := other.Parse(token); err == nil {
		t.Fatal("expected token signed with another secret to be rejected")
	}
}

func TestParseRejectsNoneAlg(t *testing.T) {
	// header {"alg":"none","typ":"JWT"}, payload {"sub":"u","email":"a@b.com","exp":9999999999}
	token := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJ1IiwiZW1haWwiOiJhQGIuY29tIiwiZXhwIjo5OTk5OTk5OTk5fQ."
	if _, err := NewTokenManager(testSecret, time.Minute).Parse(token); err == nil {
		t.Fatal("expected alg=none token to be rejected")
	}
}
