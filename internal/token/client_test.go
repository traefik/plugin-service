package token

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientGet(t *testing.T) {
	client := Client{}

	_, err := client.Get(context.Background(), "valueAAA")
	require.NoError(t, err)
}

func TestClientCheck(t *testing.T) {
	client := Client{}

	_, err := client.Check(context.Background(), "valueAAA")
	require.NoError(t, err)
}
