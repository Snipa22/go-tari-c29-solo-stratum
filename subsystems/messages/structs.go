package messages

type Response struct {
	Bytes []byte
	Error error
}

type HashToVerify struct {
	Input  []byte
	Seed   []byte
	Result chan Response
}

type PortConfig struct {
	PoolPort   int    `json:"poolPort"`
	Difficulty int    `json:"difficulty"`
	PortDesc   string `json:"portDesc"`
	PortType   string `json:"portType"`
	Hidden     bool   `json:"hidden"`
	Ssl        bool   `json:"ssl"`
}

type PortPush []PortConfig
