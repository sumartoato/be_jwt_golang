package branchauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/auth"
)

func TestMineHandler(t *testing.T) {
	tokens := auth.NewTokenManager("test-secret-that-is-at-least-32-characters", time.Minute)
	var asked string
	mapped := map[string][]Branch{"u-cs": {{Kode: "CTM", Nama: "KLINIK CITRA MEDIKA"}, {Kode: "KMJ", Nama: "KLINIK MITRA JATINEGARA"}}}
	h := tokens.Middleware(MineHandler(func(_ context.Context, userID string) ([]Branch, error) {
		asked = userID
		if userID == "u-broken" {
			return nil, errors.New("db down")
		}
		return mapped[userID], nil
	}))
	get := func(userID string) (int, map[string]any) {
		req := httptest.NewRequest("GET", "/api/me/branches", nil)
		if userID != "" {
			tok, _, _ := tokens.Generate(userID, userID+"@medika.com")
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		out := map[string]any{}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}

	code, out := get("u-cs")
	list, _ := out["branches"].([]any)
	if code != http.StatusOK || asked != "u-cs" || len(list) != 2 {
		t.Fatalf("mapped user: %d %v (asked %q)", code, out, asked)
	}
	if code, out := get("u-none"); code != http.StatusOK || out["branches"] == nil || len(out["branches"].([]any)) != 0 {
		t.Errorf("unmapped user must get an empty list: %d %v", code, out)
	}
	if code, _ := get(""); code != http.StatusUnauthorized {
		t.Errorf("no token: %d", code)
	}
	if code, _ := get("u-broken"); code != http.StatusInternalServerError {
		t.Errorf("repository error: %d", code)
	}
}
