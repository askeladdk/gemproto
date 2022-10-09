package gemtest

import (
	"testing"

	"github.com/askeladdk/gemproto"
)

func TestServer(t *testing.T) {
	handler := func(w gemproto.ResponseWriter, r *gemproto.Request) {}

	server := NewServer(gemproto.HandlerFunc(handler))
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
