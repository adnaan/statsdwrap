// Package statsdwrap exposes wrappers for http.Handler and http.HandlerFunc which send
// metrics to statsd.
// Usage:
// 		r := chi.NewRouter()
// 		statsdClient, _ := statsd.New(
// 		statsd.Prefix("myapp"),
// 		statsd.Address("localhost:8125"),
// 		)
// 		wrap := statsdwrap.NewChi("user_service", statsdClient)
// 		handleHome := func(w http.ResponseWriter, r *http.Request) {
// 			time.Sleep(time.Millisecond * 1000)
// 			w.WriteHeader(http.StatusOK)
// 			w.Write([]byte("OK"))
// 		}
// 		r.Get(wrap.HandlerFunc("home", "/", handleHome))
package statsdwrap

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/pressly/chi/middleware"
	"gopkg.in/alexcesaro/statsd.v2"
)

// HandlerWrapper ...
type HandlerWrapper interface {
	Handler(routeName string, pattern string, handler http.Handler) (string, http.Handler)
	HandlerFunc(routeName string, pattern string, handlerFunc http.HandlerFunc) (string, http.HandlerFunc)
}

// HTTPTxn a single http transaction record
type HTTPTxn interface {
	Write(status int)
	End()
}

// NewChi statsd wrapper client. Usage: NewChi("acme",statsdClient). The wrapper sends the metrics: response_time,
// count and status<HTTPStatusCode>.count. e.g. :
// acme.home.response_time where home is the route name
// acme.home.count
// acme.home.status404.count
func NewChi(prefix string, statsdClient *statsd.Client) HandlerWrapper {
	var cloneStatsdClient *statsd.Client
	if prefix == "" {
		cloneStatsdClient = statsdClient.Clone()
	} else {
		cloneStatsdClient = statsdClient.Clone(statsd.Prefix(prefix))
	}
	return &defaultWrapper{
		client: cloneStatsdClient,
	}

}

// defaultWrapper statsd client
type defaultWrapper struct {
	client *statsd.Client
}

// startTransaction ...
func (d *defaultWrapper) startTransaction(name string, w middleware.WrapResponseWriter, r *http.Request) HTTPTxn {
	entry := &httpTxn{
		name:               name,
		timing:             d.client.NewTiming(),
		responseTimeBucket: strings.Join([]string{name, "response_time"}, "."),
		hitsBucket:         strings.Join([]string{name, "count"}, "."),
		defaultWrapper:     d,
		ww:                 w,
		request:            r,
		buf:                &bytes.Buffer{},
	}

	return entry
}

type httpTxn struct {
	name               string
	responseTimeBucket string
	hitsBucket         string

	timing statsd.Timing
	*defaultWrapper
	ww      middleware.WrapResponseWriter
	request *http.Request
	buf     *bytes.Buffer
}

func (d *httpTxn) Write(status int) {
	d.timing.Send(d.responseTimeBucket)
	httpStatusBucket := fmt.Sprintf("%s.http%d", d.name, d.ww.Status())
	d.client.Increment(httpStatusBucket)
	d.client.Increment(d.hitsBucket)
}

func (d *httpTxn) End() {
	d.Write(d.ww.Status())
}

// Handler ...
func (d *defaultWrapper) Handler(routeName string, pattern string, handler http.Handler) (string, http.Handler) {
	return pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		txn := d.startTransaction(routeName, ww, r)
		defer txn.End()

		handler.ServeHTTP(ww, r)
	})
}

// HandlerFunc ...
func (d *defaultWrapper) HandlerFunc(routeName string, pattern string, handlerFunc http.HandlerFunc) (string, http.HandlerFunc) {
	p, h := d.Handler(routeName, pattern, handlerFunc)
	return p, func(w http.ResponseWriter, r *http.Request) { h.ServeHTTP(w, r) }
}
