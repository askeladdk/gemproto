package gemproto_test

import (
	"context"
	"crypto/tls"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/askeladdk/gemproto"
	"github.com/askeladdk/gemproto/gemcert"
	"github.com/askeladdk/gemproto/gemtest"
	"github.com/askeladdk/gemproto/internal/require"
)

func TestServer(t *testing.T) {
	t.Parallel()

	cert, err := gemcert.CreateX509KeyPair(gemcert.CreateOptions{
		Duration: 1 * time.Hour,
		DNSNames: []string{"localhost"},
		Subject: pkix.Name{
			CommonName: "localhost",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	s := gemproto.Server{
		Addr:   "127.0.0.1:0",
		Logger: log.Default(),
		TLSConfig: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			ClientAuth:   tls.RequestClientCert,
			Certificates: []tls.Certificate{cert},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		if err := s.ListenAndServe(ctx); !errors.Is(err, gemproto.ErrServerClosed) {
			t.Error(err)
		}
	}()

	<-ctx.Done()
}

func TestInsecureServer(t *testing.T) {
	t.Parallel()

	h := gemproto.HandlerFunc(func(w gemproto.ResponseWriter, r *gemproto.Request) {
		_, err := w.Write([]byte("hello world"))
		require.NoError(t, err)
	})

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := gemproto.Server{
		Addr:     l.Addr().String(),
		Handler:  h,
		Insecure: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go func() {
		require.ErrorIs(t, s.Serve(ctx, l), gemproto.ErrServerClosed)
	}()

	conn, err := net.Dial("tcp", s.Addr)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	_, err = conn.Write([]byte("/\r\n"))
	require.NoError(t, err)
	var rbuf [512]byte
	rn, err := conn.Read(rbuf[:])
	require.NoError(t, err)
	expected := []byte("20 text/gemini;charset=utf-8\r\nhello world")
	require.Equal(t, expected, rbuf[:rn])

	<-ctx.Done()
}

type mockListener struct{ err error }

func (l *mockListener) Accept() (net.Conn, error) { return nil, l.err }
func (l *mockListener) Close() error              { return nil }
func (l *mockListener) Addr() net.Addr            { return nil }

type timeoutError struct{}

func (err timeoutError) Error() string   { return "timeout error" }
func (err timeoutError) Timeout() bool   { return true }
func (err timeoutError) Temporary() bool { return true }

type mockLogger struct {
	Logs []string
}

func (l *mockLogger) Printf(f string, args ...any) {
	l.Logs = append(l.Logs, fmt.Sprintf(f, args...))
}

func TestServerBackoff(t *testing.T) {
	listener := mockListener{timeoutError{}}
	logger := mockLogger{}
	s := gemproto.Server{
		Addr:     "127.0.0.1:0",
		Logger:   &logger,
		Insecure: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		if err := s.Serve(ctx, &listener); !errors.Is(err, gemproto.ErrServerClosed) {
			t.Error(err)
		}
	}()

	<-ctx.Done()

	expected := []string{
		"gemproto: accept timeout: timeout error; retrying in 5ms",
		"gemproto: accept timeout: timeout error; retrying in 10ms",
		"gemproto: accept timeout: timeout error; retrying in 20ms",
		"gemproto: accept timeout: timeout error; retrying in 40ms",
		"gemproto: accept timeout: timeout error; retrying in 80ms",
		"gemproto: accept timeout: timeout error; retrying in 160ms",
		"gemproto: accept timeout: timeout error; retrying in 320ms",
		"gemproto: accept timeout: timeout error; retrying in 640ms",
		"gemproto: accept timeout: timeout error; retrying in 1s",
		"gemproto: accept timeout: timeout error; retrying in 1s",
	}

	require.Equal(t, expected, logger.Logs)
}

func TestServerHandshakeFail(t *testing.T) {
	t.Parallel()

	cert, err := gemcert.CreateX509KeyPair(gemcert.CreateOptions{
		Duration: 1 * time.Hour,
		DNSNames: []string{"localhost"},
		Subject: pkix.Name{
			CommonName: "localhost",
		},
	})
	require.NoError(t, err)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	logger := mockLogger{}
	s := gemproto.Server{
		Addr:   l.Addr().String(),
		Logger: &logger,
		TLSConfig: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			ClientAuth:   tls.RequestClientCert,
			Certificates: []tls.Certificate{cert},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go func() { _ = s.Serve(ctx, l) }()

	conn, err := net.Dial("tcp", s.Addr)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	_, err = conn.Write([]byte("/////////////////////////\r\n"))
	require.NoError(t, err)
	conn.Close()

	<-ctx.Done()

	expected := []string{
		"gemproto: tls handshake failed: tls: first record does not look like a TLS handshake",
	}

	require.Equal(t, expected, logger.Logs)
}

func TestServerRequestTooLong(t *testing.T) {
	s := gemtest.NewServer(nil)
	defer s.Close()
	c := gemproto.Client{}
	res, err := c.Get(s.URL + strings.Repeat("/", 2000))
	require.NoError(t, err)
	require.Equal(t, gemproto.StatusBadRequest, res.StatusCode)
	require.Equal(t, "request line too long", res.Meta)
}
