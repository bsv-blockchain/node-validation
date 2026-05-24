// config/short.go
package config

import "time"

// applyShort substitutes truncated durations when --short is set.
// Durations not listed are unchanged.
func applyShort(c *Config) {
	c.Durations.PC1Observation = 30 * time.Minute
	c.Durations.INTER1Observation = 5 * time.Minute
	c.Durations.CLIENT1Observation = 5 * time.Minute
	c.Durations.PERF1PerRate = 30 * time.Second
}
