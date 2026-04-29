package teranode

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// MetricsScraper fetches and parses Prometheus text-format metrics.
// Discovery: docs/discovery.md §5.
type MetricsScraper struct {
	url    string
	http   *http.Client
	logger *slog.Logger
}

func NewMetricsScraper(rawURL string, logger *slog.Logger) (*MetricsScraper, error) {
	if rawURL == "" {
		return nil, nil
	}
	if _, err := url.Parse(rawURL); err != nil {
		return nil, fmt.Errorf("teranode metrics url %q: %w", rawURL, err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &MetricsScraper{url: rawURL, http: &http.Client{Timeout: 10 * time.Second}, logger: logger}, nil
}

type Sample struct {
	Labels map[string]string
	Value  float64
}

type MetricFamily struct {
	Name    string
	Help    string
	Type    string
	Samples []Sample
}

func (m *MetricsScraper) Scrape(ctx context.Context) (map[string]MetricFamily, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, m.url, nil)
	resp, err := m.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scrape: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scrape: HTTP %d", resp.StatusCode)
	}
	return parsePromText(resp.Body)
}

// parsePromText is a minimal parser for the Prometheus exposition format
// (https://prometheus.io/docs/instrumenting/exposition_formats/). It
// handles HELP, TYPE, and metric lines with optional labels. It does NOT
// handle exemplars or OpenMetrics-specific extensions; that's documented
// in docs/superpowers/specs/2026-04-29-sp3-backend-clients-design.md §9.
func parsePromText(r io.Reader) (map[string]MetricFamily, error) {
	out := map[string]MetricFamily{}
	get := func(name string) MetricFamily {
		mf, ok := out[name]
		if !ok {
			mf = MetricFamily{Name: name}
		}
		return mf
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# HELP ") {
			rest := strings.TrimPrefix(line, "# HELP ")
			parts := strings.SplitN(rest, " ", 2)
			if len(parts) == 2 {
				mf := get(parts[0])
				mf.Help = parts[1]
				out[parts[0]] = mf
			}
			continue
		}
		if strings.HasPrefix(line, "# TYPE ") {
			rest := strings.TrimPrefix(line, "# TYPE ")
			parts := strings.SplitN(rest, " ", 2)
			if len(parts) == 2 {
				mf := get(parts[0])
				mf.Type = parts[1]
				out[parts[0]] = mf
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		name, labels, value, err := parseSampleLine(line)
		if err != nil {
			continue // best-effort parser
		}
		mf := get(name)
		mf.Samples = append(mf.Samples, Sample{Labels: labels, Value: value})
		out[name] = mf
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan metrics: %w", err)
	}
	return out, nil
}

func parseSampleLine(line string) (string, map[string]string, float64, error) {
	openBrace := strings.IndexByte(line, '{')
	closeBrace := strings.IndexByte(line, '}')
	var name string
	var labels map[string]string
	var rest string
	if openBrace > 0 && closeBrace > openBrace {
		name = strings.TrimSpace(line[:openBrace])
		labelStr := line[openBrace+1 : closeBrace]
		labels = parseLabels(labelStr)
		rest = strings.TrimSpace(line[closeBrace+1:])
	} else {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return "", nil, 0, fmt.Errorf("bad sample %q", line)
		}
		name = fields[0]
		rest = strings.Join(fields[1:], " ")
	}
	valueStr := strings.Fields(rest)
	if len(valueStr) == 0 {
		return "", nil, 0, fmt.Errorf("missing value in %q", line)
	}
	v, err := strconv.ParseFloat(valueStr[0], 64)
	if err != nil {
		return "", nil, 0, err
	}
	return name, labels, v, nil
}

func parseLabels(s string) map[string]string {
	out := map[string]string{}
	for _, pair := range splitLabels(s) {
		eq := strings.IndexByte(pair, '=')
		if eq < 0 {
			continue
		}
		k := strings.TrimSpace(pair[:eq])
		v := strings.TrimSpace(pair[eq+1:])
		v = strings.Trim(v, `"`)
		out[k] = v
	}
	return out
}

func splitLabels(s string) []string {
	var out []string
	depth := 0
	start := 0
	inQuote := false
	for i, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
		case r == ',' && !inQuote && depth == 0:
			out = append(out, s[start:i])
			start = i + 1
		case r == '{' && !inQuote:
			depth++
		case r == '}' && !inQuote:
			depth--
		}
	}
	out = append(out, s[start:])
	return out
}

// BestBlockHeight returns teranode_blockassembly_best_block_height
// (the canonical chain-tip metric per discovery).
func (m *MetricsScraper) BestBlockHeight(ctx context.Context) (uint64, error) {
	mfs, err := m.Scrape(ctx)
	if err != nil {
		return 0, err
	}
	mf, ok := mfs["teranode_blockassembly_best_block_height"]
	if !ok || len(mf.Samples) == 0 {
		return 0, fmt.Errorf("metric teranode_blockassembly_best_block_height absent")
	}
	return uint64(mf.Samples[0].Value), nil
}

func (m *MetricsScraper) FSMState(ctx context.Context) (uint64, error) {
	mfs, err := m.Scrape(ctx)
	if err != nil {
		return 0, err
	}
	mf, ok := mfs["teranode_blockchain_fsm_current_state"]
	if !ok || len(mf.Samples) == 0 {
		return 0, fmt.Errorf("metric teranode_blockchain_fsm_current_state absent")
	}
	return uint64(mf.Samples[0].Value), nil
}

func (m *MetricsScraper) CatchupActive(ctx context.Context) (bool, error) {
	mfs, err := m.Scrape(ctx)
	if err != nil {
		return false, err
	}
	mf, ok := mfs["teranode_blockvalidation_catchup_active"]
	if !ok || len(mf.Samples) == 0 {
		return false, fmt.Errorf("metric teranode_blockvalidation_catchup_active absent")
	}
	return mf.Samples[0].Value > 0, nil
}
