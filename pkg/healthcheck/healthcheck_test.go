package healthcheck

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient_Live(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/live", nil)
	rw := httptest.NewRecorder()

	client := Client{}
	client.Live(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
}

func TestClient_Ready(t *testing.T) {
	testCases := []struct {
		desc       string
		err        bool
		wantStatus int
	}{
		{
			desc:       "OK",
			wantStatus: http.StatusOK,
		},
		{
			desc:       "KO",
			err:        true,
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			client := Client{DB: pingerMock(func(ctx context.Context) error {
				if test.err {
					return errors.New("hey ya")
				}
				return nil
			})}

			req := httptest.NewRequest(http.MethodGet, "/ready", nil)
			rw := httptest.NewRecorder()

			client.Ready(rw, req)

			assert.Equal(t, test.wantStatus, rw.Code)
		})
	}
}

type pingerMock func(ctx context.Context) error

// Ping calls the mock.
func (m pingerMock) Ping(ctx context.Context) error {
	return m(ctx)
}
