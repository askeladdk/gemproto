package gemproto_test

import (
	"fmt"
	"testing"

	"github.com/askeladdk/gemproto"
	"github.com/askeladdk/gemproto/gemtest"
	"github.com/askeladdk/gemproto/internal/require"
)

func TestServeMuxSNI(t *testing.T) {
	t.Parallel()

	mux := gemproto.NewServeMux()
	mux.HandleFunc("/index.gmi", func(w gemproto.ResponseWriter, r *gemproto.Request) {
		fmt.Fprintln(w, "localhost")
	})

	mux.HandleFunc("example.com/index.gmi", func(w gemproto.ResponseWriter, r *gemproto.Request) {
		fmt.Fprintln(w, "example.com")
	})

	for _, testcase := range []struct {
		Name     string
		URL      string
		Expected string
	}{
		{
			Name:     "default",
			URL:      "gemini:///index.gmi",
			Expected: "localhost\n",
		},
		{
			Name:     "example.com",
			URL:      "gemini://example.com/index.gmi",
			Expected: "example.com\n",
		},
	} {
		w := gemtest.NewRecorder()
		r := gemtest.NewRequest(testcase.URL)

		mux.ServeGemini(w, r)
		require.Equal(t, gemproto.StatusOK, w.Code)
		require.Equal(t, testcase.Expected, w.Body.String())
	}
}

func TestServeMuxMount(t *testing.T) {
	t.Parallel()

	mux := gemproto.NewServeMux()
	mux.HandleFunc("/index.gmi", func(w gemproto.ResponseWriter, r *gemproto.Request) {
		fmt.Fprintln(w, "hello")
	})

	mux2 := gemproto.NewServeMux()
	mux2.Mount("/hello/", mux)

	w := gemtest.NewRecorder()
	r := gemtest.NewRequest("/hello/index.gmi")

	mux2.ServeGemini(w, r)
	require.Equal(t, gemproto.StatusOK, w.Code)
	require.Equal(t, "hello\n", w.Body.String())
}
