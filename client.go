// Package esms is the official Go SDK for the eSMS Africa SMS API.
//
// Send SMS across African countries, track delivery, schedule messages, and
// check your balance.
//
//	client := esms.New("esms_live_...")
//	res, err := client.Messages.Send(ctx, esms.SendParams{
//	    To:   "+256700000000",
//	    Text: "Hello!",
//	})
package esms

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the production API endpoint.
	DefaultBaseURL = "https://sms.esmsafrica.io/api"
	version        = "1.0.0"
)

// Client is the eSMS Africa API client. Create one with New.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	maxRetries int

	// Messages sends and manages SMS messages.
	Messages *MessagesService
	// Balance reads the account balance.
	Balance *BalanceService
	// Routes lists available routes and pricing.
	Routes *RoutesService
	// Verify sends and checks one-time verification codes (managed OTP).
	Verify *VerifyService
	// OptOuts manages the opt-out (STOP / DND) list.
	OptOuts *OptOutsService
}

// Option customises a Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient supplies a custom *http.Client (timeouts, proxies, transport).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.httpClient = h }
}

// WithMaxRetries sets how many times transient failures (network, 429, 5xx)
// are retried with backoff. Default 2.
func WithMaxRetries(n int) Option {
	return func(c *Client) { c.maxRetries = n }
}

// New creates a Client. The apiKey is your esms_live_... or esms_test_... key.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		maxRetries: 2,
	}
	for _, o := range opts {
		o(c)
	}
	c.Messages = &MessagesService{c: c}
	c.Balance = &BalanceService{c: c}
	c.Routes = &RoutesService{c: c}
	c.Verify = &VerifyService{c: c}
	c.OptOuts = &OptOutsService{c: c}
	return c
}

// BaseURL returns the URL requests are sent to.
func (c *Client) BaseURL() string { return c.baseURL }

// VerifyWebhook reports whether an incoming webhook's HMAC-SHA256 signature is
// valid (constant-time). Pass the exact raw request body, the X-Webhook-Signature
// header (e.g. "sha256=..."), and your signing secret.
func VerifyWebhook(rawBody []byte, signature, secret string) bool {
	if signature == "" || secret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	if c.apiKey == "" {
		return &Error{Message: "an API key is required; create one under Developers -> API Keys"}
	}

	full := c.baseURL + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}

	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return &Error{Message: "failed to encode request: " + err.Error(), Err: err}
		}
		payload = b
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, full, reader)
		if err != nil {
			return &Error{Message: "failed to build request: " + err.Error(), Err: err}
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "esms-go/"+version)
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < c.maxRetries {
				sleep(ctx, backoff(attempt, ""))
				continue
			}
			return &Error{Message: "could not reach the eSMS API: " + err.Error(), Err: err}
		}

		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		requestID := resp.Header.Get("X-Request-Id")

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if out != nil && len(raw) > 0 {
				if err := json.Unmarshal(raw, out); err != nil {
					return &Error{Status: resp.StatusCode, Message: "failed to decode response: " + err.Error(), Err: err}
				}
			}
			return nil
		}

		var parsed any
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &parsed)
			if parsed == nil {
				parsed = string(raw)
			}
		}

		if (resp.StatusCode == 429 || resp.StatusCode >= 500) && attempt < c.maxRetries {
			lastErr = errorFromResponse(resp.StatusCode, parsed, requestID)
			sleep(ctx, backoff(attempt, resp.Header.Get("Retry-After")))
			continue
		}
		return errorFromResponse(resp.StatusCode, parsed, requestID)
	}
	if e, ok := lastErr.(*Error); ok {
		return e
	}
	return &Error{Message: "request failed", Err: lastErr}
}

func backoff(attempt int, retryAfter string) time.Duration {
	if retryAfter != "" {
		if secs, err := strconv.Atoi(retryAfter); err == nil {
			d := time.Duration(secs) * time.Second
			if d > 20*time.Second {
				d = 20 * time.Second
			}
			return d
		}
	}
	base := math.Min(0.5*math.Pow(2, float64(attempt)), 10)
	jitter := base * 0.2 * rand.Float64()
	return time.Duration((base + jitter) * float64(time.Second))
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func fmtInt(i int) string { return fmt.Sprintf("%d", i) }
