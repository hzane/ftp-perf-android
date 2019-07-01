package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/paulbellamy/ratecounter"

	"github.com/secsy/goftp"
)

func list(w http.ResponseWriter, r *http.Request) {
	user := any(r.FormValue("user"), config.downloadUser)
	password := any(r.FormValue("password"), config.downloadPassword)
	host := any(r.FormValue("host"), config.host)

	settings := goftp.Config{
		User:               user,
		Password:           password,
		ConnectionsPerHost: 10,
		Timeout:            10 * time.Second,
		Logger:             os.Stderr,
	}

	client, err := goftp.DialConfig(settings, host)
	panice(err)
	defer func() { _ = client.Close() }()

	files, err := client.ReadDir("/")
	panice(err)
	for _, fi := range files {
		_, _ = fmt.Fprintf(w, fi.Name(), fi.Size(), fi.IsDir(), fi.ModTime())
	}
}

func status(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(`null`))
}

func download(w http.ResponseWriter, r *http.Request) {
	user := any(r.FormValue("user"), config.downloadUser)
	password := any(r.FormValue("password"), config.downloadPassword)
	host := any(r.FormValue("host"), config.host)
	file := any(r.FormValue("file"), config.file)
	method := any(r.FormValue("method"), "status")
	var gauge, _ = config.downloadGauge.Load().(*Gauge)

	uri := MkURI("/v3/download", qs(r.URL.Query()), q("method", "show"))
	switch method {
	case "restart", "start":
		if gauge != nil {
			_ = gauge.terminate()
		}
		gauge = &Gauge{
			counter: ratecounter.NewRateCounter(time.Second),
			logger:  logger(),
		}
		config.downloadGauge.Store(gauge)
		go ftp(gauge, "download", user, password, host, file)
		http.Redirect(w, r, uri, http.StatusFound)
	case "stop":
		if gauge != nil {
			_ = gauge.terminate()
		}
		http.Redirect(w, r, uri, http.StatusFound)
	case "status":
		w.Header().Set("content-type", "application/json; charset=utf-8")
		var m Measurement
		if gauge != nil {
			m = gauge.measure()
		}
		_ = json.NewEncoder(w).Encode(m)
	case "show":
		uri = MkURI("/static/gauge.html",
			qs(r.URL.Query()), q("method", "download"))
		http.Redirect(w, r, uri, http.StatusFound)
	}
}

func logger() io.Writer {
	if config.verbose {
		return os.Stderr
	}
	return ioutil.Discard
}

func upload(w http.ResponseWriter, r *http.Request) {
	user := any(r.FormValue("user"), config.uploadUser)
	password := any(r.FormValue("password"), config.uploadPassword)
	host := any(r.FormValue("host"), config.host)
	file := any(r.FormValue("file"), config.file)

	method := any(r.FormValue("method"), "status")
	var gauge, _ = config.uploadGauge.Load().(*Gauge)

	uri := MkURI("/v3/upload", qs(r.URL.Query()), q("method", "show"))
	switch method {
	case "restart", "start":
		if gauge != nil {
			_ = gauge.terminate()
		}
		gauge = &Gauge{
			counter: ratecounter.NewRateCounter(time.Second),
			logger:  logger(),
		}
		config.uploadGauge.Store(gauge)
		go ftp(gauge, "upload", user, password, host, file)

		http.Redirect(w, r, uri, http.StatusFound)
	case "stop":
		if gauge != nil {
			_ = gauge.terminate()
		}
		http.Redirect(w, r, uri, http.StatusFound)
	case "status":
		w.Header().Set("content-type", "application/json; charset=utf-8")
		var m Measurement
		if gauge != nil {
			m = gauge.measure()
		}
		_ = json.NewEncoder(w).Encode(m)
	case "show":
		uri := MkURI("/static/gauge.html", qs(r.URL.Query()), q("method", "upload"))
		http.Redirect(w, r, uri, http.StatusFound)
	}
}

func handles() http.Handler {
	var do, plain, jsn = PathDo, ContentPlain, ContentJSON

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/shutdown", do(plain, shutdown))
	mux.HandleFunc("/v1/list.txt", do(plain, list))
	mux.HandleFunc("/v3/download/", do(download)) // method=start/restart/stop/show/status
	mux.HandleFunc("/v3/upload/", do(upload))     // method=start...&file&user&password&host
	mux.Handle("/static/",
		http.StripPrefix("/static/", http.FileServer(http.Dir(config.assetsDir))))
	mux.HandleFunc("/", do(jsn, status))

	return mux
}

func signalHTTP(server *http.Server) {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGTERM)
	for {
		switch <-ch {
		case syscall.SIGTERM:
			_ = server.Shutdown(context.Background())
		}
	}
}

func mainHTTP(addr string) {
	server := &http.Server{Addr: addr, Handler: handles()}
	go signalHTTP(server)
	_ = server.ListenAndServe()
}

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

func qs(opts url.Values) func(url.Values) {
	return func(params url.Values) {
		for k, values := range opts {
			for _, val := range values {
				params.Add(k, val)
			}
		}
	}
}
func q(name, val string) func(url.Values) {
	return func(params url.Values) {
		params.Set(name, val)
	}
}

// MkURI ...
func MkURI(p string, opts ...func(params url.Values)) string {
	uri, err := url.Parse(p)
	if err != nil {
		return p
	}
	params := uri.Query()
	for _, set := range opts {
		set(params)
	}
	uri.RawQuery = params.Encode()
	return uri.String()
}
