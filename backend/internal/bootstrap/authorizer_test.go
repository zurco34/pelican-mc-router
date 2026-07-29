package bootstrap

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeSecretReader struct {
	value []byte
	err   error
}

func (f fakeSecretReader) Read(string) ([]byte, error) {
	return append([]byte(nil), f.value...), f.err
}

func TestAuthorizerAuthorize(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		reader  fakeSecretReader
		wantErr bool
	}{
		{name: "authorized", header: "Bearer bootstrap-test-token", reader: fakeSecretReader{value: []byte("bootstrap-test-token\n")}},
		{name: "missing", reader: fakeSecretReader{value: []byte("bootstrap-test-token")}, wantErr: true},
		{name: "wrong scheme", header: "Basic bootstrap-test-token", reader: fakeSecretReader{value: []byte("bootstrap-test-token")}, wantErr: true},
		{name: "wrong token", header: "Bearer different", reader: fakeSecretReader{value: []byte("bootstrap-test-token")}, wantErr: true},
		{name: "reader failure", header: "Bearer bootstrap-test-token", reader: fakeSecretReader{err: errors.New("unavailable")}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authorizer, err := New(test.reader, "bootstrap-token")
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/setup", nil)
			request.Header.Set("Authorization", test.header)
			err = authorizer.Authorize(request)
			if (err != nil) != test.wantErr || err != nil && !errors.Is(err, ErrUnauthorized) {
				t.Fatalf("Authorize() error = %v, want error = %t", err, test.wantErr)
			}
		})
	}
}
