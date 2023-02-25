package gemproto_test

import (
	"embed"
	"strings"
	"testing"

	"github.com/askeladdk/gemproto"
	"github.com/askeladdk/gemproto/gemtest"
	"github.com/askeladdk/gemproto/internal/require"
)

func TestFileServerOK(t *testing.T) {
	t.Parallel()

	h := gemproto.FileServer(gemproto.Dir("."), 0)
	w := gemtest.NewRecorder()
	r := gemtest.NewRequest("/fs.go")
	h.ServeGemini(w, r)
	require.Equal(t, gemproto.StatusOK, w.Code)
	require.Equal(t, "text/x-go; charset=utf-8", w.Meta)
}

func TestFileServerNotFound(t *testing.T) {
	t.Parallel()

	h := gemproto.FileServer(gemproto.Dir("."), 0)
	w := gemtest.NewRecorder()
	r := gemtest.NewRequest("/blablabla.example")
	h.ServeGemini(w, r)
	require.Equal(t, gemproto.StatusNotFound, w.Code)
	require.Equal(t, "open blablabla.example: no such file or directory", w.Meta)
}

func TestFileServerListDirs(t *testing.T) {
	t.Parallel()

	h := gemproto.FileServer(gemproto.Dir("."), gemproto.ListDirs|gemproto.ShowHiddenFiles)
	w := gemtest.NewRecorder()
	r := gemtest.NewRequest("/")
	h.ServeGemini(w, r)
	require.Equal(t, gemproto.StatusOK, w.Code)
	lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
	require.Equal(t, "# /", lines[0])
	for _, line := range lines[1:] {
		require.True(t, strings.HasPrefix(line, "=> "), line)
	}
}

func TestFileServerRedirectIndex(t *testing.T) {
	t.Parallel()

	h := gemproto.FileServer(gemproto.Dir("."), 0)
	w := gemtest.NewRecorder()
	r := gemtest.NewRequest("gemini://localhost:1965/index.gmi")
	h.ServeGemini(w, r)
	require.Equal(t, gemproto.StatusPermanentRedirect, w.Code)
	require.Equal(t, "gemini://localhost:1965/", w.Meta)
}

//go:embed testfiles/.meta
//go:embed testfiles/hello.gmi
var testfiles embed.FS

func TestFileServerMeta1(t *testing.T) {
	t.Parallel()

	mux := gemproto.NewServeMux()
	mux.Handle("/", gemproto.FileServer(testfiles, gemproto.UseMetaFile))
	w := gemtest.NewRecorder()
	r := gemtest.NewRequest("/testfiles/hello.gmi")
	mux.ServeGemini(w, r)
	require.Equal(t, gemproto.StatusOK, w.Code)
	require.Equal(t, "text/plain", w.Meta)
	require.True(t, w.Body.String() != "")
}

func TestFileServerMeta2(t *testing.T) {
	t.Parallel()

	mux := gemproto.NewServeMux()
	mux.Handle("/", gemproto.FileServer(testfiles, gemproto.UseMetaFile))

	s := gemtest.NewServer(mux)
	defer s.Close()

	c := gemproto.Client{}
	r, err := c.Get(s.URL + "/testfiles/doesnotexist.gmi")
	require.NoError(t, err)
	require.Equal(t, gemproto.StatusOK, r.StatusCode)
	require.Equal(t, "this file does not exist", r.Meta)
}
