package gemproto_test

import (
	_ "embed"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/askeladdk/gemproto"
	"github.com/askeladdk/gemproto/gemtest"
	"github.com/askeladdk/gemproto/gemtext"
)

func TestClientGet(t *testing.T) {
	client := gemproto.Client{}

	handler := func(w gemproto.ResponseWriter, r *gemproto.Request) {
		_, _ = w.Write([]byte("hello world"))
	}

	server := gemtest.NewServer(gemproto.HandlerFunc(handler))
	defer server.Close()

	res, err := client.Get(server.URL)
	requireNoError(t, err)

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	requireNoError(t, err)

	assertEqual(t, body, []byte("hello world"))
	assertEqual(t, gemproto.StatusOK, res.StatusCode)
	assertEqual(t, gemtext.MIMEType, res.Meta)
}

func TestClientRedirect(t *testing.T) {
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
		assertEqual(t, server.URL+"/a", redirerr.LastURL)
		assertEqual(t, server.URL+"/", redirerr.NextURL)
		return
	}

	t.Fatal()
}
