package http_clt

import (
	"github.com/happywbfriends/nano/logger"
	"net/http"
	"time"
)

type HttpClientConfig struct {
	RequestTimeout      time.Duration `env:"REQUEST_TIMEOUT" envDefault:"5s"`
	IdleConnTimeout     time.Duration `env:"IDLE_CON_TIMEOUT" envDefault:"90s"`
	TotalMaxIdleConns   int           `env:"MAX_IDLE_CONS_TOTAL" envDefault:"10"`
	MaxIdleConnsPerHost int           `env:"MAX_IDLE_CONS_PER_HOST" envDefault:"10"`
	MaxConnsPerHost     int           `env:"MAX_CONS_PER_HOST" envDefault:"10"`
}

func (c HttpClientConfig) Validate() error {
	return nil
}

func NewHttpClient(cfg HttpClientConfig) *http.Client {
	if err := cfg.Validate(); err != nil {
		panic(err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = cfg.TotalMaxIdleConns
	transport.MaxConnsPerHost = cfg.MaxConnsPerHost
	transport.MaxIdleConnsPerHost = cfg.MaxIdleConnsPerHost
	transport.IdleConnTimeout = cfg.IdleConnTimeout

	return &http.Client{
		Timeout:   cfg.RequestTimeout,
		Transport: transport,
	}
}

func SafeResponseCloser(r *http.Response, log logger.ILogger) {
	// http://devs.cloudimmunity.com/gotchas-and-common-mistakes-in-go-golang/index.html
	// Most of the time when your http request fails the resp variable will be nil and the err variable will be non-nil.
	// However, when you get a redirection failure both variables will be non-nil. This means you can still end up claims a leak.
	if r != nil {
		if closeErr := r.Body.Close(); closeErr != nil && log != nil {
			log.Warnf("Error closing http response: %s", closeErr.Error())
		}
	}
}
