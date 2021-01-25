package healthcheck

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient_Live(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/live", nil)
	rw := httptest.NewRecorder()

	New().Live(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
}

func TestClient_Ready(t *testing.T) {
	testCases := []struct {
		desc            string
		faunaStatusCode int
		expected        int
	}{
		{
			desc:            "OK",
			faunaStatusCode: http.StatusOK,
			expected:        http.StatusOK,
		},
		{
			desc:            "KO",
			faunaStatusCode: http.StatusServiceUnavailable,
			expected:        http.StatusServiceUnavailable,
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			faunaSrv := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(test.faunaStatusCode)
				}))
			t.Cleanup(faunaSrv.Close)

			healthChecker := New()
			healthChecker.faunaPing = faunaSrv.URL

			req := httptest.NewRequest(http.MethodGet, "/ready", nil)
			rw := httptest.NewRecorder()

			healthChecker.Ready(rw, req)

			assert.Equal(t, test.expected, rw.Code)
		})
	}
}
