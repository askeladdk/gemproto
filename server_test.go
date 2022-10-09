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

	m := gemproto.NewServeMux()
	m.HandleFunc("/", func(w gemproto.ResponseWriter, r *gemproto.Request) {
		fmt.Fprintln(w, "hello world")
	})

	s := gemproto.Server{
		Handler: m,
		Logger:  log.Default(),
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
