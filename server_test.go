package gemproto_test

import (
	"context"
	"crypto/tls"
	"crypto/x509/pkix"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/askeladdk/gemproto"
	"github.com/askeladdk/gemproto/gemcert"
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
		if err := s.ListenAndServe(ctx); err != gemproto.ErrServerClosed {
			t.Error(err)
		}
		fmt.Println("quitting")
	}()

	<-ctx.Done()
}

func TestInsecureServer(t *testing.T) {
	t.Parallel()

	s := gemproto.Server{
		Addr:     "127.0.0.1:0",
		Logger:   log.Default(),
		Insecure: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		if err := s.ListenAndServe(ctx); err != gemproto.ErrServerClosed {
			t.Error(err)
		}
		fmt.Println("quitting")
	}()

	<-ctx.Done()
}
