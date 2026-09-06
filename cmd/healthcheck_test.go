package cmd

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRunHealthcheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	t.Setenv("DITTY_PORT", u.Port())
	t.Setenv("DITTY_BASE_PATH", "")

	if err := runHealthcheck(); err != nil {
		t.Fatalf("runHealthcheck() = %v, want nil", err)
	}
}

func TestRunHealthcheck_BasePath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/term/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	t.Setenv("DITTY_PORT", u.Port())
	t.Setenv("DITTY_BASE_PATH", "/term/")

	if err := runHealthcheck(); err != nil {
		t.Fatalf("runHealthcheck() = %v, want nil", err)
	}
}

func TestRunHealthcheck_NonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	t.Setenv("DITTY_PORT", u.Port())
	t.Setenv("DITTY_BASE_PATH", "")

	if err := runHealthcheck(); err == nil {
		t.Fatal("runHealthcheck() = nil error, want error on 503")
	}
}

func TestRunHealthcheck_Unreachable(t *testing.T) {
	t.Setenv("DITTY_PORT", "1") // nothing listens on port 1
	t.Setenv("DITTY_BASE_PATH", "")

	if err := runHealthcheck(); err == nil {
		t.Fatal("runHealthcheck() = nil error, want error when unreachable")
	}
}
