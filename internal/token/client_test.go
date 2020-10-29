package token

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	guuid "github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mock(data interface{}, status int) *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			err := json.NewEncoder(w).Encode(data)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}))
}

func TestClientGet(t *testing.T) {
	token := &Token{
		ID:        guuid.New().String(),
		Value:     "valueAAA",
		Name:      "NameAAA",
		CreatedAt: time.Time{},
	}

	tokenSrv := mock(token, http.StatusOK)
	defer tokenSrv.Close()

	client := New(tokenSrv.URL, "")

	checked, err := client.Get(context.Background(), token.ID)
	require.NoError(t, err)

	assert.Equal(t, token, checked)
}

func TestClientCheck(t *testing.T) {
	token := &Token{
		ID:        guuid.New().String(),
		Value:     "valueAAA",
		Name:      "NameAAA",
		CreatedAt: time.Time{},
	}

	tokenSrv := mock(token, http.StatusOK)
	defer tokenSrv.Close()

	client := New(tokenSrv.URL, "")

	checked, err := client.Check(context.Background(), token.Value)
	require.NoError(t, err)

	assert.Equal(t, token, checked)
}

func TestClientCheck_errors(t *testing.T) {
	testCases := []struct {
		desc         string
		token        string
		data         Token
		serverStatus int
		expected     int
	}{
		{
			desc:  "empty token",
			token: "",
			data: Token{
				ID:        guuid.New().String(),
				Value:     "valueAAA",
				Name:      "NameAAA",
				CreatedAt: time.Time{},
			},
			serverStatus: http.StatusInternalServerError,
		},
	}

	for _, test := range testCases {
		tokenSrv := mock(test.data, test.serverStatus)

		test := test
		t.Run(test.desc, func(t *testing.T) {
			client := New(tokenSrv.URL, "")

			checked, err := client.Check(context.Background(), test.token)
			assert.Error(t, err)

			require.Nil(t, checked)
		})

		tokenSrv.Close()
	}
}
