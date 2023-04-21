package vat

import (
	"context"
	"github.com/happywbfriends/http/hollander"
	"github.com/happywbfriends/nano/logger"
)

type IRequestContext interface {
	Context() context.Context
	Log() logger.ILogger
	RequestId() string
}

type vatRequestContext struct {
	rc hollander.IMiddleware
}

func (r *vatRequestContext) Context() context.Context { return r.rc.Context() }
func (r *vatRequestContext) Log() logger.ILogger      { return r.rc.Log() }
func (r *vatRequestContext) RequestId() string        { return r.rc.RequestId() }
