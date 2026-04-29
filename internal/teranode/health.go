package teranode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HealthProbe queries Teranode's /health/* endpoints.
// Discovery: docs/discovery.md §6.
type HealthProbe struct {
	base   *url.URL
	http   *http.Client
	logger *slog.Logger
}

func NewHealthProbe(rawURL string, logger *slog.Logger) (*HealthProbe, error) {
	if rawURL == "" {
		return nil, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("teranode health url %q: %w", rawURL, err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &HealthProbe{base: u, http: &http.Client{Timeout: 10 * time.Second}, logger: logger}, nil
}

type DependencyHealth struct {
	Resource string `json:"resource"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
	Message  string `json:"message,omitempty"`
}

type ServiceHealth struct {
	Service      string             `json:"service"`
	Status       string             `json:"status"`
	Dependencies []DependencyHealth `json:"-"`
	Raw          json.RawMessage    `json:"dependencies"`
}

type HealthReport struct {
	Status   string          `json:"status"`
	Services []ServiceHealth `json:"services"`
}

func (h *HealthProbe) Readiness(ctx context.Context) (HealthReport, error) {
	return h.fetch(ctx, "/health/readiness")
}
func (h *HealthProbe) Liveness(ctx context.Context) (HealthReport, error) {
	return h.fetch(ctx, "/health/liveness")
}

func (h *HealthProbe) fetch(ctx context.Context, path string) (HealthReport, error) {
	full := *h.base
	full.Path = strings.TrimRight(full.Path, "/") + path
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, full.String(), nil)
	resp, err := h.http.Do(req)
	if err != nil {
		return HealthReport{}, fmt.Errorf("health get %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return HealthReport{}, err
	}
	// Body is JSON regardless of Content-Type per discovery.
	var report HealthReport
	if err := json.Unmarshal(body, &report); err != nil {
		return HealthReport{}, fmt.Errorf("decode health %s (HTTP %d, body %q): %w", path, resp.StatusCode, truncate(string(body), 200), err)
	}
	// Try to decode each service's dependencies as a list; fall back to plain string.
	for i := range report.Services {
		if len(report.Services[i].Raw) == 0 {
			continue
		}
		var deps []DependencyHealth
		if err := json.Unmarshal(report.Services[i].Raw, &deps); err == nil {
			report.Services[i].Dependencies = deps
		}
	}
	return report, nil
}

// AllOK returns true iff Status == "200" and every service's Status == "200".
func (r HealthReport) AllOK() bool {
	if r.Status != "200" {
		return false
	}
	for _, s := range r.Services {
		if s.Status != "200" {
			return false
		}
	}
	return true
}
