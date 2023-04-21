package hollander

import (
	"context"
	"encoding/json"
	"github.com/happywbfriends/nano/logger"
	"github.com/happywbfriends/nano/xerror"
	"io"
	"net/http"
)

type IMiddleware interface {
	Values() Values
	Context() context.Context
	Log() logger.ILogger
	RequestId() string
	ReadJSONBody(dest interface{}) xerror.IError
	SetHeader(name, value string)
	Writer() http.ResponseWriter
	// Все Send... методы не возвращают никаких ошибок, поскольку предполагается, что отправка ответа - это последний
	// этап обработки любого запроса, и в случае проблем с записью ответа пользовательский код все равно не может
	// ничего сделать кроме как отписаться в логи, что данные методы и делают за него.
	Send(status int, contentType string, dataOpt []byte)
	SendText(status int, text string)
	SendJSON(status int, obj interface{})
}

type _RequestContext struct {
	log       logger.ILogger
	w         *responseStatusInterceptor
	r         *http.Request
	vals      Values
	requestId string
}

func (m *_RequestContext) Values() Values {
	return m.vals
}

func (m *_RequestContext) Context() context.Context {
	return m.r.Context()
}

func (m *_RequestContext) Log() logger.ILogger {
	return m.log
}

func (m *_RequestContext) RequestId() string {
	return m.requestId
}

func (m *_RequestContext) SetHeader(name, value string) {
	m.w.Header().Set(name, value)
}

func (m *_RequestContext) Writer() http.ResponseWriter {
	return m.w
}

func (m *_RequestContext) Send(status int, contentType string, dataOpt []byte) {
	if contentType != "" {
		m.w.Header().Set(HeaderContentType, contentType)
	}
	m.w.WriteHeader(status)
	if len(dataOpt) > 0 {
		if _, writeErr := m.w.Write(dataOpt); writeErr != nil {
			// здесь нет смысла возвращать error, поскольку будет попытка переотправить новый ответ, а она провалится
			m.log.Warnf("Error writing: %s", writeErr.Error())
		}
	}
}

func (m *_RequestContext) SendText(status int, text string) {
	m.Send(status, ContentTypeText, []byte(text))
}

func (m *_RequestContext) SendJSON(status int, obj interface{}) {
	d, err := json.Marshal(obj)
	if err != nil {
		m.log.Warnf("%s %s: error marshalling object %v: %s", m.r.Method, m.r.URL.Path, obj, err.Error())
		m.SendText(http.StatusInternalServerError, "Marshalling error")
		return
	}
	m.Send(status, ContentTypeJSON, d)
}

// эта ошибка не моэет возникнуть в JSON RPC, поэтому там 0
var unsupportedContentType = xerror.NewCustom(http.StatusUnsupportedMediaType, 0, "Invalid Content-Type. Expected 'application/json'")

func (m *_RequestContext) ReadJSONBody(dest interface{}) xerror.IError {
	if m.r.Header.Get(HeaderContentType) != ContentTypeJSON {
		return unsupportedContentType
	}

	body, err := io.ReadAll(m.r.Body)
	if err != nil {
		return xerror.WrapFailure(err)
	}

	if err := json.Unmarshal(body, dest); err != nil {
		return xerror.WrapBadRequest(err)
	}
	return nil
}

/*func (m *_RequestContext) GetQueryParamInt(name string) (int, IHttpError) {
	v, ok := m.r.URL.Query()[name]
	if !ok || len(v) == 0 {
		return 0, NewErrorPub(http.StatusBadRequest, fmt.Sprintf("missing '%s' query parameter", name))
	}

	strVal := v[0]
	intVal, err := strconv.Atoi(strVal)
	if err != nil {
		return 0, NewErrorPub(http.StatusBadRequest, fmt.Sprintf("query parameter '%s=%s' is not a valid integer", name, strVal))
	}

	return intVal, nil
}*/

/*
func (m *_RequestContext) GetOptionalQueryParamInt(name string, defaultValue, min, max int) (int, xerror.IError) {
	v, ok := m.r.URL.Query()[name]
	if !ok || len(v) == 0 {
		return defaultValue, nil
	}
	strVal := v[0]
	intVal, err := strconv.Atoi(strVal)
	if err != nil {
		return 0, NewErrorPub(http.StatusBadRequest, fmt.Sprintf("query parameter '%s=%s' is not a valid integer", name, strVal))
	}

	if intVal < min || intVal > max {
		return 0, NewErrorPub(http.StatusBadRequest, fmt.Sprintf("query parameter '%s=%s' is invalid. Should be [%d .. %d]", name, strVal, min, max))
	}

	return intVal, nil
}
*/
