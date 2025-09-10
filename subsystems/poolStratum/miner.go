package poolStratum

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	core "github.com/Snipa22/core-go-lib/milieu"
	"github.com/Snipa22/go-tari-grpc-lib/v3/nodeGRPC"
	"github.com/dchest/blake2b"
	"github.com/google/uuid"
	"github.com/holiman/uint256"
	"github.com/robfig/cron/v3"
	"github.com/snipa22/go-tari-c29-solo-stratum/subsystems/blockTemplateCache"
	"github.com/snipa22/go-tari-c29-solo-stratum/subsystems/config"
	"github.com/snipa22/go-tari-c29-solo-stratum/subsystems/messages"
	"github.com/snipa22/go-tari-c29-solo-stratum/subsystems/minerTracking"
	"github.com/snipa22/go-tari-c29-solo-stratum/subsystems/security"
	"github.com/snipa22/go-tari-c29-solo-stratum/subsystems/tipDataCache"
	"github.com/snipa22/powkit/cuckoo"
)

var badXmrig, _ = regexp.Compile(`XMRig/2.([0-9]|10|11|12|13|14|15|16).*`)

type minerTrust struct {
	threshold   int
	probability int
	penalty     int
}

type minerStruct struct {
	mu           sync.RWMutex
	ID           uuid.UUID
	ConnID       string
	Connection   net.Conn
	HashChan     chan messages.HashToVerify
	Milieu       *core.Milieu
	IpAddress    string
	Active       bool
	QuitRoutine  chan bool
	Address      string
	Port         messages.PortConfig
	rpcID        int
	cronJobs     []cron.EntryID
	Difficulty   uint64
	FixedDiff    bool
	connectTime  time.Time
	identifier   string
	proxyBan     bool
	lastContact  time.Time
	trust        minerTrust
	jobLog       map[string]*minerTracking.MinerJob
	jobList      []string
	curJob       *minerTracking.MinerJob
	proxy        bool
	hashes       uint64
	CoinbaseID   []byte
	cleanRunning bool
	needsXN      bool
	xn           string
}

func newMiner(conn net.Conn, hashChan chan messages.HashToVerify, milieu *core.Milieu, quitChan chan bool) (retData *minerStruct) {
	retData = &minerStruct{}
	retData.ID = uuid.New()
	retData.ConnID = fmt.Sprintf("%x", retData.ID)[0:16]
	retData.Connection = conn
	retData.HashChan = hashChan
	retData.Milieu = milieu
	retData.IpAddress = strings.Split(conn.RemoteAddr().Network(), `:`)[0]
	retData.Active = false
	retData.QuitRoutine = quitChan
	retData.FixedDiff = false
	retData.connectTime = time.Now()
	retData.lastContact = time.Now()
	retData.Difficulty = config.StartingDifficulty
	retData.proxy = false
	retData.jobLog = make(map[string]*minerTracking.MinerJob)
	retData.jobList = make([]string, 0)
	retData.trust = minerTrust{
		threshold:   30,
		probability: 256,
		penalty:     0,
	}
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, rand.Uint64())
	retData.CoinbaseID = buf
	retData.cleanRunning = false
	retData.needsXN = true
	retData.xn = fmt.Sprintf("%x", buf[0:2])
	return
}

func (m *minerStruct) UpdateLastComm() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastContact = time.Now()
}

func (m *minerStruct) GetLastComm() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastContact
}

func (m *minerStruct) RPCResponse(clientError string, result string) {
	if !m.Active {
		return
	}
	resp := messages.MinerRPCResponse{
		ID:      m.rpcID,
		JsonRPC: `2.0`,
		Error:   clientError,
		Result:  result,
	}
	if clientBlob, err := json.Marshal(resp); err != nil {
		m.Milieu.Error(err.Error())
		m.QuitRoutine <- true
		m.Active = false
	} else {
		clientBlob = append(clientBlob, '\n')
		if _, err := m.Connection.Write(clientBlob); err != nil {
			m.Milieu.Error(err.Error())
			m.QuitRoutine <- true
			m.Active = false
		}
	}
}

