package auth

import "testing"

func TestRegisterValidate(t *testing.T) {
	tests := []struct {
		name      string
		req       RegisterRequest
		wantField []string
	}{
		{"valid", RegisterRequest{"a@b.com", "password1", "password1"}, nil},
		{"valid with spaces and caps", RegisterRequest{"  A@B.com ", "password1", "password1"}, nil},
		{"all empty", RegisterRequest{}, []string{"email", "password", "confirmPassword"}},
		{"bad email", RegisterRequest{"not-an-email", "password1", "password1"}, []string{"email"}},
		{"email without tld", RegisterRequest{"a@b", "password1", "password1"}, []string{"email"}},
		{"display name email", RegisterRequest{"Bob <a@b.com>", "password1", "password1"}, []string{"email"}},
		{"short password", RegisterRequest{"a@b.com", "short", "short"}, []string{"password"}},
		{"mismatch", RegisterRequest{"a@b.com", "password1", "password2"}, []string{"confirmPassword"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.req.Validate()
			if len(errs) != len(tt.wantField) {
				t.Fatalf("got errors %v, want fields %v", errs, tt.wantField)
			}
			for _, f := range tt.wantField {
				if _, ok := errs[f]; !ok {
					t.Errorf("missing error for %q, got %v", f, errs)
				}
			}
		})
	}
}

func TestLoginValidate(t *testing.T) {
	if errs := (LoginRequest{"a@b.com", "x"}).Validate(); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if errs := (LoginRequest{}).Validate(); len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %v", errs)
	}
}
