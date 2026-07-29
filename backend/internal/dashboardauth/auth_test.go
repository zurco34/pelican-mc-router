package dashboardauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeVerifier struct {
	identity identity
	err      error
	token    string
}

func (f *fakeVerifier) Verify(_ context.Context, token string) (identity, error) {
	f.token = token
	return f.identity, f.err
}

func TestAuthorizerAuthorize(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		verifier  fakeVerifier
		wantError error
	}{
		{
			name:   "authorized role",
			header: "Bearer signed-token",
			verifier: fakeVerifier{identity: identity{
				subject: "operator",
				roles:   []string{"viewer"},
			}},
		},
		{
			name:      "missing token",
			wantError: ErrUnauthenticated,
		},
		{
			name:      "verification failure",
			header:    "Bearer invalid-token",
			verifier:  fakeVerifier{err: errors.New("signature verification failed")},
			wantError: ErrUnauthenticated,
		},
		{
			name:      "missing subject",
			header:    "Bearer signed-token",
			verifier:  fakeVerifier{identity: identity{roles: []string{"viewer"}}},
			wantError: ErrUnauthenticated,
		},
		{
			name:   "missing role",
			header: "Bearer signed-token",
			verifier: fakeVerifier{identity: identity{
				subject: "operator",
				roles:   []string{"other"},
			}},
			wantError: ErrForbidden,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verifier := test.verifier
			authorizer := &oidcAuthorizer{verifier: &verifier, requiredRole: "viewer"}
			request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
			request.Header.Set("Authorization", test.header)

			err := authorizer.Authorize(context.Background(), request)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Authorize() error = %v, want %v", err, test.wantError)
			}
			if test.wantError == nil && verifier.token != "signed-token" {
				t.Fatalf("verified token = %q, want signed-token", verifier.token)
			}
		})
	}
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{value: "Bearer token", want: "token"},
		{value: "Bearer  token  ", want: "token"},
		{value: "bearer token"},
		{value: "Basic token"},
		{value: "Bearer "},
	}
	for _, test := range tests {
		token, err := bearerToken(test.value)
		if test.want == "" {
			if !errors.Is(err, ErrUnauthenticated) {
				t.Errorf("bearerToken(%q) error = %v", test.value, err)
			}
			continue
		}
		if err != nil || token != test.want {
			t.Errorf("bearerToken(%q) = %q, %v; want %q, nil", test.value, token, err, test.want)
		}
	}
}
