// config/defaults.go
package config

import "time"

// applyDefaults fills in non-secret default values for a Config that has
// not had any layer applied yet.
func applyDefaults(c *Config) {
	if c.Network == "" {
		c.Network = NetworkTestnet
	}
	if c.Durations.PC1Observation == 0 {
		c.Durations.PC1Observation = 168 * time.Hour
	}
	if c.Durations.INTER1Observation == 0 {
		c.Durations.INTER1Observation = 336 * time.Hour
	}
	if c.Durations.PERF1PerRate == 0 {
		c.Durations.PERF1PerRate = 5 * time.Minute
	}
	if c.Durations.DefaultPropagation == 0 {
		c.Durations.DefaultPropagation = 10 * time.Second
	}
	if c.Durations.CLIENT1Observation == 0 {
		c.Durations.CLIENT1Observation = time.Hour
	}
	if c.Durations.NewNFR7Iterations == 0 {
		c.Durations.NewNFR7Iterations = 100
	}
	if c.Limits.PERF1MaxTPS == 0 {
		c.Limits.PERF1MaxTPS = 1000
	}
	if c.Limits.INTER2TxCount == 0 {
		c.Limits.INTER2TxCount = 1000
	}
	if c.Limits.CLIENT3TxCount == 0 {
		c.Limits.CLIENT3TxCount = 500
	}
	if c.Limits.FR7ChainDepth == 0 {
		c.Limits.FR7ChainDepth = 25
	}
	if c.Limits.FR10LatencyTargetMs == 0 {
		c.Limits.FR10LatencyTargetMs = 100
	}
	if len(c.Limits.FR8PriorityLevels) == 0 {
		c.Limits.FR8PriorityLevels = []string{"economy", "standard", "priority"}
	}
	if c.Limits.NFR13MaxProbeRate == 0 {
		c.Limits.NFR13MaxProbeRate = 1000
	}
	if c.Limits.NFR13ProbeDuration == 0 {
		c.Limits.NFR13ProbeDuration = 5 * time.Second
	}
	if c.ReportJSON == "" {
		c.ReportJSON = "report.json"
	}
	if c.ReportHTML == "" {
		c.ReportHTML = "report.html"
	}
	if c.TestTimeout == 0 {
		c.TestTimeout = 30 * time.Minute
	}
}
