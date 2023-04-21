package vat

import (
	"encoding/json"
	"fmt"
	"github.com/happywbfriends/http/hollander"
	"github.com/happywbfriends/http/json_rpc"
	"github.com/happywbfriends/nano/xerror"
)

func (c *Vat[RequestT, ResponseT]) JsonRpcHandler() json_rpc.CallHandler {
	if c.readHeaders {
		panic(fmt.Sprintf("Vat: headers scan enabled for %T, but JSON RPC doesn't support headers", new(RequestT)))
	}
	if c.readQuery {
		panic(fmt.Sprintf("Vat: query scan enabled for %T, but JSON RPC doesn't support queries", new(RequestT)))
	}

	return func(mw hollander.IMiddleware, data json.RawMessage) (interface{}, *json_rpc.Error) {
		var req RequestT

		if c.readBody {
			if err := json.Unmarshal(data, &req); err != nil {
				return nil, json_rpc.ErrJrpcParseError
			}
		}

		if c.readContext {
			_, err := Enrich(&req, tagContext, func(name string) (value any, found bool) {
				value, found = mw.Values()[name]
				return
			})
			if err != nil {
				mw.Log().Warnf("Error while enrich: %s", err)
				return nil, &json_rpc.Error{Code: json_rpc.ErrCodeInternalError, Message: "error filling request from context"}
			}
		}

		rc := vatRequestContext{
			rc: mw,
		}

		var result *ResponseT
		var cachedResponse []byte
		var xe xerror.IError

		if c.cachedHandler != nil {
			result, cachedResponse, xe = c.cachedHandler(&req, &rc)
		} else {
			result, xe = c.handler(&req, &rc)
		}

		if xe != nil {
			return nil, &json_rpc.Error{Code: json_rpc.ErrCodeInternalError, Message: "error"}
		}
		if cachedResponse != nil {
			return cachedResponse, nil
		} else {
			return result, nil
		}
	}
}
