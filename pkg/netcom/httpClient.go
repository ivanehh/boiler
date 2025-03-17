package netcom

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"

	"golang.org/x/net/context"
)

const MeasurementsEP = "/measurement/gradeab"

type HTTPClient struct {
	sync.Mutex
	Url    *url.URL
	ctx    context.Context
	client *http.Client
}

type HTTPClientOpt func(*HTTPClient) error

func WithContext(ctx context.Context) HTTPClientOpt {
	return func(h *HTTPClient) error {
		h.ctx = ctx
		return nil
	}
}

func WithProvidedHttpClient(c *http.Client) HTTPClientOpt {
	return func(h *HTTPClient) error {
		h.client = c
		return nil
	}
}

func NewHTTPClient(opts ...HTTPClientOpt) *HTTPClient {
	h := &HTTPClient{
		Url:    &url.URL{},
		client: &http.Client{},
		ctx:    context.TODO(),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// TODO: Make this concurrently safe
func (e *HTTPClient) SetURL(url *url.URL) *HTTPClient {
	e.Lock()
	defer e.Unlock()
	e.Url = url
	return e
}

// if the provided url is nil, then the url set for the client is used
func (e *HTTPClient) Post(u *url.URL, b []byte) error {
	e.Lock()
	defer e.Unlock()
	if u == nil {
		u = e.Url
	}
	resp, err := e.client.Post(u.String(), "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		targetBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to post data;code:%v\nresponse:%s", resp.StatusCode, string(targetBody))
	}
	return nil
}

func (e *HTTPClient) Get(u *url.URL) (*http.Response, error) {
	e.Lock()
	defer e.Unlock()
	if u == nil {
		u = e.Url
	}
	return e.client.Get(u.String())
}
