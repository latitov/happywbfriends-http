package json_rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/happywbfriends/http/hollander"
	"github.com/happywbfriends/nano/xerror"
	"io"
	"net/http"
)

type ServerRequest struct {
	Id      interface{}     `json:"id"`
	Version string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type ServerResponse struct {
	Id      interface{} `json:"id"`
	Version string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

type CallHandler func(mw hollander.IMiddleware, data json.RawMessage) (result interface{}, jrpcErr *Error)

type IServer interface {
	Handle(r *http.Request, mw hollander.IMiddleware) (proceed bool, e xerror.IError)
	AddMethod(name string, handler CallHandler) IServer
}

type Server struct {
	handlers      map[string]CallHandler
	singleHandler CallHandler
}

func NewServer() IServer {
	return &Server{
		handlers: make(map[string]CallHandler),
	}
}

// Создает Wildberries-lile JSON RPC сервер, где имя метода указывается в Path.
// такой серв игнорирует имя в BODY
func NewSingleMethodServer(h CallHandler) IServer {
	return &Server{
		singleHandler: h,
	}
}

func (s *Server) AddMethod(name string, handler CallHandler) IServer {
	s.handlers[name] = handler
	return s
}

// https://www.jsonrpc.org/specification#error_object

var (
	ErrJrpcParseError     = &Error{Code: ErrCodeParseError, Message: "Parse error"}
	ErrJrpcInvalidRequest = &Error{Code: ErrCodeInvalidRequest, Message: "Invalid Request"}
	//ErrJrpcMethodNotFound = &Error{Code: JrpcErrCodeMethodNotFound, Message: "Method not found"}

	responseParseError = &ServerResponse{
		Id:      nil, // If there was an error in detecting the id in the Request object (e.g. Parse error/Invalid Request), it MUST be Null.
		Version: Version,
		Error:   ErrJrpcParseError,
	}
	responseInvalidRequest = &ServerResponse{
		Id:      nil, // If there was an error in detecting the id in the Request object (e.g. Parse error/Invalid Request), it MUST be Null.
		Version: Version,
		Error:   ErrJrpcInvalidRequest,
	}
)

func (s *Server) Handle(r *http.Request, mw hollander.IMiddleware) (proceed bool, e xerror.IError) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return false, xerror.WrapFailure(err) // ошибка чтения не связана с запросом
	}

	var requests []*ServerRequest

	// Это массив или одиночный запрос?
	body = bytes.TrimLeft(body, " \t\r\n")
	isSingleObject := len(body) > 0 && body[0] == '{'
	if isSingleObject {
		var req ServerRequest
		if err := json.Unmarshal(body, &req); err != nil {
			mw.Log().Warnf("error parsing JRPC call: %s", err.Error())
			mw.SendJSON(http.StatusOK, &responseParseError)
			return false, nil
		}
		requests = append(requests, &req)
	} else {
		if err := json.Unmarshal(body, &requests); err != nil {
			mw.Log().Warnf("error parsing JRPC call: %s", err.Error())
			mw.SendJSON(http.StatusOK, &responseParseError)
			return false, nil
		}
	}

	var responces []*ServerResponse
	for _, req := range requests {
		if req.Id == "" {
			responces = append(responces, responseInvalidRequest)
			continue
		}

		handler := s.singleHandler
		if handler == nil {
			method := req.Method
			handler = s.handlers[method]
			if handler == nil {
				responces = append(responces, &ServerResponse{
					Id:      req.Id,
					Version: Version,
					Error: &Error{
						Code:    ErrCodeMethodNotFound,
						Message: fmt.Sprintf("method not found: '%s'", method),
					},
				})
				continue
			}
		}
		result, jrpcErr := handler(mw, req.Params)
		responces = append(responces, &ServerResponse{
			Id:      req.Id,
			Version: Version,
			Error:   jrpcErr,
			Result:  result,
		})
	}

	if isSingleObject {
		mw.SendJSON(http.StatusOK, responces[0])
	} else {
		mw.SendJSON(http.StatusOK, &responces)
	}

	return false, nil
}
