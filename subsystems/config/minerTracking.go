package config

import "sync"

type minerDiff struct {
	mu   sync.Mutex
	diff int
}

func (c *minerDiff) Add(amt int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.diff += amt
}

func (c *minerDiff) Get() int {
	c.mu.Lock()
	defer func() {
		c.diff = 0
		c.mu.Unlock()
	}()
	return c.diff
}

var MinerDiff = minerDiff{diff: 0}
