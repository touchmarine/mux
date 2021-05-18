package mux_test

import (
	"fmt"
	"github.com/touchmarine/mux"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

var handlerFactory = func(statusCode int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// WriteHeader must be before Write, otherwise it doesn't work
		w.WriteHeader(statusCode)
		if _, err := w.Write([]byte(body)); err != nil {
			panic(err)
		}
	}
}

func ExampleMux() {
	m := mux.New(http.NotFound)
	m.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "hello")
	})
	http.ListenAndServe(":8080", m)
}

func ExampleMux_RegexpHandleFunc() {
	m := mux.New(http.NotFound)
	m.RegexpHandleFunc(`/users/(?P<id>[0-9]+)$`, func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.Context().Value("id").(string))
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		fmt.Fprintf(w, "id=%d", id)

		// ...
	})
}

func ExampleMux_Mount() {
	mu := mux.New(http.NotFound)
	mu.HandleFunc("/report", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "user report")
	})

	m := mux.New(http.NotFound)
	m.Mount("/users", mu)

	http.ListenAndServe(":8080", m)
}

func TestNew(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("got no panic, want panic")
			}
		}()

		mux.New(nil)
	})

	t.Run("set", func(t *testing.T) {
		h := handlerFactory(http.StatusNotFound, "a")
		m := mux.New(h)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		m.ServeHTTP(rec, r)
		resp := rec.Result()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("got StatusCode %d, want %d", resp.StatusCode, http.StatusNotFound)
		}

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		body := string(b)
		if body != "a" {
			t.Errorf("got body %q, want a", body)
		}
	})
}

func TestHandleFunc(t *testing.T) {
	t.Run("green", func(t *testing.T) {
		cases := []struct {
			patterns []string
			path     string
		}{
			{
				[]string{"/"},
				"/",
			},
			{
				[]string{"/a"},
				"/a",
			},
			{
				[]string{"/1"},
				"/1",
			},
			{
				[]string{"/-"},
				"/-",
			},

			{
				[]string{"/a/b"},
				"/a/b",
			},

			{
				[]string{"/a", "/b"},
				"/a",
			},
			{
				[]string{"/a/b", "/a/c"},
				"/a/b",
			},

			{
				[]string{"/a/b", "/a/b/c"},
				"/a/b",
			},
			{
				[]string{"/a/b", "/a/b/c"},
				"/a/b/c",
			},
		}

		for _, c := range cases {
			t.Run(c.path, func(t *testing.T) {
				h := handlerFactory(http.StatusTeapot, c.path)
				m := mux.New(http.NotFound)
				for _, pattern := range c.patterns {
					m.HandleFunc(pattern, h)
				}

				r := httptest.NewRequest(http.MethodGet, c.path, nil)
				rec := httptest.NewRecorder()
				m.ServeHTTP(rec, r)
				resp := rec.Result()

				if resp.StatusCode != http.StatusTeapot {
					t.Errorf("got StatusCode %d, want %d", resp.StatusCode, http.StatusTeapot)
				}

				b, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}

				body := string(b)
				if body != c.path {
					t.Errorf("got body %q, want %q", body, c.path)
				}
			})
		}
	})

	t.Run("yellow", func(t *testing.T) {
		cases := []struct {
			patterns []string
			path     string
		}{
			{
				[]string{"/a"},
				"/a/",
			},

			{
				[]string{"/a?b"},
				"/a",
			},
			{
				[]string{"/a?b"},
				"/a?b",
			},
		}

		for _, c := range cases {
			t.Run(c.path, func(t *testing.T) {
				h := handlerFactory(http.StatusTeapot, c.path)
				m := mux.New(http.NotFound)
				for _, pattern := range c.patterns {
					m.HandleFunc(pattern, h)
				}

				r := httptest.NewRequest(http.MethodGet, c.path, nil)
				rec := httptest.NewRecorder()
				m.ServeHTTP(rec, r)
				resp := rec.Result()

				if resp.StatusCode == http.StatusTeapot {
					t.Errorf("got StatusCode %d, want other", resp.StatusCode)
				}

				b, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}

				body := string(b)
				if body == c.path {
					t.Errorf("got body %q, want other", body)
				}
			})
		}
	})

	t.Run("red", func(t *testing.T) {
		cases := []struct {
			name     string
			patterns []string
		}{
			{
				"empty",
				[]string{""},
			},
			{
				"/a/",
				[]string{"/a/"},
			},

			{
				"duplicate",
				[]string{"/a", "/a"},
			},
		}

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				defer func() {
					if recover() == nil {
						t.Error("got no panic, want panic")
					}
				}()

				h := handlerFactory(http.StatusTeapot, "")
				m := mux.New(http.NotFound)
				for _, pattern := range c.patterns {
					m.HandleFunc(pattern, h)
				}
			})
		}
	})
}

