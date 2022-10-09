package gemproto_test

import (
	"fmt"
	"testing"

	"github.com/askeladdk/gemproto"
	"github.com/askeladdk/gemproto/gemtest"
)

func TestInput(t *testing.T) {
	t.Parallel()

	endpoint := gemproto.HandlerFunc(func(w gemproto.ResponseWriter, r *gemproto.Request) {
		name, _ := r.GetInput()
		fmt.Fprintln(w, "hello", name)
	})

	mux := gemproto.NewServeMux()
	mux.Handle("/index.gmi", gemproto.Input("your name?")(endpoint))

	w := gemtest.NewRecorder()
	r := gemtest.NewRequest("/index.gmi")
	mux.ServeGemini(w, r)
	assertEqual(t, gemproto.StatusInput, w.Code)
	assertEqual(t, "your name?", w.Meta)

	w2 := gemtest.NewRecorder()
	r2 := gemtest.NewRequest("/index.gmi?teh%20gopher")
	mux.ServeGemini(w2, r2)
	assertEqual(t, gemproto.StatusOK, w2.Code)
	assertEqual(t, "hello teh gopher\n", w2.Body.String())
}

func TestRedirect(t *testing.T) {
	t.Parallel()

	mux := gemproto.NewServeMux()
	mux.Handle("/hello.gmi", gemproto.RedirectHandler("/", gemproto.StatusPermanentRedirect))

	w := gemtest.NewRecorder()
	r := gemtest.NewRequest("gemini://example.com/hello.gmi")
	mux.ServeGemini(w, r)
	assertEqual(t, gemproto.StatusPermanentRedirect, w.Code)
	assertEqual(t, "gemini://example.com/", w.Meta)
}
