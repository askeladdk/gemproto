// Package gemproto provides client and server implementations
// for the Gemini protocol.
package gemproto

import (
	"context"
	"crypto/tls"
	"io"
	"net/url"
)

// Gemini status codes as described in the specification.
// See: https://gemini.circumlunar.space/docs/specification.gmi
const (
	StatusInput                          = 10
	StatusSensitiveInput                 = 11
	StatusOK                             = 20
	StatusTemporaryRedirect              = 30
	StatusPermanentRedirect              = 31
	StatusTemporaryFailure               = 40
	StatusServerUnavailable              = 41
	StatusCGIError                       = 42
	StatusProxyError                     = 43
	StatusSlowDown                       = 44
	StatusPermanentFailure               = 50
	StatusNotFound                       = 51
	StatusGone                           = 52
	StatusProxyRequestRefused            = 53
	StatusBadRequest                     = 59
	StatusClientCertificateRequired      = 60
	StatusClientCertificateNotAuthorized = 61
	StatusClientCertificateNotValid      = 62
)

// Request represents a request that has been received by the server.
type Request struct {
	// URL is the url requested by the client.
	URL *url.URL

	// RequestURI is set by Server and holds the raw URL requested by the client.
	RequestURI string

	// RemoteAddr is set by Server and holds the remote address of the client.
	RemoteAddr string

	// Host is the Server Name Indication (SNI) passed by the client.
	// It is automatically set by Server when it receives a request.
	// It must be set manually to use SNI in Client requests,
	// otherwise it defaults to URL.Host.
	Host string

	// TLS holds the basic TLS connection details.
	TLS *tls.ConnectionState

	ctx context.Context
}

// NewRequestWithContext creates a new request with a context.
func NewRequestWithContext(ctx context.Context, rawURL string) (*Request, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "" {
		u.Scheme = "gemini"
	}

	return &Request{
		URL:  u,
		Host: u.Host,
		ctx:  ctx,
	}, nil
}

// NewRequest creates a new request with the default context.
func NewRequest(rawURL string) (*Request, error) {
	return NewRequestWithContext(context.Background(), rawURL)
}

// Context returns the request context.
func (r *Request) Context() context.Context {
	return r.ctx
}

// GetInput returns the unescaped query string.
func (r *Request) GetInput() (string, bool) {
	if rq := r.URL.RawQuery; rq != "" {
		qu, err := url.QueryUnescape(rq)
		return qu, err == nil
	}
	return "", false
}

// Response is the response received from a server.
type Response struct {
	// StatusCode is the response code.
	StatusCode int

	// Meta is the response metadata.
	// It is interpreted differently depending on the status code.
	Meta string

	// Body is the request body.
	// It is never nil and must be Closed.
	Body io.ReadCloser

	// TLS holds the basic TLS connection details.
	TLS *tls.ConnectionState
}
