package gemproto_test

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/askeladdk/gemproto"
	"github.com/askeladdk/gemproto/gemcert"
	"github.com/askeladdk/gemproto/gemtest"
	"github.com/askeladdk/gemproto/gemtext"
	"github.com/askeladdk/gemproto/internal/require"
)

func TestClientGet(t *testing.T) {
	t.Parallel()

	cert, err := gemcert.CreateX509KeyPair(gemcert.CreateOptions{})
	require.NoError(t, err)

	certfp := gemcert.Fingerprint(cert.Leaf)

	client := gemproto.Client{
		ConnectTimeout: time.Second,
		ReadTimeout:    time.Second,
		WriteTimeout:   time.Second,
		GetCertificate: gemproto.SingleClientCertificate(cert),
	}

	handler := func(w gemproto.ResponseWriter, r *gemproto.Request) {
		require.True(t, r.TLS != nil)
		require.True(t, len(r.TLS.PeerCertificates) != 0)
		require.Equal(t, certfp, gemcert.Fingerprint(r.TLS.PeerCertificates[0]))
		_, _ = w.Write([]byte("hello world"))
	}

	server := gemtest.NewServer(gemproto.HandlerFunc(handler))
	defer server.Close()

	res, err := client.Get(server.URL)
	require.NoError(t, err)

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	require.Equal(t, body, []byte("hello world"))
	require.Equal(t, gemproto.StatusOK, res.StatusCode)
	require.Equal(t, gemtext.MIMEType, res.Meta)
}

func TestClientRedirect(t *testing.T) {
	client := gemproto.Client{}

	handler := func(w gemproto.ResponseWriter, r *gemproto.Request) {
		if r.URL.Path == "/index.gmi" {
			gemproto.Redirect(w, r, "/", gemproto.StatusPermanentRedirect)
			return
		}
	}

	server := gemtest.NewServer(gemproto.HandlerFunc(handler))
	defer server.Close()

	res, err := client.Get(server.URL + "/index.gmi")
	require.NoError(t, err)
	require.Equal(t, server.URL+"/", res.URL.String())
}

func TestClientRedirectTooMany(t *testing.T) {
	client := gemproto.Client{}

	handler := func(w gemproto.ResponseWriter, r *gemproto.Request) {
		if r.URL.Path != "/" {
			i := strings.LastIndexByte(r.URL.Path, '/')
			rest := r.URL.Path[:i]
			gemproto.Redirect(w, r, rest, gemproto.StatusPermanentRedirect)
			return
		}
	}

	server := gemtest.NewServer(gemproto.HandlerFunc(handler))
	defer server.Close()

	_, err := client.Get(server.URL + "/a/b/c/d/e/f")

	var redirerr gemproto.RedirectError
	if errors.As(err, &redirerr) {
		require.Equal(t, server.URL+"/a", redirerr.LastURL)
		require.Equal(t, server.URL+"/", redirerr.NextURL)
		return
	}

	t.Fatal()
}
