// Command model-sync-smoke performs one read-only Custom provider model-list
// request without starting LightBridge or writing to its database.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/WilliamWang1721/LightBridge/internal/config"
	"github.com/WilliamWang1721/LightBridge/internal/pkg/tlsfingerprint"
	"github.com/WilliamWang1721/LightBridge/internal/repository"
	"github.com/WilliamWang1721/LightBridge/internal/service"
)

const defaultAPIKeyEnv = "LIGHTBRIDGE_MODEL_SYNC_API_KEY"

type smokeOptions struct {
	Protocol  string
	BaseURL   string
	ModelsURL string
	ProxyURL  string
	APIKeyEnv string
	Timeout   time.Duration
	AllowHTTP bool
	Pretty    bool
}

type smokeResult struct {
	Protocol  string   `json:"protocol"`
	BaseURL   string   `json:"base_url"`
	ModelsURL string   `json:"models_url,omitempty"`
	Proxy     bool     `json:"proxy"`
	Count     int      `json:"count"`
	Models    []string `json:"models"`
	ElapsedMS int64    `json:"elapsed_ms"`
}

type upstreamFactory func(*config.Config) service.HTTPUpstream

type proxyOverrideUpstream struct {
	inner    service.HTTPUpstream
	proxyURL string
}

func (p *proxyOverrideUpstream) Do(req *http.Request, _ string, accountID int64, concurrency int) (*http.Response, error) {
	return p.inner.Do(req, p.proxyURL, accountID, concurrency)
}

func (p *proxyOverrideUpstream) DoWithTLS(req *http.Request, _ string, accountID int64, concurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	return p.inner.DoWithTLS(req, p.proxyURL, accountID, concurrency, profile)
}

func main() {
	err := run(
		os.Args[1:],
		os.Getenv,
		func(cfg *config.Config) service.HTTPUpstream { return repository.NewHTTPUpstream(cfg) },
		os.Stdout,
	)
	if err == nil || errors.Is(err, flag.ErrHelp) {
		return
	}
	fmt.Fprintln(os.Stderr, "model sync smoke failed:", err)
	os.Exit(1)
}

func run(args []string, getenv func(string) string, newUpstream upstreamFactory, output io.Writer) error {
	options, err := parseSmokeOptions(args, output)
	if err != nil {
		return err
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	apiKey := strings.TrimSpace(getenv(options.APIKeyEnv))
	if apiKey == "" {
		return fmt.Errorf("environment variable %s is empty", options.APIKeyEnv)
	}

	cfg := &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				Enabled:           false,
				AllowInsecureHTTP: options.AllowHTTP,
			},
		},
	}
	upstream := newUpstream(cfg)
	if upstream == nil {
		return errors.New("HTTP upstream is not configured")
	}
	if options.ProxyURL != "" {
		upstream = &proxyOverrideUpstream{inner: upstream, proxyURL: options.ProxyURL}
	}
	account := &service.Account{
		ID:          0,
		Name:        "custom-model-sync-smoke",
		Platform:    service.PlatformCustom,
		Type:        service.AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  apiKey,
			"base_url": options.BaseURL,
		},
		Extra: map[string]any{"protocol": options.Protocol},
	}
	if options.ModelsURL != "" {
		account.Credentials["models_url"] = options.ModelsURL
	}

	accountTestService := service.NewAccountTestService(
		nil, nil, nil, nil, nil, upstream, cfg, nil, nil, nil,
	)
	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()
	startedAt := time.Now()
	models, err := accountTestService.FetchUpstreamSupportedModels(ctx, account)
	if err != nil {
		var syncErr *service.UpstreamModelSyncError
		if errors.As(err, &syncErr) {
			return errors.New(syncErr.SafeMessage())
		}
		return err
	}

	encoder := json.NewEncoder(output)
	if options.Pretty {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(smokeResult{
		Protocol:  options.Protocol,
		BaseURL:   options.BaseURL,
		ModelsURL: options.ModelsURL,
		Proxy:     options.ProxyURL != "",
		Count:     len(models),
		Models:    models,
		ElapsedMS: time.Since(startedAt).Milliseconds(),
	})
}

func parseSmokeOptions(args []string, output io.Writer) (smokeOptions, error) {
	options := smokeOptions{}
	flags := flag.NewFlagSet("model-sync-smoke", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&options.Protocol, "protocol", "", "Custom protocol: openai_responses, openai_chat_completions, openai_embeddings, anthropic_messages, or gemini")
	flags.StringVar(&options.BaseURL, "base-url", "", "Custom provider Base URL")
	flags.StringVar(&options.ModelsURL, "models-url", "", "Optional explicit model-list URL")
	flags.StringVar(&options.ProxyURL, "proxy-url", "", "Optional HTTP, HTTPS, SOCKS5, or SOCKS5H proxy URL")
	flags.StringVar(&options.APIKeyEnv, "api-key-env", defaultAPIKeyEnv, "Environment variable containing the upstream API key")
	flags.DurationVar(&options.Timeout, "timeout", 30*time.Second, "Request timeout")
	flags.BoolVar(&options.AllowHTTP, "allow-http", false, "Allow an insecure http:// URL for local testing")
	flags.BoolVar(&options.Pretty, "pretty", true, "Pretty-print JSON output")
	if err := flags.Parse(args); err != nil {
		return options, err
	}
	options.Protocol = strings.TrimSpace(options.Protocol)
	options.BaseURL = strings.TrimSpace(options.BaseURL)
	options.ModelsURL = strings.TrimSpace(options.ModelsURL)
	options.ProxyURL = strings.TrimSpace(options.ProxyURL)
	options.APIKeyEnv = strings.TrimSpace(options.APIKeyEnv)
	if !isSupportedCustomProtocol(options.Protocol) {
		return options, fmt.Errorf("unsupported or missing protocol %q", options.Protocol)
	}
	if options.BaseURL == "" {
		return options, errors.New("base-url is required")
	}
	if options.APIKeyEnv == "" {
		return options, errors.New("api-key-env is required")
	}
	if options.Timeout <= 0 {
		return options, errors.New("timeout must be greater than zero")
	}
	return options, nil
}

func isSupportedCustomProtocol(protocol string) bool {
	switch protocol {
	case service.CustomProtocolOpenAIResponses,
		service.CustomProtocolOpenAIChatCompletions,
		service.CustomProtocolOpenAIEmbeddings,
		service.CustomProtocolAnthropicMessages,
		service.CustomProtocolGemini:
		return true
	default:
		return false
	}
}
