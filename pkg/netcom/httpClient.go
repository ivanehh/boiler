package netcom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// RequestOption defines a function that modifies a request
type RequestOption func(*http.Request) error

// ClientOption defines a function that modifies the client
type ClientOption func(*Client)

// Client represents an HTTP client with configurable options
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	/*
	   Headers set on the client level are applied to every request originating from the client

	   Request headers may overwrite Client headers
	*/
	Headers http.Header
}

// NewClient creates a new HTTP client with the given options
func NewClient(options ...ClientOption) *Client {
	client := &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		Headers: make(http.Header),
	}

	for _, option := range options {
		option(client)
	}

	return client
}

// WithBaseURL sets the base URL for the client
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		if u, err := url.Parse(baseURL); err == nil {
			c.baseURL = u
		}
	}
}

// WithTimeout sets the timeout for the client
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithContext adds a context to the request
func WithContext(ctx context.Context) RequestOption {
	return func(req *http.Request) error {
		*req = *req.WithContext(ctx)
		return nil
	}
}

// WithHeader adds a header to the request
func WithHeader(key, value string) RequestOption {
	return func(req *http.Request) error {
		req.Header.Add(key, value)
		return nil
	}
}

var ErrBadParameters = errors.New("bad parameters provided")

// WithQueryParam adds a query parameter to the request
func WithQueryParams(pairs ...string) RequestOption {
	return func(req *http.Request) error {
		if len(pairs)%2 != 0 {
			return fmt.Errorf("%w:pairs of keys and values must be provided", ErrBadParameters)
		}

		q := req.URL.Query()
		for idx, item := range pairs {
			if idx == 0 {
				continue
			}
			if idx%2 == 0 {
				q.Add(pairs[idx-1], item)
			}
		}
		req.URL.RawQuery = q.Encode()
		return nil
	}
}

// resolveURL resolves a URL against the base URL
func (c *Client) resolveURL(path string) (*url.URL, error) {
	if c.baseURL == nil {
		return url.Parse(path)
	}
	return c.baseURL.ResolveReference(&url.URL{Path: path}), nil
}

// newRequest creates a new HTTP request
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader, options ...RequestOption) (*http.Request, error) {
	u, err := c.resolveURL(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Apply default headers
	for key, values := range c.Headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Apply request options
	for _, option := range options {
		if err := option(req); err != nil {
			return nil, fmt.Errorf("failed to apply request option: %w", err)
		}
	}

	return req, nil
}

// Do sends an HTTP request and returns an HTTP response
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	return resp, nil
}

// Request sends an HTTP request with the given method, path, body, and options
func (c *Client) Request(ctx context.Context, method, path string, body io.Reader, options ...RequestOption) (*http.Response, error) {
	req, err := c.newRequest(ctx, method, path, body, options...)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Get sends a GET request
func (c *Client) Get(ctx context.Context, path string, options ...RequestOption) (*http.Response, error) {
	return c.Request(ctx, http.MethodGet, path, nil, options...)
}

// Post sends a POST request with the given body
func (c *Client) Post(ctx context.Context, path string, body io.Reader, options ...RequestOption) (*http.Response, error) {
	if options == nil {
		options = []RequestOption{WithHeader("Content-Type", "application/json")}
	}
	return c.Request(ctx, http.MethodPost, path, body, options...)
}

// PostJSON sends a POST request with the given JSON body
func (c *Client) PostJSON(ctx context.Context, path string, data interface{}, options ...RequestOption) (*http.Response, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Add content-type header if not already present
	hasContentType := false
	for _, opt := range options {
		// This is a simplistic check and might not catch all cases
		if fmt.Sprintf("%v", opt) == fmt.Sprintf("%v", WithHeader("Content-Type", "application/json")) {
			hasContentType = true
			break
		}
	}

	if !hasContentType {
		options = append(options, WithHeader("Content-Type", "application/json"))
	}

	return c.Post(ctx, path, bytes.NewReader(jsonData), options...)
}

// Put sends a PUT request with the given body
func (c *Client) Put(ctx context.Context, path string, body io.Reader, options ...RequestOption) (*http.Response, error) {
	return c.Request(ctx, http.MethodPut, path, body, options...)
}

// Delete sends a DELETE request
func (c *Client) Delete(ctx context.Context, path string, options ...RequestOption) (*http.Response, error) {
	return c.Request(ctx, http.MethodDelete, path, nil, options...)
}

// Patch sends a PATCH request with the given body
func (c *Client) Patch(ctx context.Context, path string, body io.Reader, options ...RequestOption) (*http.Response, error) {
	return c.Request(ctx, http.MethodPatch, path, body, options...)
}

// DecodeResponse decodes the response body into the given value
func DecodeResponse(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read error response body: %w", err)
		}
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if v == nil {
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(v)
}

// ReadResponseBody reads the response body and returns it as a string
func ReadResponseBody(resp *http.Response) (string, error) {
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(bodyBytes), nil
}
