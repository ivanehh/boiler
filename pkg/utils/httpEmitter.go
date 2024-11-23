package utils

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	u "net/url"

	"golang.org/x/net/context"
)

const MeasurementsEP = "/measurement/gradeab"

type HTTPEmitter struct {
	url     u.URL
	payload any
	ctx     context.Context
	client  *http.Client
}

type HTTPEmitterOpt func(*HTTPEmitter) error

func WithHTTPS(host string, path string) HTTPEmitterOpt {
	return func(h *HTTPEmitter) error {
		h.url.Host = host
		h.url.Scheme = "https"
		h.url.Path = path
		return nil
	}
}

func WithHTTP(host string, path string) HTTPEmitterOpt {
	return func(h *HTTPEmitter) error {
		h.url.Host = host
		h.url.Scheme = "http"
		h.url.Path = path
		return nil
	}
}

func WithPort(port string) HTTPEmitterOpt {
	return func(h *HTTPEmitter) error {
		h.url.Host = h.url.Host + ":" + port
		return nil
	}
}

func WithContext(ctx context.Context) HTTPEmitterOpt {
	return func(h *HTTPEmitter) error {
		h.ctx = ctx
		return nil
	}
}

func WithProvidedHttpClient(c *http.Client) HTTPEmitterOpt {
	return func(h *HTTPEmitter) error {
		h.client = c
		return nil
	}
}

func NewRequestEmitter(opts ...HTTPEmitterOpt) *HTTPEmitter {
	h := &HTTPEmitter{
		url:     u.URL{},
		payload: nil,
		client:  &http.Client{},
		ctx:     context.TODO(),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func (e *HTTPEmitter) ChangePath(p string) *HTTPEmitter {
	e.url.Path = p
	return e
}

func (e *HTTPEmitter) Post(b []byte) error {
	resp, err := e.client.Post(e.url.String(), "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		targetBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to post data; response:%+v", resp.StatusCode, string(targetBody))
	}
	return nil
}

func (e *HTTPEmitter) Get() (*http.Response, error) {
	return e.client.Get(e.url.String())
}
