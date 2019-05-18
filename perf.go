package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/paulbellamy/ratecounter"
	"github.com/secsy/goftp"
	"gitlab.com/hearts.zhang/tools/httputil"
)

type Dummy struct {
	logger  io.Writer
	counter *ratecounter.RateCounter
	size    int64 // for upload
	offset  int64
}

func (w *Dummy) Write(p []byte) (int, error) {
	w.counter.Incr(int64(len(p)))
	w.offset += int64(len(p))
	_, _ = fmt.Fprintln(w.logger, w.offset, w.counter.Rate()<<3, "bps", w.counter.Rate()>>17, "mbps downloads")
	if rw, ok := w.logger.(*bufio.ReadWriter); ok {
		_ = rw.Flush()
	}
	return len(p), nil
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
		_, _ = fmt.Fprintln(w.logger, w.offset, w.counter.Rate()<<3, "bps", w.counter.Rate()>>17, "mbps uploads")
	}
	if rw, ok := w.logger.(*bufio.ReadWriter); ok {
		_ = rw.Flush()
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
	settings := goftp.Config{
		User:               config.user,
		Password:           config.password,
		ConnectionsPerHost: 10,
		Timeout:            10 * time.Second,
		Logger:             os.Stderr,
	}

	client, err := goftp.DialConfig(settings, config.host)
	if err != nil {
		panic(err)
	}
	var v datasize.ByteSize
	_ = v.UnmarshalText([]byte(config.upload))

	dummy := Dummy{
		logger:  os.Stdout,
		counter: ratecounter.NewRateCounter(time.Second),
		size:    int64(v.Bytes()),
		offset:  0,
	}
	if config.file != "" {
		err = client.Retrieve(config.file, &dummy)
	}
	if v.Bytes() > 0 {
		err = client.Store(fmt.Sprintf("dummy-%s.zero", config.upload), &dummy)
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

func ftp(w http.ResponseWriter, r *http.Request) {
	user := any(r.FormValue("user"), "heilongjiangdl")
	password := any(r.FormValue("password"), "hlj!@#$%hlj")
	host := any(r.FormValue("host"), "218.203.61.198:21")
	file := any(r.FormValue("file"), "80M.rar")
	download := any(r.FormValue("method"), "download")

	fmt.Println(password, user)

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
		size:    0, //
		offset:  0,
	}
	if download != "upload" {
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
func mainHTTP(addr string) {
	var do, plain, json = httputil.PathDo, httputil.ContentPlain, httputil.ContentJSON

	http.HandleFunc("/v1/ftp.txt", do(plain, ftp))
	http.HandleFunc("/v1/status.json", do(json, status))
	http.HandleFunc("/", do(json, status))

	_ = http.ListenAndServe(addr, nil)
}

func init() {
	flag.StringVar(&config.host, "host", "192.168.1.5:12121", "host:port")
	flag.StringVar(&config.user, "user", "heilongjiangdl", "user name")
	flag.StringVar(&config.password, "password", "hlj!@#$%hlj", "password")
	flag.StringVar(&config.file, "file", "4M.rar", "download file")
	flag.StringVar(&config.ftpAddr, "ftp-addr", "", "0.0.0.0:18101")
	flag.StringVar(&config.httpAddr, "http-addr", "", "0.0.0.0:18103")
	flag.StringVar(&config.upload, "upload", "5MB", "5KB 1MB")

	flag.Parse()
}

var config struct {
	host     string
	user     string
	password string
	file     string
	ftpAddr  string
	upload   string
	httpAddr string
}

// 218.203.61.198
// heilongjiangdl
// hlj!@#$%hlj
// heilongjiangul
// hlj!@#$
