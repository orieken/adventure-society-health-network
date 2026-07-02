package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ashn/packages/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeSeverity(t *testing.T) {
	assert.Equal(t, domain.SeverityNormal, normalizeSeverity("normal"))
	assert.Equal(t, domain.SeverityAwakened, normalizeSeverity("Awakened"))
	assert.Equal(t, domain.SeverityDiamond, normalizeSeverity("DIAMOND"))
	assert.Equal(t, domain.IncidentSeverity("Cosmic"), normalizeSeverity("Cosmic"))
}

func TestConfiguredURLUsesEnvironmentConfigFileAndFallback(t *testing.T) {
	t.Setenv("ASHN_API_URL", "https://env-gateway")
	assert.Equal(t, "https://env-gateway", configuredURL())

	t.Setenv("ASHN_API_URL", "")
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("HOME", configRoot)
	configFile, err := configPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(configFile), 0o755))
	require.NoError(t, os.WriteFile(configFile, []byte("url: https://file-gateway\n"), 0o600))
	assert.Equal(t, "https://file-gateway", configuredURL())

	require.NoError(t, os.Remove(configFile))
	assert.Equal(t, "http://localhost:8080", configuredURL())
}

func TestRootCommandExecutesWorkflowCommands(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantMethod string
		wantPath   string
	}{
		{name: "enroll", args: []string{"enroll", "Farros", "Iron", "Grim Foundations", "Greenstone"}, wantMethod: http.MethodPost, wantPath: "/v1/adventurers"},
		{name: "eligibility", args: []string{"check-eligibility", "adv-1", "provider-1"}, wantMethod: http.MethodPost, wantPath: "/v1/eligibility"},
		{name: "auth", args: []string{"request-auth", "adv-1", "provider-1", "resurrection"}, wantMethod: http.MethodPost, wantPath: "/v1/auth-requests"},
		{name: "claim", args: []string{"submit-claim", "adv-1", "provider-1", "normal"}, wantMethod: http.MethodPost, wantPath: "/v1/claims"},
		{name: "claim-status", args: []string{"claim-status", "claim-1"}, wantMethod: http.MethodGet, wantPath: "/v1/claims/claim-1/status"},
		{name: "providers-list", args: []string{"providers", "list"}, wantMethod: http.MethodGet, wantPath: "/v1/providers"},
		{name: "providers-show", args: []string{"providers", "show", "provider-1"}, wantMethod: http.MethodGet, wantPath: "/v1/providers/provider-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawJSON = false
			var gotMethod string
			var gotPath string
			restoreClient := replaceDefaultClient(cliRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				return cliJSONResponse(t, domain.Envelope{Lore: "CLI test lore", Data: map[string]string{"ok": "true"}}), nil
			}))
			defer restoreClient()
			t.Setenv("ASHN_API_URL", "http://ashn.test")

			cmd := rootCmd()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tt.args)

			require.NoError(t, cmd.Execute())
			assert.Equal(t, tt.wantMethod, gotMethod)
			assert.Equal(t, tt.wantPath, gotPath)
		})
	}
}

func TestCallPrintsRawJSONAndPlainTextFallback(t *testing.T) {
	restoreClient := replaceDefaultClient(cliRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"lore":"raw lore"}`))}, nil
	}))
	defer restoreClient()
	t.Setenv("ASHN_API_URL", "http://ashn.test")
	rawJSON = true
	assert.NoError(t, call(http.MethodGet, "/v1/health", nil))

	restoreClient()
	restoreTextClient := replaceDefaultClient(cliRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`not-json`))}, nil
	}))
	defer restoreTextClient()
	rawJSON = false
	assert.NoError(t, call(http.MethodGet, "/v1/health", nil))
}

func TestConfigSetURLWritesConfigFile(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("HOME", configRoot)

	cmd := rootCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"config", "set-url", "https://saved-gateway"})

	require.NoError(t, cmd.Execute())
	configFile, err := configPath()
	require.NoError(t, err)
	content, err := os.ReadFile(configFile)
	require.NoError(t, err)
	assert.Equal(t, "url: https://saved-gateway\n", string(content))
}

func cliJSONResponse(t *testing.T, value any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(value)
	require.NoError(t, err)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(payload))),
	}
}

type cliRoundTripFunc func(*http.Request) (*http.Response, error)

func (f cliRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func replaceDefaultClient(transport http.RoundTripper) func() {
	original := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: transport}
	return func() {
		http.DefaultClient = original
	}
}
