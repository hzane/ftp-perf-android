package main

import (
	"encoding/json"
	"net/http"
	"net/url"
)

// PathDo ...
func PathDo(handlers ...func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Del("cache-control")
				w.Header().Set("content-type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusBadGateway)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.(error).Error()}) // nolint: gas
			}
		}()
		_ = r.ParseForm() // nolint: gas
		for _, handler := range handlers {
			handler(w, r)
		}
	}
}

// ContentJSON ...
func ContentJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.Header().Set("access-control-allow-origin", "*")
}

// ContentPlain ...
func ContentPlain(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "text/plain")
}

// MkURI ...
func MkURI(p string, opts ...func(params url.Values)) (string, error) {
	uri, err := url.Parse(p)
	if err != nil {
		return p, err
	}
	params := uri.Query()
	for _, set := range opts {
		set(params)
	}
	uri.RawQuery = params.Encode()
	return uri.String(), err
}
