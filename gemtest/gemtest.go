// Package gemtest contains utilities for writing tests.
package gemtest

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509/pkix"
	"net"
	"time"

	"github.com/askeladdk/gemproto"
	"github.com/askeladdk/gemproto/gemcert"
	"github.com/askeladdk/gemproto/gemtext"
)

// Server is a temporary server suitable for writing tests.
type Server struct {
	// Certificate is the temporary certificate.
	Certificate tls.Certificate

	// URL is the base URL to the server.
	URL string

	srv    *gemproto.Server
	cancel context.CancelFunc
}

// Close stops the server.
func (srv *Server) Close() error {
	srv.cancel()
	return nil
}

// NewServer creates a server initialized with a temporary certificate.
// The server runs in a separate goroutine and listens on localhost.
// Call Close() to stop the server.
func NewServer(h gemproto.Handler) *Server {
	cert, err := gemcert.CreateX509KeyPair(gemcert.CreateOptions{
		Duration: 1 * time.Hour,
		DNSNames: []string{"localhost"},
		Subject: pkix.Name{
			CommonName: "localhost",
		},
	})
	if err != nil {
		panic(err)
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	srv := &gemproto.Server{
		Addr:    l.Addr().String(),
		Handler: h,
		TLSConfig: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			ClientAuth:   tls.RequestClientCert,
			Certificates: []tls.Certificate{cert},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		_ = srv.Serve(ctx, l)
	}()

	_, port, _ := net.SplitHostPort(l.Addr().String())

	return &Server{
		URL:         "gemini://localhost:" + port,
		Certificate: cert,
		srv:         srv,
		cancel:      cancel,
	}
}

type ResponseRecorder struct {
	Body bytes.Buffer
	Code int
	Meta string

	wroteHeader bool
}

func NewRecorder() *ResponseRecorder {
	return &ResponseRecorder{
		Code: gemproto.StatusOK,
		Meta: gemtext.MIMEType,
	}
}

func (r *ResponseRecorder) WriteHeader(statusCode int, meta string) {
	if !r.wroteHeader {
		r.wroteHeader = true
		r.Code = statusCode
		r.Meta = meta
	}
}

func (r *ResponseRecorder) Write(p []byte) (int, error) {
	r.wroteHeader = true
	return r.Body.Write(p)
}

func NewRequest(rawURL string) *gemproto.Request {
	req, err := gemproto.NewRequest(rawURL)
	if err != nil {
		panic(err)
	}
	return req
}