func (m *minerStruct) RPCShareResponse(clientError string, result bool) {
	if !m.Active {
		return
	}
	resp := messages.MinerRPCShareResponse{
		ID:      m.rpcID,
		JsonRPC: `2.0`,
		Error:   clientError,
		Result:  result,
	}
	if clientBlob, err := json.Marshal(resp); err != nil {
		m.Milieu.Error(err.Error())
		m.QuitRoutine <- true
		m.Active = false
	} else {
		clientBlob = append(clientBlob, '\n')
		if _, err := m.Connection.Write(clientBlob); err != nil {
			m.Milieu.Error(err.Error())
			m.QuitRoutine <- true
			m.Active = false
		}
	}
}

func (m *minerStruct) Login(jsonData json.RawMessage) {
	login := messages.MinerRPCLogin{}
	if err := json.Unmarshal([]byte(jsonData), &login); err != nil {
		m.Milieu.Error(err.Error())
		m.QuitRoutine <- true
		return
	}
	// Address should not be a 0 length field
	if len(login.Login) == 0 {
		m.RPCResponse(`Invalid address provided, please use a valid address`, ``)
		m.QuitRoutine <- true
		return
	}
	// If an agent is set, and a password is not, throw an error because there's no worker
	if len(login.Pass) == 0 && len(login.Agent) != 0 && !strings.Contains(login.Agent, `MinerGate`) {
		m.identifier = `x`
	} else {
		m.identifier = login.Pass
	}
	// Handle custom difficulty provided by miner
	m.Difficulty = config.StartingDifficulty
	m.Address = login.Login
	// Address format is <address>.paymentID+difficulty
	if strings.Contains(login.Login, `.`) {
		// Modern miners don't remember payment ID days, so perform some work.
		loginStrings := strings.Split(login.Login, `.`)
		m.Address = loginStrings[0]
		m.identifier = loginStrings[1]
	}
	if strings.Contains(login.Login, `+`) {
		m.FixedDiff = true
		split := strings.Split(login.Login, `+`)
		m.Address = split[0]
		if val, err := strconv.Atoi(split[1]); err != nil {
			m.RPCResponse(`Invalid difficulty provided`, ``)
			m.QuitRoutine <- true
			return
		} else {
			if uint64(val) < config.MinimumDifficulty {
				m.Difficulty = config.MinimumDifficulty
			} else if uint64(val) > config.MinimumDifficulty {
				m.Difficulty = config.MinimumDifficulty
			} else {
				m.Difficulty = uint64(val)
			}
		}
	}
	// Address should not be banned
	if m.isBanned() {
		return
	} else {
		// Enable ban autocheck
		if entry, err := config.SystemCrons.AddCronJob("0 */5 * * * *", func() {
			_ = m.isBanned()
		}); err != nil {
			m.Milieu.CaptureException(err)
		} else {
			m.cronJobs = append(m.cronJobs, entry)
		}
	}
	// Handle Nicehash
	if login.Agent != `` && strings.Contains(login.Agent, `NiceHash`) {
		m.FixedDiff = true
		m.Difficulty = config.StartingDifficulty
	}
	// Verify the address is valid
	if err := security.ValidateAddress(m.Address); err != nil {
		m.RPCResponse(`Invalid address provided, please use a valid address`, ``)
		m.QuitRoutine <- true
		return
	}
	// Handle old xmrrig miners
	if login.Agent != `` && badXmrig.Find([]byte(login.Agent)) != nil {
		m.RPCResponse(`XMRig version is too old, please update`, ``)
		m.QuitRoutine <- true
		return
	}
	// Handle password insanity
	if login.RigID != "" {
		m.identifier = login.RigID
	}
	if strings.Contains(login.Pass, `:`) {
		passwordSplit := strings.Split(login.Pass, `:`)
		if len(passwordSplit) > 2 {
			m.RPCResponse(`Too many options in the password field`, ``)
			m.QuitRoutine <- true
			return
		}
		m.identifier = passwordSplit[0]
	}

	if strings.Contains(login.Agent, `tari-osprey`) {
		m.needsXN = true
	}
	if strings.Contains(login.Agent, `hashreactor`) {
		m.needsXN = true
	}
	// Is this a proxy?
	if len(login.Agent) != 0 && strings.Contains(login.Agent, "xmr-node-proxy") {
		m.proxy = true
	}
	m.SendNewJob(true)
	if entry, err := config.SystemCrons.AddCronJob("* * * * * *", m.checkForNewWork); err != nil {
		m.Milieu.CaptureException(err)
	} else {
		m.cronJobs = append(m.cronJobs, entry)
	}
	if entry, err := config.SystemCrons.AddCronJob("*/60 * * * * *", m.NewDiff); err != nil {
		m.Milieu.CaptureException(err)
	} else {
		m.cronJobs = append(m.cronJobs, entry)
	}
	if entry, err := config.SystemCrons.AddCronJob("*/30 * * * * *", func() {
		m.mu.RLock()
		defer m.mu.RUnlock()
		config.MinerDiff.Add(int(m.Difficulty))
	}); err != nil {
		m.Milieu.CaptureException(err)
	} else {
		m.cronJobs = append(m.cronJobs, entry)
	}

}

