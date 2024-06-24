package handlers

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			name := cleanModuleName(test.name)
			assert.Equal(t, test.expected, name)
		})
	}
}

func Test_extractPluginInfo(t *testing.T) {
	type expected struct {
		moduleName string
		version    string
	}

	testCases := []struct {
		desc     string
		url      string
		sep      string
		expected expected
	}{
		{
			desc: "public URL",
			url:  "https://plugins.traefik.io/public/download/github.com/tomMoulard/fail2ban/v0.6.6",
			sep:  "/download/",
			expected: expected{
				moduleName: "github.com/tomMoulard/fail2ban",
				version:    "v0.6.6",
			},
		},
		{
			desc: "internal URL",
			url:  "https://plugins.traefik.io/download/github.com/tomMoulard/fail2ban/v0.6.6",
			sep:  "/download/",
			expected: expected{
				moduleName: "github.com/tomMoulard/fail2ban",
				version:    "v0.6.6",
			},
		},
		{
			desc: "with extra slash",
			url:  "https://plugins.traefik.io/public/download/github.com/tomMoulard/fail2ban/v0.6.6/",
			sep:  "/download/",
			expected: expected{
				moduleName: "github.com/tomMoulard/fail2ban",
				version:    "v0.6.6",
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			endpoint, err := url.Parse(test.url)
			require.NoError(t, err)

			moduleName, version := extractPluginInfo(endpoint, test.sep)

			assert.Equal(t, test.expected.moduleName, moduleName)
			assert.Equal(t, test.expected.version, version)
		})
	}
}
