// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This code is adapted from:
// https://cs.opensource.google/go/go/+/refs/tags/go1.19.2:src/net/http/server.go

package gemproto

import (
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
)

type muxEntry struct {
	pattern string
	handler Handler
}

// ServeMux is an Gemini request multiplexer.
// It matches the URL of each incoming request against a list of registered
// patterns and calls the handler for the pattern that
// most closely matches the URL.
//
// It functions just like http.ServeMux.
type ServeMux struct {
	exact    map[string]muxEntry
	entries  []muxEntry
	hosts    bool
	notFound Handler
	mu       sync.RWMutex
}

// NewServeMux returns a fresh ServeMux.
func NewServeMux() *ServeMux {
	return &ServeMux{
		notFound: HandlerFunc(NotFound),
	}
}

// Handler returns the handler to use for the given request,
// consulting r.Host and r.URL.Path. It always returns
// a non-nil handler. If the path is not in its canonical form, the
// handler will be an internally-generated handler that redirects
// to the canonical path. If the host contains a port, it is ignored
// when matching handlers.
//
// Handler also returns the registered pattern that matches the
// request or, in the case of internally-generated redirects,
// the pattern that will match after following the redirect.
//
// If there is no registered handler that applies to the request,
// Handler returns the handler set by NotFound.
func (mux *ServeMux) Handler(r *Request) (handler Handler, pattern string) {
	if r.URL.Scheme != "gemini" {
		return mux.notFound, ""
	}

	host, _ := splitHostPort(r.Host)
	path := cleanPath(r.URL.Path)

	if mux.shouldRedirect(host, path) {
		u := url.URL{Path: path + "/", RawQuery: r.URL.RawQuery}
		return RedirectHandler(u.String(), StatusPermanentRedirect), u.Path
	}

	if path != r.URL.Path {
		_, pattern = mux.handler(host, path)
		u := url.URL{Path: path, RawQuery: r.URL.RawQuery}
		return RedirectHandler(u.String(), StatusPermanentRedirect), pattern
	}

	return mux.handler(host, path)
}

// NotFound sets the handler to use when a requested resource is not found.
// It defaults to the NotFound function.
func (mux *ServeMux) NotFound(h HandlerFunc) {
	mux.mu.Lock()
	defer mux.mu.Unlock()
	mux.notFound = h
}

// Handle registers the handler for the given pattern.
// If a handler already exists for pattern, Handle panics.
func (mux *ServeMux) Handle(pattern string, handler Handler) {
	mux.mu.Lock()
	defer mux.mu.Unlock()

	if pattern == "" {
		panic("gemproto: empty pattern")
	} else if handler == nil {
		panic("gemproto: nil handler")
	} else if _, exist := mux.exact[pattern]; exist {
		panic("gemproto: multiple registrations for " + pattern)
	}

	if mux.exact == nil {
		mux.exact = make(map[string]muxEntry)
	}

	entry := muxEntry{pattern, handler}

	mux.exact[pattern] = entry

	if pattern[len(pattern)-1] == '/' {
		mux.entries = appendSorted(mux.entries, entry)
	}

	mux.hosts = mux.hosts || pattern[0] != '/'
}

// HandleFunc registers the handler function for the given pattern.
func (mux *ServeMux) HandleFunc(pattern string, handler HandlerFunc) {
	mux.Handle(pattern, handler)
}

// Mount attaches a handler as a subrouter along a routing path.
// The prefixed pattern is stripped from the route.
func (mux *ServeMux) Mount(pattern string, handler Handler) {
	if len(pattern) > 0 && pattern[0] == '/' {
		mux.Handle(pattern, StripPrefix(strings.TrimSuffix(pattern, "/"), handler))
	} else {
		mux.Handle(pattern, handler)
	}
}

// Route creates a fresh ServeMux and attaches it along the routing path.
func (mux *ServeMux) Route(pattern string, fn func(*ServeMux)) {
	mux2 := NewServeMux()
	fn(mux2)
	mux.Mount(pattern, mux2)
}

// ServeGemini implements Handler.
func (mux *ServeMux) ServeGemini(w ResponseWriter, r *Request) {
	h, _ := mux.Handler(r)
	h.ServeGemini(w, r)
}

func (mux *ServeMux) handler(host, path string) (h Handler, pattern string) {
	mux.mu.RLock()
	defer mux.mu.RUnlock()

	// Host-specific pattern takes precedence over generic ones
	if mux.hosts {
		h, pattern = mux.match(host + path)
	}
	if h == nil {
		h, pattern = mux.match(path)
	}
	if h == nil {
		h, pattern = mux.notFound, ""
	}
	if h == nil {
		h, pattern = HandlerFunc(NotFound), ""
	}
	return h, pattern
}

func (mux *ServeMux) match(path string) (h Handler, pattern string) {
	if e, ok := mux.exact[path]; ok {
		return e.handler, e.pattern
	}

	// Check for longest valid match. mux.entries contains all patterns
	// that end in / sorted from longest to shortest.
	for _, entry := range mux.entries {
		if strings.HasPrefix(path, entry.pattern) {
			return entry.handler, entry.pattern
		}
	}

	return nil, ""
}

func (mux *ServeMux) shouldRedirect(host, path string) bool {
	if _, exists := mux.exact[path]; exists {
		return false
	} else if _, exists := mux.exact[host+path]; exists {
		return false
	} else if len(path) == 0 {
		return false
	} else if _, exists := mux.exact[path+"/"]; exists {
		return path[len(path)-1] != '/'
	} else if _, exists := mux.exact[host+path+"/"]; exists {
		return path[len(path)-1] != '/'
	}
	return false
}

// cleanPath returns the canonical path for p, eliminating . and .. elements.
func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	np := path.Clean(p)
	// path.Clean removes trailing slash except for root;
	// put the trailing slash back if necessary.
	if p[len(p)-1] == '/' && np != "/" {
		// Fast path for common case of p being the string we want:
		if len(p) == len(np)+1 && strings.HasPrefix(p, np) {
			np = p
		} else {
			np += "/"
		}
	}
	return np
}

// appendSorted inserts e in es while keeping es alphabetically sorted by pattern.
func appendSorted(es []muxEntry, e muxEntry) []muxEntry {
	n := len(es)
	i := sort.Search(n, func(i int) bool {
		return len(es[i].pattern) < len(e.pattern)
	})
	if i == n {
		return append(es, e)
	}
	es = append(es, muxEntry{})
	copy(es[i+1:], es[i:])
	es[i] = e
	return es
}