func (m *minerStruct) getConnSeconds() int {
	return int(time.Now().Sub(m.connectTime).Seconds())
}

func (m *minerStruct) NewDiff() {
	if m.getConnSeconds() < 60 {
		return
	}
	targetTime := config.TargetTime
	if m.proxy {
		targetTime = 5
	}
	m.mu.RLock()
	curDiff := m.Difficulty
	newDiff := m.Difficulty
	if m.hashes > 0 {
		// Primary mechanism uses (hashes / connection time) * target time (5s for proxy)
		newDiff = (m.hashes / uint64(m.getConnSeconds())) * uint64(targetTime)
	} else {
		// Secondary mechanism provides a 10% reduction from inital to help miners settle in
		newDiff = uint64(float64(newDiff) * 0.9)
	}
	m.mu.RUnlock()
	if newDiff > uint64(float64(curDiff)*.95) && newDiff < uint64(float64(curDiff)*1.05) {
		// No change when diff is within 10% of current.  We only want to shift when it's useful.
		return
	}
	if newDiff < uint64(float64(curDiff)*.5) {
		newDiff = uint64(float64(curDiff) * .5)
	}
	if newDiff > uint64(float64(curDiff)*1.5) {
		newDiff = uint64(float64(curDiff) * 1.5)
	}
	if newDiff < config.MinimumDifficulty {
		newDiff = config.MinimumDifficulty
	} else if newDiff > config.MaxDifficulty && !m.proxy {
		newDiff = config.MaxDifficulty
	}
	m.mu.Lock()
	m.Difficulty = newDiff
	m.mu.Unlock()
	m.SendNewJob(false)
	m.Milieu.Debug(fmt.Sprintf("New difficulty: %d from %d with %d hashes over %d connection time\n", newDiff, curDiff, m.hashes, m.getConnSeconds()))
}

func (m *minerStruct) isBanned() bool {
	return false
}

// TODO: Work on tracking subsystems

func (m *minerStruct) checkForNewWork() {
	tipData := tipDataCache.GetTipData()
	if tipData == nil || m.curJob == nil {
		return
	}
	if tipData.Metadata.BestBlockHeight > m.curJob.BlockResult.Block.Header.Height-1 {
		m.SendNewJob(false)
	}
}

func (m *minerStruct) getJob() (*minerTracking.MinerJob, bool) {
	blockResult, err := blockTemplateCache.GetBlockWithXN(m.xn)
	if err != nil {
		fmt.Println(err)
		m.Milieu.CaptureException(err)
		return nil, false
	}
	// Add the job to the various lists via mutexes
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobList = append(m.jobList, fmt.Sprintf("%x", blockResult.BlockHash)[0:16])
	job := &minerTracking.MinerJob{
		BlockResult: blockResult,
		UsedNonces:  make([]uint64, 0),
		NonceMutex:  sync.RWMutex{},
		Target:      m.Difficulty,
	}
	m.jobLog[fmt.Sprintf("%x", blockResult.BlockHash)[0:16]] = job
	m.curJob = job
	m.checkForNewWork()
	return job, true
}

