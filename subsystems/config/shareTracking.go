package config

import (
	"sync"
)

type shareCounters struct {
	mu       sync.Mutex
	counters map[string]int
}

func (c *shareCounters) Incr(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counters[name]++
	c.counters["total"]++
}

func (c *shareCounters) Get(name string) int {
	c.mu.Lock()
	defer func() {
		c.counters[name] = 0
		c.mu.Unlock()
	}()
	return c.counters[name]
}

var ShareCount = shareCounters{counters: make(map[string]int)}

func InitShareCount() {
	ShareCount.mu.Lock()
	defer ShareCount.mu.Unlock()
	ShareCount.counters["valid"] = 0
	ShareCount.counters["invalid"] = 0
	ShareCount.counters["trusted"] = 0
	ShareCount.counters["total"] = 0

}
