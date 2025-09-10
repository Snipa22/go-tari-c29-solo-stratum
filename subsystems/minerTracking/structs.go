package minerTracking

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/Snipa22/core-go-lib/milieu"
	"github.com/Snipa22/go-tari-grpc-lib/v3/tari_generated"
	"github.com/google/uuid"
	"github.com/snipa22/go-tari-c29-solo-stratum/subsystems/messages"
)

/*
Miner is the base object which contains the details for a connected miner, this is to work out their hashrate
and other data structure systems, every miner is a TCP connection, so this should be trackable-enough with that.

Given this context, we need to store only basic information, but we do need to include structural support bits,
importantly, we must have a way to generate new block templates and push them to the clients.

This only supports sha3x, so verification is nominally, easy.

Block Coinbase TXN's are much simpler, as they /only/ go to the pool address, we need to edit the coinbase
*/

type minerTrust struct {
	threshold   int
	probability int
	penalty     int
}

type MinerJob struct {
	BlockResult *tari_generated.GetNewBlockResult
	UsedNonces  []uint64
	Target      uint64
	NonceMutex  sync.RWMutex
}

func (job *MinerJob) diffToTarget() uint64 {
	return uint64(18446744073709551615) / job.Target
}

// GetJobJSON converts a job into a JSON byte array appropriate for sending over the wire to the miner
func (job *MinerJob) GetJobJSON() (*messages.MinerJobJSON, error) {
	jobTarget := job.diffToTarget()
	targetSlice := make([]byte, 8)
	binary.LittleEndian.PutUint64(targetSlice, jobTarget)
	return &messages.MinerJobJSON{
		Algo:   "C29",
		Blob:   fmt.Sprintf("%x", job.BlockResult.MergeMiningHash),
		Height: int(job.BlockResult.Block.Header.Height),
		JobID:  fmt.Sprintf("%x", job.BlockResult.BlockHash)[0:16],
		Target: fmt.Sprintf("%x", targetSlice),
	}, nil
}

var running = false
var jobCache = make(map[string]*MinerJob)
var jobCacheList = make([]string, 0)
var jobCacheMutex sync.RWMutex

type Miner struct {
	// Miner Data
	PayoutAddress string
	ID            uuid.UUID
	Difficulty    int
	CoinbaseID    []byte
	BlockID       uint64

	// Internal Data
	connectTime time.Time
	hashes      int
	lastContact time.Time
	trust       minerTrust

	jobMutex sync.RWMutex

	// Structural/External support
	core *milieu.Milieu
}

func NewMiner(core *milieu.Milieu) Miner {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, rand.Uint64())
	return Miner{
		PayoutAddress: "",
		ID:            uuid.New(),
		CoinbaseID:    buf,
		Difficulty:    10000000000,
		connectTime:   time.Now(),
		hashes:        0,
		lastContact:   time.Now(),
		trust:         minerTrust{},
		core:          core,
	}
}

func (miner *Miner) Checkin() {
	miner.lastContact = time.Now()
}

func (miner *Miner) GetMinerJob(block *tari_generated.Block) *MinerJob {
	key := make([]byte, 0)
	for _, v := range block.Body.Outputs {
		if v.Features.OutputType != 1 {
			continue
		}
		// We're now in the coinbase txn, extract the coinbase data.
		key = v.Hash
		break
	}
	if len(key) == 0 {
		return nil
	}
	jobCacheMutex.RLock()
	defer jobCacheMutex.RUnlock()
	if v, ok := jobCache[hex.EncodeToString(key)]; ok {
		return v
	}
	return nil
}

func (miner *Miner) AddMinerJob(block *tari_generated.GetNewBlockResult) {
	key := make([]byte, 0)
	for _, v := range block.Block.Body.Outputs {
		if v.Features.OutputType != 1 {
			continue
		}
		// We're now in the coinbase txn, extract the coinbase data.
		key = v.Hash
		break
	}
	keyStr := hex.EncodeToString(key)
	jobCacheMutex.Lock()
	defer jobCacheMutex.Unlock()
	if _, ok := jobCache[keyStr]; ok {
		return
	}
	jobCache[keyStr] = &MinerJob{
		BlockResult: block,
		UsedNonces:  make([]uint64, 0),
	}
	jobCacheList = append(jobCacheList, keyStr)
}

func (miner *Miner) CleanMinerJobs() {
	if running {
		return
	}
	running = true
	defer func() {
		running = false
	}()
	jobCacheMutex.Lock()
	defer jobCacheMutex.Unlock()
	newJobList := make([]string, 0)
	for _, jobStr := range jobCacheList {
		if v, ok := jobCache[jobStr]; ok {
			if v.BlockResult.Block.Header.Timestamp > uint64(time.Now().Add(-1*5*time.Minute).Unix()) {
				newJobList = append(newJobList, jobStr)
			} else {
				delete(jobCache, jobStr)
			}
		}
	}
	jobCacheList = newJobList
}
