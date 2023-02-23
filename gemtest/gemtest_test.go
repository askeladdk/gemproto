package gemtest_test

import (
	"testing"

	"github.com/askeladdk/gemproto"
	"github.com/askeladdk/gemproto/gemtest"
	"github.com/askeladdk/gemproto/internal/require"
)

func TestServer(t *testing.T) {
	handler := func(w gemproto.ResponseWriter, r *gemproto.Request) {}

	server := gemtest.NewServer(gemproto.HandlerFunc(handler))
	defer server.Close()

	client := gemproto.Client{}

	res, err := client.Get(server.URL)

	require.NoError(t, err)
	require.Equal(t, gemproto.StatusOK, res.StatusCode)
}

func TestResponseRecorder(t *testing.T) {
	h := gemproto.HandlerFunc(func(w gemproto.ResponseWriter, r *gemproto.Request) {
		w.WriteHeader(gemproto.StatusOK, "text/plain")
		_, _ = w.Write([]byte("hello world"))
	})

	w := gemtest.NewRecorder()
	r := gemtest.NewRequest("/")
	h.ServeGemini(w, r)

	require.Equal(t, gemproto.StatusOK, w.Code)
	require.Equal(t, "text/plain", w.Meta)
	require.Equal(t, "hello world", w.Body.String())
}
