package functions

import (
	"os"
	"testing"

	"github.com/caarlos0/env/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_config_Parse(t *testing.T) {
	testCases := []struct {
		desc     string
		envs     map[string]string
		expected config
	}{
		{
			desc: "all",
			envs: map[string]string{
				"FAUNADB_ENDPOINT":            "faunaDB endpoint",
				"FAUNADB_SECRET":              "faunaDB secret",
				"PILOT_TOKEN_URL":             "Pilot token URL",
				"PILOT_JWT_CERT":              "JWT certificate",
				"PILOT_SERVICES_ACCESS_TOKEN": "c2VjcmV0",
				"PILOT_GO_PROXY_URL":          "Go Proxy URL",
				"PILOT_GO_PROXY_USERNAME":     "Go Proxy Username",
				"PILOT_GO_PROXY_PASSWORD":     "Go Proxy Password",
				"PILOT_GITHUB_TOKEN":          "GitHub Token",
			},
			expected: config{
				FaunaDB: faunaDB{
					Endpoint: "faunaDB endpoint",
					Secret:   "faunaDB secret",
				},
				Pilot: pilot{
					TokenURL:            "Pilot token URL",
					JWTCert:             "JWT certificate",
					ServicesAccessToken: "secret",
					GoProxyURL:          "Go Proxy URL",
					GoProxyUsername:     "Go Proxy Username",
					GoProxyPassword:     "Go Proxy Password",
					GitHubToken:         "GitHub Token",
				},
				Tracing: tracing{
					Endpoint:    "https://collector.infra.traefiklabs.tech",
					Username:    "jaeger",
					Password:    "jaeger",
					Probability: 0.1,
				},
			},
		},
		{
			desc: "optional FaunaDBSecret",
			envs: map[string]string{
				"FAUNADB_SECRET":              "faunaDB secret",
				"PILOT_TOKEN_URL":             "Pilot token URL",
				"PILOT_JWT_CERT":              "JWT certificate",
				"PILOT_SERVICES_ACCESS_TOKEN": "c2VjcmV0",
				"PILOT_GO_PROXY_URL":          "Go Proxy URL",
				"PILOT_GO_PROXY_USERNAME":     "Go Proxy Username",
				"PILOT_GO_PROXY_PASSWORD":     "Go Proxy Password",
				"PILOT_GITHUB_TOKEN":          "GitHub Token",
			},
			expected: config{
				FaunaDB: faunaDB{
					Endpoint: "",
					Secret:   "faunaDB secret",
				},
				Pilot: pilot{
					TokenURL:            "Pilot token URL",
					JWTCert:             "JWT certificate",
					ServicesAccessToken: "secret",
					GoProxyURL:          "Go Proxy URL",
					GoProxyUsername:     "Go Proxy Username",
					GoProxyPassword:     "Go Proxy Password",
					GitHubToken:         "GitHub Token",
				},
				Tracing: tracing{
					Endpoint:    "https://collector.infra.traefiklabs.tech",
					Username:    "jaeger",
					Password:    "jaeger",
					Probability: 0.1,
				},
			},
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			for k, v := range test.envs {
				_ = os.Setenv(k, v)
			}

			t.Cleanup(func() {
				for k := range test.envs {
					_ = os.Unsetenv(k)
				}
			})

			cfg := config{}

			err := env.Parse(&cfg)
			require.NoError(t, err)

			assert.Equal(t, test.expected, cfg)
		})
	}
}

