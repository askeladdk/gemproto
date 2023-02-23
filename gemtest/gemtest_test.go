package gemtest_test

import (
	"testing"

	"github.com/askeladdk/gemproto"
	"github.com/askeladdk/gemproto/gemtest"
)

func TestServer(t *testing.T) {
	handler := func(w gemproto.ResponseWriter, r *gemproto.Request) {}

	server := gemtest.NewServer(gemproto.HandlerFunc(handler))
	defer server.Close()

	client := gemproto.Client{}

	res, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != gemproto.StatusOK {
		t.Fatal(res.StatusCode)
	}
}

func TestResponseRecorder(t *testing.T) {
	h := gemproto.HandlerFunc(func(w gemproto.ResponseWriter, r *gemproto.Request) {
		w.WriteHeader(gemproto.StatusOK, "text/plain")
		_, _ = w.Write([]byte("hello world"))
	})

	w := gemtest.NewRecorder()
	r := gemtest.NewRequest("/")
	h.ServeGemini(w, r)

	if w.Code != gemproto.StatusOK {
		t.Error(w.Code)
	}

	if w.Meta != "text/plain" {
		t.Error(w.Meta)
	}

	if s := w.Body.String(); s != "hello world" {
		t.Error(s)
	}
}