func (m *minerStruct) SendNewJob(login bool) {
	if !m.Active {
		return
	}
	if msg, newJob := m.getJob(); newJob {
		if val, err := msg.GetJobJSON(); err != nil {
			m.Milieu.CaptureException(err)
		} else {
			if m.needsXN {
				val.XNonce = m.xn
			}
			var fmtMsg interface{}
			if !login {
				fmtMsg = messages.MinerRPCPush{
					JsonRPC: "2.0",
					Method:  "job",
					Params:  *val,
				}
			} else {
				fmtMsg = messages.MinerRPCLoginResponse{
					ID:      m.rpcID,
					JsonRPC: "2.0",
					Result: struct {
						ID     string                `json:"id"`
						Job    messages.MinerJobJSON `json:"job"`
						Status string                `json:"status"`
					}{ID: m.ConnID, Job: *val, Status: "OK"},
					Status: "OK",
				}
			}
			jsonData, _ := json.Marshal(fmtMsg)
			jsonData = append(jsonData, '\n')
			if _, err := m.Connection.Write(jsonData); err != nil {
				m.Milieu.Error(err.Error())
				m.QuitRoutine <- true
			}
		}
	}
}

func (m *minerStruct) CleanMinerJobs() {
	if m.cleanRunning {
		return
	}
	m.cleanRunning = true
	defer func() {
		m.cleanRunning = false
	}()
	m.mu.Lock()
	defer m.mu.Unlock()
	newJobList := make([]string, 0)
	for _, jobStr := range m.jobList {
		if v, ok := m.jobLog[jobStr]; ok {
			if v.BlockResult.Block.Header.Timestamp > uint64(time.Now().Add(-1*6*time.Minute).Unix()) {
				newJobList = append(newJobList, jobStr)
			} else {
				delete(m.jobLog, jobStr)
			}
		}
	}
	m.jobList = newJobList
}

