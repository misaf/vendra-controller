package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBearerAuthentication(t *testing.T) {
	handler := Bearer("secret", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, test := range []struct {
		header string
		status int
	}{{"", 401}, {"Bearer wrong", 401}, {"Bearer secret", 204}} {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.Header.Set("Authorization", test.header)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("header %q: got %d", test.header, response.Code)
		}
	}
}