func Test_config_Parse_errors(t *testing.T) {
	testCases := []struct {
		desc     string
		envs     map[string]string
		expected string
	}{
		{
			desc: "FAUNADB_SECRET is required",
			envs: map[string]string{
				"FAUNADB_ENDPOINT":            "faunaDB endpoint",
				"PILOT_TOKEN_URL":             "Pilot token URL",
				"PILOT_JWT_CERT":              "JWT certificate",
				"PILOT_SERVICES_ACCESS_TOKEN": "c2VjcmV0",
				"PILOT_GO_PROXY_URL":          "Go Proxy URL",
				"PILOT_GO_PROXY_USERNAME":     "Go Proxy Username",
				"PILOT_GO_PROXY_PASSWORD":     "Go Proxy Password",
				"PILOT_GITHUB_TOKEN":          "GitHub Token",
			},
			expected: `env: required environment variable "FAUNADB_SECRET" is not set`,
		},
		{
			desc: "PILOT_TOKEN_URL is required",
			envs: map[string]string{
				"FAUNADB_ENDPOINT":            "faunaDB endpoint",
				"FAUNADB_SECRET":              "faunaDB secret",
				"PILOT_JWT_CERT":              "JWT certificate",
				"PILOT_SERVICES_ACCESS_TOKEN": "c2VjcmV0",
				"PILOT_GO_PROXY_URL":          "Go Proxy URL",
				"PILOT_GO_PROXY_USERNAME":     "Go Proxy Username",
				"PILOT_GO_PROXY_PASSWORD":     "Go Proxy Password",
				"PILOT_GITHUB_TOKEN":          "GitHub Token",
			},
			expected: `env: required environment variable "PILOT_TOKEN_URL" is not set`,
		},
		{
			desc: "PILOT_JWT_CERT is required",
			envs: map[string]string{
				"FAUNADB_ENDPOINT":            "faunaDB endpoint",
				"FAUNADB_SECRET":              "faunaDB secret",
				"PILOT_TOKEN_URL":             "Pilot token URL",
				"PILOT_SERVICES_ACCESS_TOKEN": "c2VjcmV0",
				"PILOT_GO_PROXY_URL":          "Go Proxy URL",
				"PILOT_GO_PROXY_USERNAME":     "Go Proxy Username",
				"PILOT_GO_PROXY_PASSWORD":     "Go Proxy Password",
				"PILOT_GITHUB_TOKEN":          "GitHub Token",
			},
			expected: `env: required environment variable "PILOT_JWT_CERT" is not set`,
		},
		{
			desc: "PILOT_SERVICES_ACCESS_TOKEN is required",
			envs: map[string]string{
				"FAUNADB_ENDPOINT":        "faunaDB endpoint",
				"FAUNADB_SECRET":          "faunaDB secret",
				"PILOT_TOKEN_URL":         "Pilot token URL",
				"PILOT_JWT_CERT":          "JWT certificate",
				"PILOT_GO_PROXY_URL":      "Go Proxy URL",
				"PILOT_GO_PROXY_USERNAME": "Go Proxy Username",
				"PILOT_GO_PROXY_PASSWORD": "Go Proxy Password",
				"PILOT_GITHUB_TOKEN":      "GitHub Token",
			},
			expected: `env: required environment variable "PILOT_SERVICES_ACCESS_TOKEN" is not set`,
		},
		{
			desc: "PILOT_GO_PROXY_URL is required",
			envs: map[string]string{
				"FAUNADB_ENDPOINT":            "faunaDB endpoint",
				"FAUNADB_SECRET":              "faunaDB secret",
				"PILOT_TOKEN_URL":             "Pilot token URL",
				"PILOT_JWT_CERT":              "JWT certificate",
				"PILOT_SERVICES_ACCESS_TOKEN": "c2VjcmV0",
				"PILOT_GO_PROXY_USERNAME":     "Go Proxy Username",
				"PILOT_GO_PROXY_PASSWORD":     "Go Proxy Password",
				"PILOT_GITHUB_TOKEN":          "GitHub Token",
			},
			expected: `env: required environment variable "PILOT_GO_PROXY_URL" is not set`,
		},
		{
			desc: "PILOT_GO_PROXY_USERNAME is required",
			envs: map[string]string{
				"FAUNADB_ENDPOINT":            "faunaDB endpoint",
				"FAUNADB_SECRET":              "faunaDB secret",
				"PILOT_TOKEN_URL":             "Pilot token URL",
				"PILOT_JWT_CERT":              "JWT certificate",
				"PILOT_SERVICES_ACCESS_TOKEN": "c2VjcmV0",
				"PILOT_GO_PROXY_URL":          "Go Proxy URL",
				"PILOT_GO_PROXY_PASSWORD":     "Go Proxy Password",
				"PILOT_GITHUB_TOKEN":          "GitHub Token",
			},
			expected: `env: required environment variable "PILOT_GO_PROXY_USERNAME" is not set`,
		},
		{
			desc: "PILOT_GO_PROXY_PASSWORD is required",
			envs: map[string]string{
				"FAUNADB_ENDPOINT":            "faunaDB endpoint",
				"FAUNADB_SECRET":              "faunaDB secret",
				"PILOT_TOKEN_URL":             "Pilot token URL",
				"PILOT_JWT_CERT":              "JWT certificate",
				"PILOT_SERVICES_ACCESS_TOKEN": "c2VjcmV0",
				"PILOT_GO_PROXY_URL":          "Go Proxy URL",
				"PILOT_GO_PROXY_USERNAME":     "Go Proxy Username",
				"PILOT_GITHUB_TOKEN":          "GitHub Token",
			},
			expected: `env: required environment variable "PILOT_GO_PROXY_PASSWORD" is not set`,
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			for k, v := range test.envs {
				_ = os.Setenv(k, v)
			}

			t.Cleanup(func() {
				for k := range test.envs {
					_ = os.Unsetenv(k)
				}
			})

			cfg := config{}

			err := env.Parse(&cfg)
			assert.EqualError(t, err, test.expected)
		})
	}
}
