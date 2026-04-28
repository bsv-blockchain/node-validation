// Command check-refs validates docs/discovery.md and docs/discovery.yaml
// for SP2: every path:line reference in the markdown must resolve, and
// the YAML must match the structural rules in docs/schema/discovery.schema.json.
//
// Run via: go run ./scripts/check-refs.go --discovery docs/discovery.md \
//
//	--yaml docs/discovery.yaml \
//	--upstream /path/to/teranode
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	mdPath := flag.String("discovery", "docs/discovery.md", "discovery markdown")
	yamlPath := flag.String("yaml", "docs/discovery.yaml", "discovery YAML")
	upstream := flag.String("upstream", "/Users/oskarsson/gitcheckout/teranode", "upstream root")
	flag.Parse()

	var errs []string
	if err := checkMarkdownRefs(*mdPath, *upstream); err != nil {
		errs = append(errs, err.Error())
	}
	if err := checkYAML(*yamlPath); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr, strings.Join(errs, "\n"))
		os.Exit(1)
	}
	fmt.Println("check-refs: OK")
}

var refPattern = regexp.MustCompile("`([^`\\s]+\\.[A-Za-z0-9]+):(\\d+)(?:-(\\d+))?`")

func checkMarkdownRefs(mdPath, upstreamRoot string) error {
	f, err := os.Open(mdPath)
	if err != nil {
		return fmt.Errorf("opening %s: %w", mdPath, err)
	}
	defer f.Close()

	var problems []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		matches := refPattern.FindAllStringSubmatch(scanner.Text(), -1)
		for _, m := range matches {
			rel, startS, endS := m[1], m[2], m[3]
			start, _ := strconv.Atoi(startS)
			end := start
			if endS != "" {
				end, _ = strconv.Atoi(endS)
			}
			full := filepath.Join(upstreamRoot, rel)
			info, err := os.Stat(full)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s:%d: ref %q not found (%v)", mdPath, lineNum, rel, err))
				continue
			}
			if info.IsDir() {
				problems = append(problems, fmt.Sprintf("%s:%d: ref %q is a directory", mdPath, lineNum, rel))
				continue
			}
			lines, err := countLines(full)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s:%d: counting lines of %q: %v", mdPath, lineNum, rel, err))
				continue
			}
			if start < 1 || end < start || end > lines {
				problems = append(problems, fmt.Sprintf("%s:%d: ref %q line %d-%d out of bounds (file has %d)",
					mdPath, lineNum, rel, start, end, lines))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanning %s: %w", mdPath, err)
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}

func countLines(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(b) == 0 {
		return 0, nil
	}
	n := strings.Count(string(b), "\n")
	if !strings.HasSuffix(string(b), "\n") {
		n++
	}
	return n, nil
}

// --- YAML validator ---

type discoveryDoc struct {
	UpstreamCommit string             `yaml:"upstream_commit"`
	DiscoveredAt   string             `yaml:"discovered_at"`
	Surfaces       []discoverySurface `yaml:"surfaces"`
}

type discoverySurface struct {
	ID         string   `yaml:"id"`
	Name       string   `yaml:"name"`
	Present    any      `yaml:"present"`
	SourceRefs []string `yaml:"source_refs"`
	Notes      string   `yaml:"notes"`
}

var allowedIDs = map[string]bool{
	"json_rpc": true, "rest_asset": true, "notifications": true,
	"p2p": true, "metrics": true, "health": true,
	"extended_tx": true, "testmempoolaccept": true, "fee_estimation": true,
	"mempool_query": true, "double_spend": true,
}

var commitPattern = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
var refLinePattern = regexp.MustCompile(`.+:[0-9]+(-[0-9]+)?$`)

func checkYAML(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	var doc discoveryDoc
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	var errs []string
	if !commitPattern.MatchString(doc.UpstreamCommit) {
		errs = append(errs, fmt.Sprintf("upstream_commit %q is not a 7-40 char hex sha", doc.UpstreamCommit))
	}
	if doc.DiscoveredAt == "" {
		errs = append(errs, "discovered_at is required")
	}
	if len(doc.Surfaces) != 11 {
		errs = append(errs, fmt.Sprintf("surfaces: want 11, got %d", len(doc.Surfaces)))
	}

	seen := map[string]bool{}
	for i, s := range doc.Surfaces {
		ctx := fmt.Sprintf("surfaces[%d] (id=%q)", i, s.ID)
		if !allowedIDs[s.ID] {
			errs = append(errs, fmt.Sprintf("%s: id not in allowed enum", ctx))
		}
		if seen[s.ID] {
			errs = append(errs, fmt.Sprintf("%s: duplicate id", ctx))
		}
		seen[s.ID] = true
		switch v := s.Present.(type) {
		case bool:
		case string:
			if v != "partial" {
				errs = append(errs, fmt.Sprintf("%s: present must be true|false|\"partial\", got %q", ctx, v))
			}
		default:
			errs = append(errs, fmt.Sprintf("%s: present must be bool or \"partial\"", ctx))
		}
		if s.Name == "" {
			errs = append(errs, fmt.Sprintf("%s: name required", ctx))
		}
		for j, ref := range s.SourceRefs {
			if !refLinePattern.MatchString(ref) {
				errs = append(errs, fmt.Sprintf("%s: source_refs[%d] %q must be path:line[-line]", ctx, j, ref))
			}
		}
		// "absent" surfaces must explain the search method in notes.
		if pres, ok := s.Present.(bool); ok && !pres {
			if !strings.Contains(strings.ToLower(s.Notes), "search") {
				errs = append(errs, fmt.Sprintf("%s: present=false requires notes documenting search method", ctx))
			}
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}
