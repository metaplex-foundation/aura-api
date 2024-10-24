package types

import (
	"encoding/json"

	"github.com/gagliardetto/solana-go/rpc/jsonrpc"
)

const (
	JSONRPCVersion = "2.0"
)

const (
	InvalidRequestErrCode = -32600
	ParseErrCode          = -32700
)

var (
	ParseError      = NewRPCError(ParseErrCode, "Parse error", nil)
	InvalidReqError = NewRPCError(InvalidRequestErrCode, "Invalid request", nil)
)

type (
	// copied from jsonrpc, 'id' field changed to interface (can be int, string, null)
	RPCRequest struct {
		Method  string      `json:"method"`
		Params  interface{} `json:"params,omitempty"`
		ID      interface{} `json:"id"`
		JSONRPC string      `json:"jsonrpc"`
	}
	RPCRequests []*RPCRequest

	RPCResponse struct {
		JSONRPC string            `json:"jsonrpc"`
		ID      interface{}       `json:"id"`
		Error   *jsonrpc.RPCError `json:"error,omitempty"`
		Result  json.RawMessage   `json:"result,omitempty"`
	}
)

func NewRPCErrorResponse(err *jsonrpc.RPCError, id interface{}) *RPCResponse {
	return &RPCResponse{
		JSONRPC: JSONRPCVersion,
		Error:   err,
		ID:      id,
	}
}

func NewRPCError(code int, message string, data interface{}) *jsonrpc.RPCError {
	return &jsonrpc.RPCError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}
