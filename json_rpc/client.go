package json_rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/happywbfriends/http/hollander"
	"github.com/happywbfriends/http/http_clt"
	"github.com/happywbfriends/nano/logger"
	"io"
	"net/http"
)

type ClientRequest struct {
	Id      string      `json:"id"` // по стандарту там string/integer/null, но мы как клиент вольны выбрать тип
	Version string      `json:"jsonrpc"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
}

type ClientResponse struct {
	Id     string          `json:"id"`
	Error  json.RawMessage `json:"error,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

var errReqRespMismatch = errors.New("request/response mismatch")
var errBadStatusCode = errors.New("invalid status code")

func Call(c *http.Client, url, jrpcMethod string, requestId string, headersOpt map[string]string,
	reqParams interface{},
	respParams interface{},
	ctx context.Context) (*Error, error) {

	if requestId == "" {
		UUID, err := uuid.NewRandom()
		if err != nil {
			return nil, err
		}
		requestId = UUID.String()
	}

	reqBody := ClientRequest{
		Id:      requestId,
		Version: Version,
		Method:  jrpcMethod,
		Params:  reqParams,
	}

	reqBytes, err := json.Marshal(&reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set(hollander.HeaderContentType, hollander.ContentTypeJSON)
	req.Header.Set(hollander.HeaderRequestId, requestId)
	for k, v := range headersOpt {
		req.Header.Set(k, v)
	}

	resp, err := c.Do(req)
	defer http_clt.SafeResponseCloser(resp, logger.NoLogger)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errBadStatusCode
	}

	blob, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var jrpcResponse ClientResponse
	if err := json.Unmarshal(blob, &jrpcResponse); err != nil {
		return nil, err
	}

	if reqBody.Id != jrpcResponse.Id {
		return nil, errReqRespMismatch
	}

	if len(jrpcResponse.Error) > 0 {
		jrpcErr := new(Error)
		if err := json.Unmarshal(jrpcResponse.Error, jrpcErr); err != nil {
			return nil, err
		}
		return jrpcErr, nil
	}

	if err := json.Unmarshal(jrpcResponse.Result, respParams); err != nil {
		return nil, err
	}
	return nil, nil
}
