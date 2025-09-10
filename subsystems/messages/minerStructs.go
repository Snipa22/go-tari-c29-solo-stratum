package messages

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type MinerRPCResponse struct {
	ID      int    `json:"id"`
	JsonRPC string `json:"jsonrpc"`
	Error   string `json:"error,omitempty"`
	Result  string `json:"result"`
}

type MinerRPCShareResponse struct {
	ID      int    `json:"id"`
	JsonRPC string `json:"jsonrpc"`
	Error   string `json:"error,omitempty"`
	Result  bool   `json:"result"`
}

type MinerRPCLoginResponse struct {
	ID      int    `json:"id"`
	JsonRPC string `json:"jsonrpc"`
	Result  struct {
		ID     string       `json:"id"`
		Job    MinerJobJSON `json:"job"`
		Status string       `json:"status"`
	} `json:"result"`
	Status string `json:"status"`
}

/*
{
"id":10,
"jsonrpc":"2.0",
"method":"submit",
"params":{"id":"ad215793-3891-445e-819d-1c2a03385943","job_id":"ed2d638234073c20cf34c3840b9fcd1dcfb44f5d59bdc6b4efdd3bf2d6f06886","nonce":"1438011623ffe312","result":"000000003c603daf1752c37b1ae6534ea94174468c7747faee188d36bd1a855e"}
}
*/
type MinerRPCRequest struct {
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	JsonRPC string          `json:"jsonrpc"`
}

type MinerRPCPush struct {
	JsonRPC string       `json:"jsonrpc"`
	Method  string       `json:"method"`
	Params  MinerJobJSON `json:"params"`
}

type MinerRPCLogin struct {
	Login string   `json:"login"`
	Pass  string   `json:"pass"`
	Agent string   `json:"agent"`
	Algo  []string `json:"algo"`
	RigID string   `json:"rigid"`
}

/*
		{"id":36,"jsonrpc":"2.0","method":"submit","params": {
			"id":"e0b5baa28cb125ed",
			"job_id":"96c2b726aaebc0e2",
			"nonce":"ef5a332c4ab80f82",
			"result":"0000000005d8ddc97fd4ca49abc3426a6957cbb87c7133bb4cb9331d405f7e6a"
	}}
*/
type MinerRPCSubmit struct {
	ID     string   `json:"id"`
	JobID  string   `json:"job_id"`
	Nonce  string   `json:"nonce"`
	POW    []uint64 `json:"pow"`
	Result string   `json:"result"`
}

type MinerUpstreamLogin struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ByteHex []byte

func (b *ByteHex) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%x", b)), nil
}

type MinerJobJSON struct {
	Algo   string `json:"algo"`
	Blob   string `json:"blob"`
	Height int    `json:"height"`
	JobID  string `json:"job_id"`
	Target string `json:"target"`
	XNonce string `json:"xn,omitempty"`
}

type MinerJobJson struct {
	ID                  uuid.UUID `json:"id"`
	ExtraNonce          uint32    `json:"extraNonce"`
	Height              int       `json:"height"`
	SeedHash            ByteHex   `json:"seed_hash"`
	Difficulty          int       `json:"difficulty"`
	DiffHex             ByteHex   `json:"diffHex"`
	Submissions         [][]byte  `json:"-"`
	BlockTemplateBlob   []byte    `json:"blocktemplate_blob,omitempty"`
	BlockHash           ByteHex   `json:"blockHash"`
	ClientPoolLocation  int       `json:"clientPoolLocation,omitempty"`
	ClientNonceLocation int       `json:"clientNonceLocation,omitempty"`
	Algo                string    `json:"algo"`
	HashingBlob         []byte    `json:"blob,omitempty"`
}
