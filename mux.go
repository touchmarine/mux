// Package mux is a stripped-down http.ServeMux with a few select extras like
// regular expression patterns and mounting.
//
// mux supports only http.HandlerFunc, http.Handler is not supported.
// Non-regexp handler pattern must begin with a slash "/" and must not end with
// a slash "/".
// Requests with a trailing slash are redirected to the slash-less version.
package mux

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

// Mux is an HTTP request multiplexer.
// It matches the URL of each incoming request against a list of registered
// patterns and calls the handler for the pattern that matches. It calls
// notFound if pattern does not match.
type Mux struct {
	mu       sync.RWMutex
	m        map[string]muxEntry
	notFound http.HandlerFunc
}

type muxEntry struct {
	handler http.HandlerFunc
	regexp  bool // whether pattern is an regular expression
}

// New allocates and returns a new Mux.
func New(notFound http.HandlerFunc) *Mux {
	if notFound == nil {
		panic("mux: nil notFound")
	}
	return &Mux{notFound: notFound}
}

// Mount submux into mux with prefix added to submux's patterns.
func (mux *Mux) Mount(prefix string, submux *Mux) {
	for pattern, e := range submux.m {
		mux.HandleFunc(prefix+pattern, e.handler)
	}
}

// HandleFunc registers the handler function for the given pattern.
func (mux *Mux) HandleFunc(pattern string, handler http.HandlerFunc) {
	mux.register(pattern, handler, false)
}

// RegexpHandleFunc registers the handler function for the given regular
// expression pattern.
func (mux *Mux) RegexpHandleFunc(pattern string, handler http.HandlerFunc) {
	mux.register(pattern, handler, true)
}

// register the handler for the given pattern.
// Panics if a handler already exists for pattern.
func (mux *Mux) register(pattern string, handler http.HandlerFunc, regexp bool) {
	mux.mu.Lock()
	defer mux.mu.Unlock()

	if pattern == "" {
		panic("mux: invalid pattern")
	}
	if !regexp && pattern != "/" {
		if pattern[0] != '/' {
			panic("mux: pattern must begin with \"/\"")
		}
		if pattern[len(pattern)-1] == '/' {
			panic("mux: pattern must not end with \"/\"")
		}
	}
	if handler == nil {
		panic("mux: nil handler")
	}
	if _, ok := mux.m[pattern]; ok {
		panic("mux: multiple registrations for " + pattern)
	}

	if mux.m == nil {
		mux.m = make(map[string]muxEntry)
	}

	e := muxEntry{handler, regexp}
	mux.m[pattern] = e
}

// ServeHTTP dispatches the request to the handler whose pattern most closely
// matches the request URL.
func (mux *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.RequestURI == "*" {
		if r.ProtoAtLeast(1, 1) {
			w.Header().Set("Connection", "close")
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	mux.mu.RLock()
	defer mux.mu.RUnlock()

	for pattern, e := range mux.m {
		if u, ok := urlWithoutSlash(r.URL.Path, pattern, r.URL); ok {
			http.Redirect(w, r, u.String(), http.StatusPermanentRedirect)
			return
		}

		// CONNECT requests are not canonicalized.
		if r.Method != http.MethodConnect {
			if !isLower(r.URL.Path) {
				lowerURL := strings.ToLower(r.URL.String())
				http.Redirect(w, r, lowerURL, http.StatusPermanentRedirect)
				return
			}
		}

		if e.regexp {
			re := regexp.MustCompile(pattern)
			if re.MatchString(r.URL.Path) {
				addRegexpSubmatchesToContext(e.handler, re)(w, r)
				return
			}
		} else {
			if r.URL.Path == pattern {
				e.handler(w, r)
				return
			}
		}
	}

	mux.notFound(w, r)
}

// urlWithoutSlash determines if the given path needs removing "/" from it. If
// the path needs removing, it creates a new URL, setting the path to
// u.Path - "/" and returning true to indicate so.
func urlWithoutSlash(path, pattern string, u *url.URL) (*url.URL, bool) {
	re := regexp.MustCompile(pattern)
	if lastIndex := len(path) - 1; path[lastIndex] == '/' && (path[:lastIndex] == pattern ||
		re.MatchString(path[:lastIndex])) {
		u := &url.URL{Path: path[:lastIndex], RawQuery: u.RawQuery}
		return u, true
	}
	return u, false
}

// isLower determines if s is lower case.
func isLower(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && !unicode.IsLower(r) {
			return false
		}
	}
	return true
}

// addRegexpSubmatchesToContext adds regexp submatches from the provided re to
// r.Context().
func addRegexpSubmatchesToContext(next http.HandlerFunc, re *regexp.Regexp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// And named regexp submatches to request context.
		submatches := re.FindStringSubmatch(r.URL.Path)
		for i, name := range re.SubexpNames() {
			if i == 0 || name == "" {
				continue
			}
			r = r.WithContext(context.WithValue(r.Context(), name, submatches[i]))
		}
		next(w, r)
	}
}
