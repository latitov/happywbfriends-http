package vat

import (
	"errors"
	"github.com/happywbfriends/http/hollander"
	"github.com/happywbfriends/nano/xerror"
	"net/http"
	"reflect"
)

/*
	Vat используется для разделения транспорта и бизнес-логики.
	Вы пишете бизнес-логику, Vat делает из нее транспортные хендлеры (сейчас поддерживается Hollander HTTP handler)

	Пример
	------

	Рассмотрим обработчик, где смешивается транспорт и бизнес.

	type Params struct {
		Price int `json:"price"`
	}

	func mixed(r *http.Request, mw hollander.IMiddleware) (proceed bool, e xerror.IError) {
		var bodyParams Params
		if httpErr := mw.ReadJSONBody(&bodyParams); httpErr != nil {
			return false, httpErr
		}

		// А это параметр из хедера
		supplierId := r.Headers.Get("X-Supplier-Id")

		// Это типа бизнес-логика: печатать в консоль
		fmt.Printf("%s changed price to %d", supplierId, bodyParams.Price)
	}

	router.Handle(http.MethodPost, "/setPrice", hollander.NewMiddleware(log).Use(mixed))


	Теперь переделаем как надо.

	1. Все параметры бизнес-метода объявляем как поля структуры
	type Request struct {
		SupplierId 	string 	`header:"X-Supplier-Id"`	// здесь тег headers говорит, что параметр будет браться из HTTP заголовков
		Price 		int 	`json:"price"`
	}

	2. Объявляем бизнес-метод (сигнатура всегда одна)
	func business(req *Request, rc IRequestContext) (resp *None, e xerror.IError) {
		// Это типа бизнес-логика: печатать в консоль
		fmt.Printf("%s changed price to %d", req.SupplierId, req.Price)
	}

	3. Делаем из него Vat-обертку
	router.Handle(http.MethodPost, "/setPrice", hollander.NewMiddleware(log).
		Use(New[AllParams, None](business).Handler()))
*/

const (
	tagQuery   = "query"
	tagHeader  = "header"
	tagCookie  = "cookie"
	tagContext = "context"
	tagBody    = "json"
)

type Vat[RequestT, ResponseT any] struct {
	readBody      bool
	readHeaders   bool
	readCookie    bool
	readContext   bool
	readQuery     bool
	handler       func(*RequestT, IRequestContext) (*ResponseT, xerror.IError)
	cachedHandler func(*RequestT, IRequestContext) (*ResponseT, []byte, xerror.IError)
}

type None struct{}

func New[RequestT, ResponseT any](h func(*RequestT, IRequestContext) (*ResponseT, xerror.IError)) *Vat[RequestT, ResponseT] {

	var req RequestT
	refType := reflect.TypeOf(req)

	var query, header, cookie, context, body bool
	for i := 0; i < refType.NumField(); i++ {
		tf := refType.Field(i)
		if _, ok := tf.Tag.Lookup(tagQuery); ok {
			query = true
		}
		if _, ok := tf.Tag.Lookup(tagHeader); ok {
			header = true
		}
		if _, ok := tf.Tag.Lookup(tagCookie); ok {
			cookie = true
		}
		if _, ok := tf.Tag.Lookup(tagContext); ok {
			context = true
		}
		if _, ok := tf.Tag.Lookup(tagBody); ok {
			body = true
		}
	}

	return &Vat[RequestT, ResponseT]{
		readBody:      body,
		readHeaders:   header,
		readCookie:    cookie,
		readContext:   context,
		readQuery:     query,
		handler:       h,
		cachedHandler: nil,
	}
}

func NewCached[RequestT, ResponseT any](h func(*RequestT, IRequestContext) (*ResponseT, []byte, xerror.IError)) *Vat[RequestT, ResponseT] {
	var req RequestT
	refType := reflect.TypeOf(req)

	var query, header, cookie, context, body bool
	for i := 0; i < refType.NumField(); i++ {
		tf := refType.Field(i)
		if _, ok := tf.Tag.Lookup(tagQuery); ok {
			query = true
		}
		if _, ok := tf.Tag.Lookup(tagHeader); ok {
			header = true
		}
		if _, ok := tf.Tag.Lookup(tagCookie); ok {
			cookie = true
		}
		if _, ok := tf.Tag.Lookup(tagContext); ok {
			context = true
		}
		if _, ok := tf.Tag.Lookup(tagBody); ok {
			body = true
		}
	}

	return &Vat[RequestT, ResponseT]{
		readBody:      body,
		readHeaders:   header,
		readCookie:    cookie,
		readContext:   context,
		readQuery:     query,
		handler:       nil,
		cachedHandler: h,
	}
}

func (c *Vat[RequestT, ResponseT]) Handler() hollander.HttpHandler {
	return func(r *http.Request, mw hollander.IMiddleware) (proceed bool, e xerror.IError) {
		var req RequestT

		if c.readBody {
			if xe := mw.ReadJSONBody(&req); xe != nil {
				return false, xe
			}
		}

		if c.readHeaders {
			_, err := Enrich(&req, tagHeader, func(name string) (value any, found bool) {
				value = r.Header.Get(name)
				return value, value != ""
			})
			if err != nil {
				return false, xerror.NewBadRequestDetailed("error compiling request", err.Error())
			}
		}

		if c.readCookie {
			_, err := Enrich(&req, tagCookie, func(name string) (value any, found bool) {
				cookie, err := r.Cookie(name)
				if err != nil {
					if errors.Is(err, http.ErrNoCookie) {
						return nil, false
					}
					return nil, false // как бы других ошибок вроде и нет
				}
				if cookie == nil {
					return nil, false // вроде не должно быть такого, но мало ли
				}
				return cookie.Value, true
			})
			if err != nil {
				return false, xerror.NewBadRequestDetailed("error compiling request", err.Error())
			}
		}

		if c.readContext {
			_, err := Enrich(&req, tagContext, func(name string) (value any, found bool) {
				value, found = mw.Values()[name]
				return
			})
			if err != nil {
				return false, xerror.NewBadRequestDetailed("error compiling request", err.Error())
			}
		}

		if c.readQuery {
			_, err := Enrich(&req, tagQuery, func(name string) (value any, found bool) {
				values, _found := r.URL.Query()[name]
				if _found && len(values) > 0 {
					return values[0], true
				}
				return nil, false
			})
			if err != nil {
				return false, xerror.NewBadRequestDetailed("error compiling request", err.Error())
			}
		}

		rc := vatRequestContext{
			rc: mw,
		}

		var result *ResponseT
		var cachedResponse []byte
		var herr xerror.IError

		if c.cachedHandler != nil {
			result, cachedResponse, herr = c.cachedHandler(&req, &rc)
		} else {
			result, herr = c.handler(&req, &rc)
		}

		if herr != nil {
			return false, herr
		}
		if cachedResponse != nil {
			mw.Send(http.StatusOK, hollander.ContentTypeJSON, cachedResponse)
		} else {
			mw.SendJSON(http.StatusOK, result)
		}

		return false, nil
	}
}
