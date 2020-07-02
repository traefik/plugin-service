package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_cleanModuleName(t *testing.T) {
	testCases := []struct {
		name     string
		expected string
	}{
		{
			name:     "/powpow/",
			expected: "powpow",
		},
		{
			name:     "/powpow/v2",
			expected: "powpow/v2",
		},
		{
			name:     "powpow/v2",
			expected: "powpow/v2",
		},

		{
			name:     "powpow",
			expected: "powpow",
		},

		{
			name:     "powpow/v2/",
			expected: "powpow/v2",
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			name := cleanModuleName(test.name)
			assert.Equal(t, test.expected, name)
		})
	}
}
