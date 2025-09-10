package blockTemplateCache

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"github.com/Snipa22/core-go-lib/milieu"
	"github.com/Snipa22/go-tari-grpc-lib/v3/nodeGRPC"
	"github.com/Snipa22/go-tari-grpc-lib/v3/tari_generated"
	"github.com/snipa22/go-tari-c29-solo-stratum/subsystems/config"
	"github.com/snipa22/go-tari-c29-solo-stratum/subsystems/tipDataCache"
)

// poolID is a random byte string used to ID the pool in the coinbase txn
var poolID *[]byte

// poolStringIDList is a 9 character string used to ID the pool in the coinbase txn, this is a list of valid ones.
var poolStringIDList [][]byte = make([][]byte, 0)

// Yes, I'm writing a function to turn strings into []bytes

// blockTemplateCacheStruct contains the response from a GetBlockTemplate, this is used to generate the
// GetNewBlockTemplateWithCoinbasesRequest that is sent to the Tari daemon, this is needed to build the base coinbase
// transaction, which needs to contain the pool ID and other data related to the miner unique nonce.
type blockTemplateCacheStruct struct {
	blockTemplate *tari_generated.NewBlockTemplate
	reward        uint64
	mutex         sync.RWMutex
}

var c29BTCache *blockTemplateCacheStruct = nil
var sha3xBTCache *blockTemplateCacheStruct = nil
var rxmBTCache *blockTemplateCacheStruct = nil

var globalBlockCache = make([]blockCacheStruct, 0)
var globalBlocKCacheMutex sync.Mutex

type blockCacheStruct struct {
	block  *tari_generated.GetNewBlockResult
	xnUsed []string
}

// When a miner requests a block template, we need to update the coinbase txn with their unique hash and the pool hash

// UpdateBlockTemplateCache keeps the main block template system spinning and updating to keep things clean
func UpdateBlockTemplateCache(core *milieu.Milieu) {
	if len(poolStringIDList) == 0 {
		poolStringIDList = append(poolStringIDList, []byte(" Stratum "))
	}
	if poolID == nil {
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, rand.Uint64())
		poolID = &buf
	}

	blockTemplateResponse, err := nodeGRPC.GetBlockTemplate(&tari_generated.PowAlgo{PowAlgo: tari_generated.PowAlgo_POW_ALGOS_CUCKAROO})
	if err != nil {
		core.CaptureException(err)
		return
	}
	if c29BTCache == nil {
		c29BTCache = &blockTemplateCacheStruct{}
	}

	c29BTCache.mutex.Lock()
	c29BTCache.blockTemplate = blockTemplateResponse.GetNewBlockTemplate()
	c29BTCache.reward = blockTemplateResponse.MinerData.Reward
	c29BTCache.mutex.Unlock()
}

// GetBlockSha3 uses the block template cache to call GetBlockWithCoinbases with the miner ID as well as the
// pool ID in the coinbase TXN, this is essentially a hard-wrapper for the GetNewBlock GRPC call.
func GetBlockSha3(minerID []byte) (*tari_generated.GetNewBlockResult, error) {

	// Generate the coinbase Extra
	coinbaseExtra := make([]byte, 40)
	/*
		Format is as follows:
		0-2: "WUF"
		3-12: Squad Identifier - poolStringIDList
		13-20: PoolID
		21-28: MinerID
		29-37: RandomData
		37-39: "WUF"
	*/
	coinbaseExtra[0] = 0x57
	coinbaseExtra[1] = 0x55
	coinbaseExtra[2] = 0x46
	coinbaseExtra[37] = 0x57
	coinbaseExtra[38] = 0x55
	coinbaseExtra[39] = 0x46
	poolString := poolStringIDList[rand.Intn(len(poolStringIDList))]
	for i, v := range poolString {
		coinbaseExtra[i+3] = v
	}
	for i, v := range *poolID {
		coinbaseExtra[i+13] = v
	}
	for i, v := range minerID {
		coinbaseExtra[i+21] = v
	}
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, rand.Uint64())
	for i, v := range buf {
		coinbaseExtra[i+28] = v
	}

	// Generate the coinbase transaction
	coinbaseData := make([]*tari_generated.NewBlockCoinbase, 0)
	coinbaseData = append(coinbaseData, &tari_generated.NewBlockCoinbase{
		Address:            config.PoolPayoutAddress,
		Value:              100,
		StealthPayment:     false,
		RevealedValueProof: true,
		CoinbaseExtra:      coinbaseExtra,
	})

	// Get the block data w/ the coinbases
	return nodeGRPC.GetNewBlockTemplateWithCoinbases(&tari_generated.GetNewBlockTemplateWithCoinbasesRequest{
		Algo:      &tari_generated.PowAlgo{PowAlgo: tari_generated.PowAlgo_POW_ALGOS_CUCKAROO},
		Coinbases: coinbaseData,
	})
}

