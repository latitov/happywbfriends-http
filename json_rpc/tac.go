package json_rpc

const Version = "2.0"

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	ErrCodeParseError     = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternalError  = -32603
)

// https://www.jsonrpc.org/specification#error_object
// Диапазон -32000 .. -32099 зарезервирован под пользовательские коды
