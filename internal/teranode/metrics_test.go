package teranode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func newMetricsStub(t *testing.T) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile("testdata/sample.prom")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write(body)
	}))
}

func TestMetrics_BestBlockHeight(t *testing.T) {
	srv := newMetricsStub(t)
	defer srv.Close()
	m, _ := NewMetricsScraper(srv.URL, nil)
	h, err := m.BestBlockHeight(context.Background())
	if err != nil {
		t.Fatalf("BestBlockHeight: %v", err)
	}
	if h != 12345 {
		t.Errorf("h: %d", h)
	}
}

func TestMetrics_FSMState(t *testing.T) {
	srv := newMetricsStub(t)
	defer srv.Close()
	m, _ := NewMetricsScraper(srv.URL, nil)
	st, err := m.FSMState(context.Background())
	if err != nil {
		t.Fatalf("FSMState: %v", err)
	}
	if st != 4 {
		t.Errorf("st: %d", st)
	}
}

func TestMetrics_CatchupActiveFalse(t *testing.T) {
	srv := newMetricsStub(t)
	defer srv.Close()
	m, _ := NewMetricsScraper(srv.URL, nil)
	active, err := m.CatchupActive(context.Background())
	if err != nil || active {
		t.Errorf("want active=false err=nil; got active=%v err=%v", active, err)
	}
}

func TestMetrics_HistogramSamplesPreserved(t *testing.T) {
	srv := newMetricsStub(t)
	defer srv.Close()
	m, _ := NewMetricsScraper(srv.URL, nil)
	mfs, err := m.Scrape(context.Background())
	if err != nil {
		t.Fatalf("Scrape: %v", err)
	}
	h, ok := mfs["teranode_validator_transactions_bucket"]
	if !ok {
		t.Fatal("histogram bucket family missing")
	}
	if len(h.Samples) != 3 {
		t.Errorf("want 3 bucket samples, got %d", len(h.Samples))
	}
	if h.Samples[0].Labels["le"] != "0.005" {
		t.Errorf("first bucket label: %v", h.Samples[0].Labels)
	}
}

func TestMetrics_NilOnEmptyURL(t *testing.T) {
	m, err := NewMetricsScraper("", nil)
	if err != nil || m != nil {
		t.Fatalf("want (nil, nil), got (%v, %v)", m, err)
	}
}