func GetBlockWithXN(xn string) (*tari_generated.GetNewBlockResult, error) {
	// CurTip
	tip := tipDataCache.GetTipData()
	if tip == nil {
		return nil, errors.New("tipDataCache is nil")
	}
	// Lock the global mutex
	globalBlocKCacheMutex.Lock()
	defer globalBlocKCacheMutex.Unlock()
	newBlockCache := make([]blockCacheStruct, 0)
	var result *tari_generated.GetNewBlockResult = nil
	for _, v := range globalBlockCache {
		// Discard any blocks that have a height less or equal to tip, as they're too old
		if v.block.Block.Header.Height <= tip.Metadata.BestBlockHeight {
			continue
		}
		if result != nil {
			newBlockCache = append(newBlockCache, v)
			continue
		}
		// Check to see if the XN is used already
		xnUsed := false
		for _, usedXN := range v.xnUsed {
			if usedXN == xn {
				xnUsed = true
			}
		}
		if !xnUsed {
			v.xnUsed = append(v.xnUsed, xn)
			result = v.block
		}
		newBlockCache = append(newBlockCache, v)
	}
	if result == nil {
		// All current blocks have this XN in use, generate a new block
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, rand.Uint64())
		poolID = &buf
		rawBlock, err := GetBlockSha3(*poolID)
		if err != nil {
			return rawBlock, err
		}
		result = rawBlock
		newBlockCache = append(newBlockCache, blockCacheStruct{
			block:  rawBlock,
			xnUsed: []string{xn},
		})
	}
	return result, nil
}

// convertUint64AsLEBytes
func convertUint64AsLEBytes(in uint64) []byte {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, in)
	if err != nil {
		fmt.Println("binary.Write failed:", err)
	}
	return buf.Bytes()
}

// convertUint64AsBEBytes
func convertUint64AsBEBytes(in uint64) []byte {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, in)
	if err != nil {
		fmt.Println("binary.Write failed:", err)
	}
	return buf.Bytes()
}

/*
// convertUint32AsLEBytes
func convertUint32AsLEBytes(in uint32) []byte {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, in)
	if err != nil {
		fmt.Println("binary.Write failed:", err)
	}
	return buf.Bytes()
}

// convertUint32AsBEBytes
func convertUint32AsBEBytes(in uint32) []byte {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, in)
	if err != nil {
		fmt.Println("binary.Write failed:", err)
	}
	return buf.Bytes()
}



type hashableBlockHeader struct {
	HeaderLength uint64
}

// HashBlockHeader processes a block header with the blake2b domain seperated hasher
func HashBlockHeader(header *tari_generated.BlockHeader) ([]byte, error) {
	hasher, err := blake2b.New256(nil)
	if err != nil {
		return nil, err
	}
	// header is a reverse engineered magic value used to seperate block data
	// header := "com.tari.base_layer.core.blocks.v(0x30).block_header.n0"
	fmt.Printf("%v\n", header.Version)
	fmt.Printf("%v\n", header.Height)
	fmt.Printf("%x\n", header.PrevHash)
	fmt.Printf("%v\n", header.Timestamp)
	fmt.Printf("%x\n", header.InputMr)
	fmt.Printf("%x\n", header.OutputMr)
	fmt.Printf("%v\n", header.OutputMmrSize)
	fmt.Printf("%x\n", header.BlockOutputMr)
	fmt.Printf("%x\n", header.KernelMr)
	fmt.Printf("%v\n", header.KernelMmrSize)
	fmt.Printf("%x\n", header.TotalKernelOffset)
	fmt.Printf("%x\n", header.TotalScriptOffset)
	fmt.Printf("%x\n", header.ValidatorNodeMr)
	fmt.Printf("%v\n", header.ValidatorNodeSize)

	hasher.Write(convertUint64AsLEBytes(50))
	hasher.Write([]byte("com.tari.base_layer.core.blocks.v"))
	hasher.Write([]byte{0x30})
	hasher.Write([]byte(".block_header.n0"))
	hasher.Write(convertUint32AsLEBytes(header.Version)[0:2])
	hasher.Write(convertUint64AsLEBytes(header.Height))
	hasher.Write(header.PrevHash)
	hasher.Write(convertUint64AsLEBytes(header.Timestamp))
	hasher.Write(header.InputMr)
	hasher.Write(header.OutputMr)
	hasher.Write(convertUint64AsLEBytes(header.OutputMmrSize))
	hasher.Write(header.BlockOutputMr)
	hasher.Write(header.KernelMr)
	hasher.Write(convertUint64AsLEBytes(header.KernelMmrSize))
	hasher.Write(convertUint32AsLEBytes(32))
	hasher.Write(header.TotalKernelOffset)
	hasher.Write(convertUint32AsLEBytes(32))
	hasher.Write(header.TotalScriptOffset)
	hasher.Write(header.ValidatorNodeMr)
	hasher.Write(convertUint64AsLEBytes(header.ValidatorNodeSize))
	resp := hasher.Sum(nil)
	return resp, nil
}

*/

func reverse(s []byte) []byte {
	a := make([]byte, len(s))
	copy(a, s)

	for i := len(a)/2 - 1; i >= 0; i-- {
		opp := len(a) - 1 - i
		a[i], a[opp] = a[opp], a[i]
	}

	return a
}
