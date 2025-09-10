package config

import (
	"github.com/robfig/cron/v3"
	"sync"
)

var PoolPayoutAddress string
var MinimumDifficulty uint64
var MaxDifficulty uint64
var TargetTime = 30
var StartingDifficulty uint64

var BannedAddresses struct {
	Addresses []string
	sync.RWMutex
}

var AllowedAddressPrefixes = []string{"12", "14"}

type CronSync struct {
	mu         sync.RWMutex
	CronMaster *cron.Cron
}

func (c *CronSync) AddCronJob(spec string, function func()) (cron.EntryID, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.CronMaster.AddFunc(spec, function)
}

func (c *CronSync) DeleteCronJob(id cron.EntryID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CronMaster.Remove(id)
}

var SystemCrons CronSync