func (m *minerStruct) SubmitJob(jsonData json.RawMessage) {
	submittedWork := messages.MinerRPCSubmit{}
	if err := json.Unmarshal(jsonData, &submittedWork); err != nil {
		m.Milieu.Error(err.Error())
		m.QuitRoutine <- true
		return
	}
	if job, ok := m.jobLog[submittedWork.JobID]; !ok {
		m.RPCResponse(`Invalid job ID: `, submittedWork.JobID)
		m.SendNewJob(false)
		return
	} else {
		blakeHash := blake2b.New256()
		packedData := edgePacking(submittedWork.POW, 29)
		blakeHash.Write(packedData)
		result := blakeHash.Sum(nil)

		if hex.EncodeToString(result) != submittedWork.Result {
			m.RPCResponse(fmt.Sprintf(`Invalid Nonce %v`, submittedWork.Nonce), ``)
			return
		}

		b, err := hex.DecodeString(submittedWork.Nonce)
		if err != nil {
			m.RPCResponse(fmt.Sprintf(`Invalid Nonce %v`, submittedWork.Nonce), ``)
			return
		}
		nonce := binary.BigEndian.Uint64(b)
		// Check to see if the nonce has the XN
		if !strings.HasPrefix(strings.ToLower(submittedWork.Nonce), m.xn) {
			m.RPCShareResponse(fmt.Sprintf(`Invalid XNonce %v`, submittedWork.Nonce), false)
			return
		}
		job.NonceMutex.RLock()
		for _, v := range job.UsedNonces {
			if nonce == v {
				m.RPCResponse(`Duplicate nonce `, submittedWork.Nonce)
				m.SendNewJob(false)
				job.NonceMutex.RUnlock()
				return
			}
		}
		job.NonceMutex.RUnlock()
		job.NonceMutex.Lock()
		job.UsedNonces = append(job.UsedNonces, nonce)
		defer job.NonceMutex.Unlock()

		// Validate the cycle
		hasher := cuckoo.NewCuckarooWithSipBlock24(29, 42)
		nonceBytes, _ := hex.DecodeString(submittedWork.Nonce)
		nonceBytes = append(nonceBytes, job.BlockResult.MergeMiningHash...)
		valid, err := hasher.Verify(nonceBytes, submittedWork.POW)
		if err != nil || !valid {
			m.RPCResponse(fmt.Sprintf(`Invalid Nonce %v`, submittedWork.Nonce), ``)
			return
		}

		// Cycle is valid, check for difficulty
		blockDiffRaw := uint256.NewInt(0)
		blockDiffRaw.SetBytes(result)
		maxUint256 := uint256.NewInt(0)
		maxUint256Bytes, _ := hex.DecodeString("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
		maxUint256.SetBytes(maxUint256Bytes)
		resultUint256 := uint256.NewInt(0)
		resultUint256.Div(maxUint256, blockDiffRaw)
		diff := resultUint256.Uint64()

		// Perform the difficulty generation
		job.BlockResult.Block.Header.Nonce = nonce
		job.BlockResult.Block.Header.Pow.PowData = packedData
		if diff < job.Target {
			// Not a valid share, do not track, do not pass go.
			m.RPCShareResponse(fmt.Sprintf(`Low difficulty share %v`, submittedWork.ID), false)
			config.ShareCount.Incr("invalid")
			return
		}
		if diff < job.BlockResult.MinerData.TargetDifficulty {
			// Submit share to backend as valid, but non-block find.
			// OKAY SUCCESS!
			m.hashes += job.Target
			m.RPCShareResponse(``, true)
			config.ShareCount.Incr("valid")
			config.MinerDiff.Add(int(job.Target))
			return
		}
		go m.CleanMinerJobs()

		_, err = nodeGRPC.SubmitBlock(job.BlockResult.Block)
		if err != nil {
			// Submit share to backend as valid, but non-block find.
			m.hashes += job.Target
			m.Milieu.Info("SubmitBlock called with invalid block")
			m.RPCShareResponse(fmt.Sprintf(`Invalid block %v`, submittedWork.ID), false)
			config.ShareCount.Incr("valid")
			config.MinerDiff.Add(int(job.Target))
			return
		}

		// Submit share to backend as valid, with block find
		m.Milieu.Info(fmt.Sprintf("Block Found!  SubmitBlock called with valid block for %v - %x", job.BlockResult.Block.Header.Height, job.BlockResult.Block.Header.Hash))
		m.RPCShareResponse(``, true)
		m.hashes += job.Target
		config.ShareCount.Incr("valid")
		config.MinerDiff.Add(int(job.Target))
	}
}

func edgePacking(uncompressed []uint64, edgeBits int) []byte {
	// Generate a return slice that is len(input) * edge bits / 8 bytes long
	var returnSlice []byte
	miniBuffer := uint64(0)
	remaining := 64
	for _, rawEdge := range uncompressed {
		miniBuffer |= rawEdge << (64 - remaining)
		if edgeBits < remaining {
			remaining -= edgeBits
		} else {
			returnSlice = binary.LittleEndian.AppendUint64(returnSlice, miniBuffer)
			miniBuffer = rawEdge >> remaining
			remaining = 64 + remaining - edgeBits
		}
	}
	remainder := len(returnSlice) % 8
	if remainder == 0 {
		remainder = 8
	}
	if miniBuffer > 0 {
		tempSlice := make([]byte, 8)
		binary.LittleEndian.PutUint64(tempSlice, miniBuffer)
		returnSlice = append(returnSlice, tempSlice[0:remainder]...)
	}
	return returnSlice[0:int(math.Ceil(float64(len(uncompressed)*edgeBits)/8.0))]
}
