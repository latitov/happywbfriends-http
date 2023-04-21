package hollander

import (
	"context"
	"github.com/google/uuid"
	"github.com/happywbfriends/nano/logger"
	"github.com/happywbfriends/nano/xerror"
	"net/http"
	"time"
)

// Ошибка в ответе имеет тип IHttpError, потому что для error всегда будет непонятно, считать ее 400 или 500
type HttpHandler func(r *http.Request, mw IMiddleware) (proceed bool, e xerror.IError)
type PanicHandler func(w http.ResponseWriter, r *http.Request, mw IMiddleware, e interface{})

type Values map[string]interface{}

type Middleware struct {
	log            logger.ILogger
	handlers       []HttpHandler
	values         Values
	requestTimeout time.Duration
	panicHandler   PanicHandler
	metricsEnabled bool
	metrics        HTTPMetrics
	maxReadBytes   int64
}

func NewMiddleware(log logger.ILogger) *Middleware {
	return &Middleware{
		log: log,
	}
}

func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.panicHandler != nil {
		defer func() {
			if err := recover(); err != nil {
				m.panicHandler(w, r, nil, err)
			}
		}()
	}

	// Надо сделать обработку 404, 405, иначе мы про них не узнаем, а нам надо банить тех, кто часто 400-ит

	var startTm time.Time

	if m.metricsEnabled {
		startTm = time.Now()
		m.metrics.NbReq.Inc()
		m.metrics.NbCurrentConns.Inc()
		defer m.metrics.NbCurrentConns.Dec()
	}

	vals := make(Values)
	for k, v := range m.values {
		vals[k] = v
	}

	// RequestId
	requestId := r.Header.Get(HeaderRequestId)
	if requestId == "" {
		UUID, err := uuid.NewRandom()
		if err != nil {
			m.log.Warnf("Failed generating request id: %s", err)
		} else {
			requestId = UUID.String()
		}
	}
	if requestId != "" {
		w.Header().Set(HeaderRequestId, requestId)
	}

	// Timeout
	if m.requestTimeout > 0 {
		// For incoming server requests, the context is canceled when the client's connection closes,
		// the request is canceled (with HTTP/2), or when the ServeHTTP method returns.
		newCtx, cancel := context.WithTimeout(r.Context(), m.requestTimeout)
		defer cancel()

		r = r.WithContext(newCtx)
	}

	// Max bytes
	if m.maxReadBytes > 0 {
		// https://www.alexedwards.net/blog/how-to-properly-parse-a-json-request-body
		r.Body = http.MaxBytesReader(w, r.Body, m.maxReadBytes)
	}

	interceptor := newResponseStatusInterceptor(w)

	rc := _RequestContext{
		log:       m.log.With("x-request-id", requestId),
		w:         interceptor,
		r:         r,
		vals:      vals,
		requestId: requestId,
	}

	for _, h := range m.handlers {
		proceed, xe := h(r, &rc)
		if xe != nil {

			publicMessage := xe.PublicMessage()
			privateDetails := xe.PrivateDetails()
			if publicMessage != "" && privateDetails != "" {
				rc.log.Warnf("%s %s: %s -- %s", r.Method, r.RequestURI, publicMessage, privateDetails)
			} else if publicMessage != "" {
				rc.log.Warnf("%s %s: %s", r.Method, r.RequestURI, publicMessage)
			} else if privateDetails != "" {
				rc.log.Warnf("%s %s: %s", r.Method, r.RequestURI, privateDetails)
			} else { // both empty
				rc.log.Warnf("%s %s: %+v", r.Method, r.RequestURI, xe)
			}

			statusCode := xe.HttpStatus()
			if statusCode == 0 {
				statusCode = http.StatusInternalServerError
			}
			if publicMessage != "" {
				rc.SendText(statusCode, publicMessage)
			} else {
				rc.Send(statusCode, "", nil)
			}
			break
		} else if !proceed {
			break
		}
	}

	statusCode := rc.w.statusCode

	if m.metricsEnabled {
		if statusCode >= 400 && statusCode <= 499 {
			m.metrics.NbReq4xx.Inc()
		} else if statusCode >= 500 && statusCode <= 599 {
			m.metrics.NbReq5xx.Inc()
		} else {
			m.metrics.NbReq2xx.Inc()
			m.metrics.Latency2xxMillis.Observe(time.Since(startTm).Seconds())
		}
	}
}

func (m *Middleware) Set(k string, v interface{}) *Middleware {
	if m.values == nil {
		m.values = make(Values)
	}
	m.values[k] = v

	return m
}

func (m *Middleware) Use(h HttpHandler) *Middleware {
	m.handlers = append(m.handlers, h)
	return m
}

func (m *Middleware) WithMetrics(ns, method string) *Middleware {
	m.metrics = NewHttpMetrics(ns, method)
	m.metricsEnabled = true
	return m
}

func (m *Middleware) WithTimeoutContext(timeout time.Duration) *Middleware {
	m.requestTimeout = timeout
	return m
}

func (m *Middleware) WithMaxBytesReader(maxBytes int64) *Middleware {
	m.maxReadBytes = maxBytes
	return m
}
