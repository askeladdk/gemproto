package gemproto

import (
	urlpkg "net/url"
	"strings"
)

// Redirect responds with a 3x redirection to the given URL.
func Redirect(w ResponseWriter, r *Request, url string, code int) {
	w.WriteHeader(code, absoluteURL(r, url))
}

// RedirectHandler returns a Handler that redirects to the given URL.
func RedirectHandler(url string, code int) Handler {
	return HandlerFunc(func(w ResponseWriter, r *Request) {
		Redirect(w, r, url, code)
	})
}

// NotFound responds with 51 Not Found.
func NotFound(w ResponseWriter, r *Request) {
	w.WriteHeader(StatusNotFound, "Not Found")
}

// NotFoundHandler returns a Handler that responds with 51 Not Found.
func NotFoundHandler() Handler {
	return HandlerFunc(NotFound)
}

// StripPrefix returns a handler that serves Gemini requests by removing the
// given prefix from the request URL's Path (and RawPath if set) and invoking
// the handler h. StripPrefix handles a request for a path that doesn't begin
// with prefix by replying with 51 Not Found. The prefix must
// match exactly: if the prefix in the request contains escaped characters
// the reply is also 51 Not Found.
func StripPrefix(prefix string, h Handler) Handler {
	if prefix == "" {
		return h
	}
	return HandlerFunc(func(w ResponseWriter, r *Request) {
		p := strings.TrimPrefix(r.URL.Path, prefix)
		rp := strings.TrimPrefix(r.URL.RawPath, prefix)
		if len(p) < len(r.URL.Path) && (r.URL.RawPath == "" || len(rp) < len(r.URL.RawPath)) {
			r2 := new(Request)
			*r2 = *r
			r2.URL = new(urlpkg.URL)
			*r2.URL = *r.URL
			r2.URL.Path = p
			r2.URL.RawPath = rp
			h.ServeGemini(w, r2)
			return
		}

		NotFound(w, r)
	})
}

// Input responds with 10 INPUT if the query string is empty.
func Input(prompt string) func(Handler) Handler {
	return func(next Handler) Handler {
		return HandlerFunc(func(w ResponseWriter, r *Request) {
			if r.URL.RawQuery == "" {
				w.WriteHeader(StatusInput, prompt)
				return
			}
			next.ServeGemini(w, r)
		})
	}
}
