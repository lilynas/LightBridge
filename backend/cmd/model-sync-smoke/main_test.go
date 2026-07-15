package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/WilliamWang1721/LightBridge/internal/config"
	"github.com/WilliamWang1721/LightBridge/internal/pkg/tlsfingerprint"
	"github.com/WilliamWang1721/LightBridge/internal/service"
	"github.com/stretchr/testify/require"
)

type smokeHTTPUpstream struct {
	response *http.Response
	request  *http.Request
	proxyURL string
}

func (s *smokeHTTPUpstream) Do(req *http.Request, proxyURL string, _ int64, _ int) (*http.Response, error) {
	s.request = req
	s.proxyURL = proxyURL
	return s.response, nil
}

func (s *smokeHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(req, proxyURL, accountID, concurrency)
}

func TestRunPerformsOneWayCustomModelDiscovery(t *testing.T) {
	t.Parallel()

	upstream := &smokeHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(`{"result":{"models":["model-b",{"model_id":"model-a"}]}}`)),
	}}
	var output bytes.Buffer
	err := run(
		[]string{
			"-protocol", service.CustomProtocolOpenAIChatCompletions,
			"-base-url", "https://custom.example.com/v1/chat/completions",
		},
		func(key string) string {
			if key == defaultAPIKeyEnv {
				return "smoke-secret"
			}
			return ""
		},
		func(*config.Config) service.HTTPUpstream { return upstream },
		&output,
	)
	require.NoError(t, err)
	require.NotNil(t, upstream.request)
	require.Equal(t, "https://custom.example.com/v1/models", upstream.request.URL.String())
	require.Equal(t, "Bearer smoke-secret", upstream.request.Header.Get("Authorization"))

	var result smokeResult
	require.NoError(t, json.Unmarshal(output.Bytes(), &result))
	require.Equal(t, 2, result.Count)
	require.Equal(t, []string{"model-a", "model-b"}, result.Models)
}

func TestRunUsesExplicitModelsURL(t *testing.T) {
	t.Parallel()

	upstream := &smokeHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(`{"data":[{"id":"model-a"}]}`)),
	}}
	err := run(
		[]string{
			"-protocol", service.CustomProtocolOpenAIResponses,
			"-base-url", "https://custom.example.com/v1",
			"-models-url", "https://catalog.example.com/models/list",
			"-pretty=false",
		},
		func(string) string { return "smoke-secret" },
		func(*config.Config) service.HTTPUpstream { return upstream },
		io.Discard,
	)
	require.NoError(t, err)
	require.Equal(t, "https://catalog.example.com/models/list", upstream.request.URL.String())
}

func TestRunSupportsAllCustomProtocols(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		protocol   string
		baseURL    string
		wantURL    string
		wantHeader string
		wantValue  string
	}{
		{"OpenAI responses", service.CustomProtocolOpenAIResponses, "https://custom.example.com/v1/responses", "https://custom.example.com/v1/models", "Authorization", "Bearer smoke-secret"},
		{"OpenAI chat", service.CustomProtocolOpenAIChatCompletions, "https://custom.example.com/v1/chat/completions", "https://custom.example.com/v1/models", "Authorization", "Bearer smoke-secret"},
		{"OpenAI embeddings", service.CustomProtocolOpenAIEmbeddings, "https://custom.example.com/v1/embeddings", "https://custom.example.com/v1/models", "Authorization", "Bearer smoke-secret"},
		{"Anthropic messages", service.CustomProtocolAnthropicMessages, "https://custom.example.com/v1/messages", "https://custom.example.com/v1/models", "x-api-key", "smoke-secret"},
		{"Gemini", service.CustomProtocolGemini, "https://custom.example.com/v1/models/gemini:generateContent", "https://custom.example.com/v1beta/models", "x-goog-api-key", "smoke-secret"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			upstream := &smokeHTTPUpstream{response: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewBufferString(`{"data":[{"id":"model-a"}]}`)),
			}}
			err := run(
				[]string{"-protocol", tt.protocol, "-base-url", tt.baseURL},
				func(string) string { return "smoke-secret" },
				func(*config.Config) service.HTTPUpstream { return upstream },
				io.Discard,
			)
			require.NoError(t, err)
			require.Equal(t, tt.wantURL, upstream.request.URL.String())
			require.Equal(t, tt.wantValue, upstream.request.Header.Get(tt.wantHeader))
		})
	}
}

func TestRunUsesProxyWithoutPrintingCredentials(t *testing.T) {
	t.Parallel()

	upstream := &smokeHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(`{"data":[{"id":"model-a"}]}`)),
	}}
	var output bytes.Buffer
	proxyURL := "http://proxy-user:proxy-password" + "@" + "proxy.example.com:7890"
	err := run(
		[]string{
			"-protocol", service.CustomProtocolOpenAIResponses,
			"-base-url", "https://custom.example.com/v1",
			"-proxy-url", proxyURL,
		},
		func(string) string { return "smoke-secret" },
		func(*config.Config) service.HTTPUpstream { return upstream },
		&output,
	)
	require.NoError(t, err)
	require.Equal(t, proxyURL, upstream.proxyURL)
	require.NotContains(t, output.String(), "proxy-user")
	require.NotContains(t, output.String(), "proxy-password")
	require.NotContains(t, output.String(), "smoke-secret")
}

func TestRunSanitizesUpstreamFailure(t *testing.T) {
	t.Parallel()

	upstream := &smokeHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(`{"error":"invalid credential","api_key":"smoke-secret"}`)),
	}}
	err := run(
		[]string{"-protocol", service.CustomProtocolOpenAIResponses, "-base-url", "https://custom.example.com/v1"},
		func(string) string { return "smoke-secret" },
		func(*config.Config) service.HTTPUpstream { return upstream },
		io.Discard,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTP 401")
	require.Contains(t, err.Error(), "invalid credential")
	require.NotContains(t, err.Error(), "smoke-secret")
}
