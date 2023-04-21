package http_clt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/happywbfriends/nano/logger"
	"io"
	"net/http"
	"net/url"
)

type IHttpClientJSON interface {
	Client() *http.Client
	JSON(method, url string, request, response interface{}, requestId string, headersOpt map[string]string, ctx context.Context) error
	GetJSON(url string, response interface{}, requestId string, headersOpt map[string]string, ctx context.Context) error
	PostJSON(url string, request, response interface{}, requestId string, headersOpt map[string]string, ctx context.Context) error
	// Если ответ = 200, то распарсит его в response и вернет 200, nil
	// Если ответ != 200, то не будет парсить и вернет status, nil
	// Если произошла ошибка, вернет 0, err
	JSONX(method, url string, request, response interface{}, requestId string, headersOpt map[string]string, ctx context.Context) (httpStatus int, e error)
	// Proxy не возвращает error потому что не понятно, что считать ошибкой, а что-нет.
	// Например, ответ 500 это ошибка? Мы же его просто пересылаем наверх
	// А отсутствие ответа? Его мы тоже репортим наверх как 502
	// В итоге было решено ничего не возвращать, а вместо этого передать log в метод
	Proxy(targetUrl string, r *http.Request, w http.ResponseWriter, headersOpt map[string]string, ctx context.Context, log logger.ILogger)
}

// Обработчик, вызываемый, когда status != 200
type InvalidResponseStatusHandlerFunc func(r *http.Response) error

func InvalidResponseStatusReadBodyToError(r *http.Response) error {
	// TODO: сделать чтение с ограничением по размеру, а не как щас - все считал и взял кусок
	blob, _ := io.ReadAll(r.Body)
	//if err != nil {
	//	return err
	//}
	if len(blob) > 0 {
		if len(blob) > 100 {
			blob = blob[:100]
		}
		return fmt.Errorf("invalid status code %d: %s", r.StatusCode, blob)
	}
	// TODO: сделать мапку с частыми кодами и готовыми ответами
	return fmt.Errorf("invalid status code %d", r.StatusCode)
}

func NewHttpClientJSON(c *http.Client) *HttpClientJSON {
	return &HttpClientJSON{
		clt:                          c,
		invalidResponseStatusHandler: InvalidResponseStatusReadBodyToError,
	}
}

type HttpClientJSON struct {
	clt                          *http.Client
	invalidResponseStatusHandler InvalidResponseStatusHandlerFunc
}

func (c *HttpClientJSON) Client() *http.Client {
	return c.clt
}

func (c *HttpClientJSON) GetJSON(url string, response interface{}, requestId string, headersOpt map[string]string, ctx context.Context) error {
	return c.JSON(http.MethodGet, url, nil, response, requestId, headersOpt, ctx)
}

func (c *HttpClientJSON) PostJSON(url string, request, response interface{}, requestId string, headersOpt map[string]string, ctx context.Context) error {
	return c.JSON(http.MethodPost, url, request, response, requestId, headersOpt, ctx)
}

func (c *HttpClientJSON) JSON(method, url string, request, response interface{}, requestId string, headersOpt map[string]string, ctx context.Context) error {
	var requestReader io.Reader
	if request != nil {
		requestBytes, err := json.Marshal(request)
		if err != nil {
			return err
		}
		requestReader = bytes.NewReader(requestBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, requestReader)
	if err != nil {
		return err
	}
	req.Header.Set(HeaderContentType, ContentTypeJSON)
	req.Header.Set(HeaderRequestId, requestId)
	for k, v := range headersOpt {
		req.Header.Set(k, v)
	}

	resp, err := c.clt.Do(req)

	defer SafeResponseCloser(resp, logger.NoLogger)

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return c.invalidResponseStatusHandler(resp)
	}

	if response != nil {
		blob, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(blob, response); err != nil {
			return err
		}
	}

	return nil
}

func (c *HttpClientJSON) JSONX(method, url string, request, response interface{}, requestId string, headersOpt map[string]string, ctx context.Context) (httpStatus int, e error) {
	var requestReader io.Reader
	if request != nil {
		requestBytes, err := json.Marshal(request)
		if err != nil {
			return 0, err
		}
		requestReader = bytes.NewReader(requestBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, requestReader)
	if err != nil {
		return 0, err
	}
	req.Header.Set(HeaderContentType, ContentTypeJSON)
	req.Header.Set(HeaderRequestId, requestId)
	for k, v := range headersOpt {
		req.Header.Set(k, v)
	}

	resp, err := c.clt.Do(req)

	defer SafeResponseCloser(resp, logger.NoLogger)

	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}

	if response != nil {
		blob, err := io.ReadAll(resp.Body)
		if err != nil {
			return 0, err
		}

		if err := json.Unmarshal(blob, response); err != nil {
			return 0, err
		}
	}

	return http.StatusOK, nil
}

var endToEndRequestHeaders = []string{"Content-Type"}
var endToEndResponseHeaders = []string{"Content-Type", "X-Request-Id"}

//var hopByHopHeaders []string

func (c *HttpClientJSON) Proxy(targetUrl string, r *http.Request, w http.ResponseWriter, headersOpt map[string]string, ctx context.Context, log logger.ILogger) {
	dataToSend := r.Body
	if r.Method == http.MethodGet {
		dataToSend = nil // чтобы избежать пересылки body в GET запросах (никто же не мешает злоумышленнику вложить тело)
	}

	req, err := http.NewRequestWithContext(ctx, r.Method, targetUrl, dataToSend)
	if err != nil {
		log.Warnf("Error in NewRequestWithContext() for upstream %s: %s", targetUrl, err.Error())
		return
	}

	for _, h := range endToEndRequestHeaders {
		if v := r.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}

	for k, v := range headersOpt {
		req.Header.Set(k, v)
	}

	resp, err := c.clt.Do(req)

	defer SafeResponseCloser(resp, logger.NoLogger)

	if err != nil {
		log.Warnf("Error calling upstream %s: %s", targetUrl, err.Error())
		// Any returned error will be of type *url.Error.
		// The url.Error value's Timeout method will report true if the request timed out.
		if urlError, ok := err.(*url.Error); ok {
			if urlError.Timeout() {
				w.WriteHeader(http.StatusBadGateway)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				//_, _ = w.Write([]byte(urlError.Error())) // TODO: отправлять текст ошибки без модерации плохо
			}
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			//_, _ = w.Write([]byte(err.Error())) // TODO: отправлять текст ошибки без модерации плохо
		}
		return
	}

	blob, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Warnf("Error reading body from upstream %s: %s", targetUrl, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		//_, _ = w.Write([]byte(err.Error())) // TODO: отправлять текст ошибки без модерации плохо
		return
	}

	for _, h := range endToEndResponseHeaders {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Add(h, v)
		}
	}

	w.WriteHeader(resp.StatusCode)

	_, err = w.Write(blob)
	if err != nil {
		log.Warnf("Error writing response for %s: %s", targetUrl, err.Error())
	}
}
