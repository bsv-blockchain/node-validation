package teranode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func newHealthStub(t *testing.T) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile("testdata/health-ready.json")
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Deliberately set Content-Type: text/plain to mirror Teranode quirk.
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(body)
	}))
}

func TestHealth_ReadinessAllOK(t *testing.T) {
	srv := newHealthStub(t)
	defer srv.Close()
	h, _ := NewHealthProbe(srv.URL, nil)
	r, err := h.Readiness(context.Background())
	if err != nil {
		t.Fatalf("Readiness: %v", err)
	}
	if !r.AllOK() {
		t.Errorf("want AllOK; report=%+v", r)
	}
	if len(r.Services) != 2 {
		t.Errorf("services: %d", len(r.Services))
	}
	bv := r.Services[1]
	if len(bv.Dependencies) != 1 || bv.Dependencies[0].Resource != "CatchupStatus" {
		t.Errorf("dep parse: %+v", bv.Dependencies)
	}
	if !strings.Contains(bv.Dependencies[0].Message, "active=false") {
		t.Errorf("catchup message: %q", bv.Dependencies[0].Message)
	}
}

func TestHealth_NilOnEmptyURL(t *testing.T) {
	h, err := NewHealthProbe("", nil)
	if err != nil || h != nil {
		t.Fatalf("want (nil, nil), got (%v, %v)", h, err)
	}
}

func TestHealth_Liveness(t *testing.T) {
	srv := newHealthStub(t)
	defer srv.Close()
	h, _ := NewHealthProbe(srv.URL, nil)
	r, err := h.Liveness(context.Background())
	if err != nil {
		t.Fatalf("Liveness: %v", err)
	}
	_ = r
}

func TestHealth_AllOKFailsOnBadService(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"200","services":[{"service":"svc","status":"503","dependencies":[]}]}`))
	}))
	defer srv.Close()
	h, _ := NewHealthProbe(srv.URL, nil)
	r, err := h.Readiness(context.Background())
	if err != nil {
		t.Fatalf("Readiness: %v", err)
	}
	if r.AllOK() {
		t.Error("AllOK should be false when a service has non-200 status")
	}
}

func TestHealth_AllOKFailsOnTopLevel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"503","services":[]}`))
	}))
	defer srv.Close()
	h, _ := NewHealthProbe(srv.URL, nil)
	r, err := h.Readiness(context.Background())
	if err != nil {
		t.Fatalf("Readiness: %v", err)
	}
	if r.AllOK() {
		t.Error("AllOK should be false when top-level status is not 200")
	}
}
