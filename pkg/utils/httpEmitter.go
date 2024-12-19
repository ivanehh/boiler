package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	u "net/url"
	"time"
)

// HTTPEmitter defines the contract for HTTP operations
type HTTPEmitter interface {
	Do(ctx context.Context, method string, body []byte) (*http.Response, error)
	SetHeader(key, value string) HTTPEmitter
	SetQueryParam(key, value string) HTTPEmitter
	ChangePath(path string) HTTPEmitter
}

// httpEmitter implements HTTPEmitterInterface
type httpEmitter struct {
	url     u.URL
	headers map[string]string
	client  *http.Client
}

// Custom errors for better error handling
var (
	ErrInvalidStatus = fmt.Errorf("invalid status code received")
	ErrTimeout       = fmt.Errorf("request timed out")
	ErrCanceled      = fmt.Errorf("request was canceled")
)

type HTTPEmitterOpt func(*httpEmitter) error

func WithHTTPS(host string, path string) HTTPEmitterOpt {
	return func(h *httpEmitter) error {
		h.url.Host = host
		h.url.Scheme = "https"
		h.url.Path = path
		return nil
	}
}

func WithHTTP(host string, path string) HTTPEmitterOpt {
	return func(h *httpEmitter) error {
		h.url.Host = host
		h.url.Scheme = "http"
		h.url.Path = path
		return nil
	}
}

func WithTimeout(timeout time.Duration) HTTPEmitterOpt {
	return func(h *httpEmitter) error {
		h.client.Timeout = timeout
		return nil
	}
}

func WithRedirectFunc(redirectFunc func(req *http.Request, via []*http.Request) error) HTTPEmitterOpt {
	return func(h *httpEmitter) error {
		h.client.CheckRedirect = redirectFunc
		return nil
	}
}

func WithProvidedHttpClient(c *http.Client) HTTPEmitterOpt {
	return func(h *httpEmitter) error {
		h.client = c
		return nil
	}
}

func NewRequestEmitter(opts ...HTTPEmitterOpt) *httpEmitter {
	h := &httpEmitter{
		url:     u.URL{},
		headers: make(map[string]string),
		client: &http.Client{
			Timeout: 30 * time.Second, // Default timeout
		},
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// SetHeader adds or updates a header for subsequent requests
func (e *httpEmitter) SetHeader(key, value string) HTTPEmitter {
	e.headers[key] = value
	return e
}

// SetQueryParam adds a query parameter to the URL
func (e *httpEmitter) SetQueryParam(key, value string) HTTPEmitter {
	q := e.url.Query()
	q.Set(key, value)
	e.url.RawQuery = q.Encode()
	return e
}

func (e *httpEmitter) ChangePath(p string) HTTPEmitter {
	e.url.Path = p
	return e
}

// Do performs the HTTP request with the given method and body
func (e *httpEmitter) Do(ctx context.Context, method string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, e.url.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set default headers
	if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		req.Header.Set("Content-Type", "application/json")
	}

	// Set custom headers
	for k, v := range e.headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		switch err {
		case context.DeadlineExceeded:
			return nil, ErrTimeout
		case context.Canceled:
			return nil, ErrCanceled
		default:
			return nil, fmt.Errorf("executing request: %w", err)
		}
	}

	return resp, nil
}

// Post is a convenience method for POST requests
func (e *httpEmitter) Post(ctx context.Context, body []byte) error {
	resp, err := e.Do(ctx, http.MethodPost, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading error response: %w", err)
		}
		return fmt.Errorf("%w: status %d, body: %s", ErrInvalidStatus, resp.StatusCode, string(body))
	}

	return nil
}

// Get is a convenience method for GET requests
func (e *httpEmitter) Get(ctx context.Context) (*http.Response, error) {
	return e.Do(ctx, http.MethodGet, nil)
}
