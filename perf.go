package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/paulbellamy/ratecounter"
	"github.com/secsy/goftp"
)

// Dummy ...
type Dummy struct {
	logger  io.Writer
	counter *ratecounter.RateCounter
	size    int64 // for upload
	offset  int64
}

func (w *Dummy) Write(p []byte) (int, error) {
	w.counter.Incr(int64(len(p)))
	w.offset += int64(len(p))
	_, err := fmt.Fprintln(w.logger, w.offset, w.counter.Rate()<<3, "bps", w.counter.Rate()>>17, "mbps downloads")
	if rw, ok := w.logger.(*bufio.ReadWriter); ok {
		err = rw.Flush()
	}
	return len(p), err
}

func (w *Dummy) Read(p []byte) (n int, err error) {
	left := w.size - w.offset
	n = len(p)
	if int64(n) > left {
		n = int(left)
	}
	if n <= 0 {
		err = io.EOF
	}
	w.offset += int64(n)
	w.counter.Incr(int64(n))

	if n > 0 {
		_, err = fmt.Fprintln(w.logger, w.offset, w.counter.Rate()<<3, "bps", w.counter.Rate()>>17, "mbps uploads")
	}
	if rw, ok := w.logger.(*bufio.ReadWriter); ok {
		err = rw.Flush()
	}

	return
}

func main() {
	if config.ftpAddr != "" {
		mainSRV(config.ftpAddr)
		return
	}
	if config.httpAddr != "" {
		mainHTTP(config.httpAddr)
		return
	}
	isDownload := config.method != "upload"
	settings := goftp.Config{
		User:               if2(isDownload, config.downloadUser, config.uploadUser),
		Password:           if2(isDownload, config.downloadPassword, config.uploadPassword),
		ConnectionsPerHost: 10,
		Timeout:            10 * time.Second,
		Logger:             os.Stderr,
	}

	client, err := goftp.DialConfig(settings, config.host)
	if err != nil {
		panic(err)
	}
	sz := sizeFromName(config.file)

	dummy := Dummy{
		logger:  os.Stdout,
		counter: ratecounter.NewRateCounter(time.Second),
		size:    sz,
		offset:  0,
	}
	if isDownload {
		err = client.Retrieve(config.file, &dummy)
	} else {
		fn := strings.TrimSuffix(config.file, path.Ext(config.file)) + ".zero"
		err = client.Store(fn, &dummy)
	}
	_ = client.Close()
}

func any(strs ...string) string {
	for _, s := range strs {
		if s != "" {
			return s
		}
	}
	return ""
}

func panice(err error) {
	if err != nil {
		panic(err)
	}
}
func if2(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
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
		fmt.Fprintln(w, fi.Name(), fi.Size(), fi.IsDir(), fi.ModTime())
	}
}
func ftp(w http.ResponseWriter, r *http.Request) {
	isUpload := any(r.FormValue("method"), "download") == "upload"
	user := any(r.FormValue("user"), if2(!isUpload, config.downloadUser, config.uploadUser))
	password := any(r.FormValue("password"), if2(!isUpload, config.downloadPassword, config.uploadPassword))
	host := any(r.FormValue("host"), config.host)
	file := any(r.FormValue("file"), if2(!isUpload, "1M.rar", "1M.zero"))
	if isUpload {
		file = strings.TrimSuffix(file, path.Ext(file)) + ".zero"
	}

	hijacker, _ := w.(http.Hijacker)
	conn, writer, err := hijacker.Hijack()
	panice(err)
	defer func() { _ = conn.Close() }()

	_, _ = writer.WriteString("HTTP/1.1 200 OK\r\n\r\n")
	_ = writer.Flush()

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

	dummy := Dummy{
		logger:  writer,
		counter: ratecounter.NewRateCounter(time.Second),
		size:    sizeFromName(file), //
		offset:  0,
	}
	if !isUpload {
		err = client.Retrieve(file, &dummy)
	} else {
		err = client.Store(file, &dummy)
	}
	panice(err)
	_, _ = fmt.Fprintln(writer, dummy.offset, dummy.counter.Rate(), "bps", dummy.counter.Rate()>>17, "mbps", "ends")
	_ = writer.Flush()
}

func status(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("{}"))
}

func shutdown(w http.ResponseWriter, r *http.Request) {
	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
}

func handles() http.Handler {
	var do, plain, json = PathDo, ContentPlain, ContentJSON

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/ftp.txt", do(plain, ftp))
	mux.HandleFunc("/v1/status.json", do(json, status))
	mux.HandleFunc("/v1/shutdown", do(plain, shutdown))
	mux.HandleFunc("/v1/list.txt", do(plain, list))
	mux.HandleFunc("/", do(json, status))

	return mux
}

func signalHTTP(server *http.Server) {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGTERM)
	for {
		switch <-ch {
		case syscall.SIGTERM:
			server.Shutdown(context.Background())
		}
	}
}

func mainHTTP(addr string) {
	server := &http.Server{Addr: addr, Handler: handles()}
	go signalHTTP(server)
	server.ListenAndServe()
}

func init() {
	flag.StringVar(&config.host, "host", "218.203.61.198:21", "host:port")
	flag.StringVar(&config.downloadUser, "download-user", "heilongjiangdl", "user name")
	flag.StringVar(&config.downloadPassword, "download-password", "hlj!@#$%hlj", "password")
	flag.StringVar(&config.uploadUser, "upload-user", "heilongjiangul", "")
	flag.StringVar(&config.uploadPassword, "upload-password", "hlj!@#$", "")
	flag.StringVar(&config.file, "file", "1M.rar", "download file")
	flag.StringVar(&config.ftpAddr, "ftp-addr", "", "0.0.0.0:18101")
	flag.StringVar(&config.httpAddr, "http-addr", "", "0.0.0.0:18103")
	flag.StringVar(&config.method, "method", "download", "5KB 1MB")

	flag.Parse()
}

var config struct {
	host             string
	downloadUser     string
	downloadPassword string
	uploadUser       string
	uploadPassword   string
	file             string
	ftpAddr          string
	method           string
	httpAddr         string
}

// 218.203.61.198
// heilongjiangdl
// hlj!@#$%hlj
// heilongjiangul
// hlj!@#$