func TestRegexpHandleFunc(t *testing.T) {
	t.Run("green", func(t *testing.T) {
		cases := []struct {
			patterns []string
			path     string
			id       string // parameter value
		}{
			{
				[]string{"/(?P<id>.+)"},
				"/a",
				"a",
			},
			{
				[]string{"/(?P<id>.+)"},
				"/1",
				"1",
			},
			{
				[]string{"/(?P<id>.+)"},
				"/-",
				"-",
			},

			{
				[]string{"/(?P<id>[0-9])"},
				"/1",
				"1",
			},
			{
				[]string{"/(?P<id>[0-9])"},
				"/12",
				"1",
			},
			{
				[]string{"/(?P<id>[0-9]+)"},
				"/12",
				"12",
			},

			{
				[]string{"^/a$"},
				"/a",
				"<nil>",
			},
		}

		for _, c := range cases {
			t.Run(c.path, func(t *testing.T) {
				h := func(w http.ResponseWriter, r *http.Request) {
					id := fmt.Sprintf("%v", r.Context().Value("id"))
					if id != c.id {
						t.Errorf("got parameter id %s, want %s", id, c.id)
					}

					w.WriteHeader(http.StatusTeapot)
					if _, err := w.Write([]byte(c.path)); err != nil {
						panic(err)
					}
				}

				m := mux.New(http.NotFound)
				for _, pattern := range c.patterns {
					m.RegexpHandleFunc(pattern, h)
				}

				r := httptest.NewRequest(http.MethodGet, c.path, nil)
				rec := httptest.NewRecorder()
				m.ServeHTTP(rec, r)
				resp := rec.Result()

				if resp.StatusCode != http.StatusTeapot {
					t.Errorf("got StatusCode %d, want %d", resp.StatusCode, http.StatusTeapot)
				}

				b, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}

				body := string(b)
				if body != c.path {
					t.Errorf("got body %q, want %q", body, c.path)
				}
			})
		}
	})

	t.Run("yellow", func(t *testing.T) {
		cases := []struct {
			patterns []string
			path     string
		}{
			{
				[]string{"/(?P<id>[0-9])"},
				"/a",
			},
		}

		for _, c := range cases {
			t.Run(c.path, func(t *testing.T) {
				h := handlerFactory(http.StatusTeapot, c.path)
				m := mux.New(http.NotFound)
				for _, pattern := range c.patterns {
					m.RegexpHandleFunc(pattern, h)
				}

				r := httptest.NewRequest(http.MethodGet, c.path, nil)
				rec := httptest.NewRecorder()
				m.ServeHTTP(rec, r)
				resp := rec.Result()

				if resp.StatusCode == http.StatusTeapot {
					t.Errorf("got StatusCode %d, want other", resp.StatusCode)
				}

				b, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}

				body := string(b)
				if body == c.path {
					t.Errorf("got body %q, want other", body)
				}
			})
		}
	})

	t.Run("red", func(t *testing.T) {
		cases := []struct {
			name     string
			patterns []string
		}{
			{
				"invalid regexp",
				[]string{"/(?P<id>.+"},
			},

			{
				"duplicate",
				[]string{"/(?P<id>.+)", "/(?P<id>.+)"},
			},
		}

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				defer func() {
					if recover() == nil {
						t.Error("got no panic, want panic")
					}
				}()

				h := handlerFactory(http.StatusTeapot, "")
				m := mux.New(http.NotFound)
				for _, pattern := range c.patterns {
					m.RegexpHandleFunc(pattern, h)
				}

				// we need to exec request for regexp to compile
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				rec := httptest.NewRecorder()
				m.ServeHTTP(rec, r)
				rec.Result()
			})
		}
	})
}

func TestMount(t *testing.T) {
	t.Run("green", func(t *testing.T) {
		h1 := handlerFactory(http.StatusTeapot, "/a")
		m1 := mux.New(http.NotFound)
		m1.HandleFunc("/a", h1)

		h2 := handlerFactory(http.StatusTeapot, "/b")
		m2 := mux.New(http.NotFound)
		m2.HandleFunc("/b", h2)

		m1.Mount("", m2)

		paths := []string{
			"/a",
			"/b",
		}

		for _, path := range paths {
			t.Run(path, func(t *testing.T) {
				r := httptest.NewRequest(http.MethodGet, path, nil)
				rec := httptest.NewRecorder()
				m1.ServeHTTP(rec, r)
				resp := rec.Result()

				if resp.StatusCode != http.StatusTeapot {
					t.Errorf("got StatusCode %d, want %d", resp.StatusCode, http.StatusTeapot)
				}

				b, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}

				body := string(b)
				if body != path {
					t.Errorf("got body %q, want %q", body, path)
				}
			})
		}
	})

	t.Run("prefix", func(t *testing.T) {
		m0 := mux.New(http.NotFound)

		h1 := handlerFactory(http.StatusTeapot, "/a/a")
		m1 := mux.New(http.NotFound)
		m1.HandleFunc("/a", h1)

		h2 := handlerFactory(http.StatusTeapot, "/b/b")
		m2 := mux.New(http.NotFound)
		m2.HandleFunc("/b", h2)

		m0.Mount("/a", m1)
		m0.Mount("/b", m2)

		paths := []string{
			"/a/a",
			"/b/b",
		}

		for _, path := range paths {
			t.Run(path, func(t *testing.T) {
				r := httptest.NewRequest(http.MethodGet, path, nil)
				rec := httptest.NewRecorder()
				m0.ServeHTTP(rec, r)
				resp := rec.Result()

				if resp.StatusCode != http.StatusTeapot {
					t.Errorf("got StatusCode %d, want %d", resp.StatusCode, http.StatusTeapot)
				}

				b, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}

				body := string(b)
				if body != path {
					t.Errorf("got body %q, want %q", body, path)
				}
			})
		}
	})

	t.Run("duplicate patterns", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("got no panic, want panic")
			}
		}()

		h1 := handlerFactory(http.StatusTeapot, "/a")
		m1 := mux.New(http.NotFound)
		m1.HandleFunc("/a", h1)

		h2 := handlerFactory(http.StatusTeapot, "/a")
		m2 := mux.New(http.NotFound)
		m2.HandleFunc("/a", h2)

		m1.Mount("", m2)
	})
}
