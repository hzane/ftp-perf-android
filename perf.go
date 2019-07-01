package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/paulbellamy/ratecounter"

	"github.com/secsy/goftp"
)

func parseURI(user, password, host, file, method string, parallels int) (string, string,
	string,
	string, string, int) {
	uri, err := url.Parse(file)
	if err != nil {
		return user, password, host, file, method, parallels
	}
	if uri.User.Username() != "" {
		user = uri.User.Username()
	}
	if p, ok := uri.User.Password(); ok {
		password = p
	}
	if uri.Host != "" {
		host = uri.Host
	}
	file = uri.Path
	if m := uri.Query().Get("method"); m != "" {
		method = m
	}
	if p, err := strconv.Atoi(uri.Query().Get("parallels")); err == nil && p > 0 {
		parallels = p
	}
	return user, password, host, file, method, parallels
}
func ftpTransfer(g *Gauge, user, password, host, file, method string, parallels int) {
	defer func() {
		if err := recover(); err != nil {
			_ = g.finish("goftp: " + err.(error).Error())
		}
		_ = g.end("session ends")
	}()

	_, _ = fmt.Fprintln(g.logger, "goftp: current pid", os.Getpid())
	user, password, host, file, method, parallels = parseURI(user, password, host, file,
		method,
		parallels)

	settings := goftp.Config{
		User:               user,
		Password:           password,
		ConnectionsPerHost: config.ftp.parallels,
		Timeout:            config.ftp.timeout,
		Logger:             &FTPLogger{g},
	}
	if config.ftp.ePSVDisable > 0 {
		settings.DisableEPSV = true
	} else if config.ftp.ePSVDisable < 0 {
		settings.DisableEPSV = false
	}

	if config.ftp.activeTransfer > 0 {
		settings.ActiveTransfers = true
	} else if config.ftp.activeTransfer < 0 {
		settings.ActiveTransfers = false
	}

	client, err := goftp.DialConfig(settings, host)
	panice(err)
	defer func() { _ = client.Close() }()

	var filesz uint64
	if method != "upload" {
		sz, err := client.Size(file)
		panice(err)
		if sz >= 0 {
			g.FileLength += uint64(sz)
			filesz = uint64(sz)
		}
	} else {
		// d := path.Dir(file)
		file = path.Base(file)
		filesz = sizeFromName(file)
		g.FileLength += filesz
		if !strings.HasPrefix(file, "zero-") { // 避免覆盖其他人的文件
			file = "zero-" + file
		}
		file = path.Join(config.ftp.destDIR, file)
	}
	dummy := &FTPData{g: g, size: filesz, offset: 0}

	wg := &sync.WaitGroup{}

	pd := pieceDispatcher(file, filesz, parallels, dummy)
	for i := 0; i < parallels; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			transferWorker(client, pd, config.method)
		}()
	}
	wg.Wait()
}

func ftp(g *Gauge, method, user, password, host, file string) {
	defer func() {
		if err := recover(); err != nil {
			_ = g.finish("goftp: " + err.(error).Error())
		}
		_ = g.end("session ends")
	}()

	settings := goftp.Config{
		User:               user,
		Password:           password,
		ConnectionsPerHost: 5,
		Timeout:            time.Second * 10,
		Logger:             &FTPLogger{g},
	}

	client, err := goftp.DialConfig(settings, host)
	panice(err)
	defer func() { _ = client.Close() }()

	sz := sizeFromName(file)

	dummy := &FTPData{g: g, size: sz, offset: 0}
	switch method {
	case "download":
		err = client.Retrieve(file, dummy)
	case "upload":
		g.FileLength = sz
		fn := strings.TrimSuffix(file, path.Ext(file)) + ".zero"
		err = client.Store(fn, dummy)
	case "list", "dir":
		var fis []os.FileInfo
		fis, err = client.ReadDir("/")
		for _, fi := range fis {
			_, _ = fmt.Fprintln(g.logger,
				fi.Name(), "\t", fi.IsDir(), "\t",
				fi.Size(), "\t", fi.Mode())
		}
	}
	panice(err)
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
	if config.method == "ping" {
		mainPing()
		return
	}
	if config.method == "speedtest" {
		mainSpeedTest2()
		return
	}
	isDownload := config.method != "upload"

	for i := 0; i < config.ftp.count; i++ {
		wg := &sync.WaitGroup{}
		gauge := &Gauge{
			duration: config.ftp.duration,
			counter:  ratecounter.NewRateCounter(time.Second),
			logger:   os.Stdout,
		}
		for _, uri := range uris(config.ftp.uris) {
			wg.Add(1)
			go func(uri string) {
				defer wg.Done()
				ftpTransfer(gauge,
					if2(isDownload, config.downloadUser, config.uploadUser),
					if2(isDownload, config.downloadPassword, config.uploadPassword),
					config.host,
					uri,
					config.method,
					config.ftp.parallels)
			}(uri)
		}
		wg.Wait()
	}
	fmt.Println("goftp: tasks ends")
}

func uris(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ';' || r == ',' || r == '|'
	})
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

func init() {

	flag.StringVar(&config.host, "host", "218.203.61.198:21", "host:port")
	flag.StringVar(&config.downloadUser, "download-user", "heilongjiangdl", "user name")
	flag.StringVar(&config.downloadPassword, "download-password", "hlj!@#$%hlj", "password")
	/*
	   flag.StringVar(&config.uploadUser, "upload-user", "heilongjiangul", "")
	   flag.StringVar(&config.uploadPassword, "upload-password", "hlj!@#$", "")
	*/

	/*
	   flag.StringVar(&config.host, "host", "221.130.36.146:21", "host:port")
	   flag.StringVar(&config.downloadUser, "download-user", "ceshi", "user name")
	   flag.StringVar(&config.downloadPassword, "download-password", "ceshi", "password")
	*/
	// flag.StringVar(&config.host, "host", "112.53.75.102:21", "host:port")
	flag.StringVar(&config.uploadUser, "upload-user", "rztest", "")
	flag.StringVar(&config.uploadPassword, "upload-password", "Ytbz@201902", "")

	flag.StringVar(&config.file, "file", "1M.rar", "download file")
	flag.StringVar(&config.ftpAddr, "ftp-addr", "", "0.0.0.0:18101")
	flag.StringVar(&config.httpAddr, "http-addr", "", "0.0.0.0:18103")
	flag.StringVar(&config.method, "method", "download", "download / upload / list")
	flag.StringVar(&config.assetsDir, "assets-dir", "/data/data/assets/", "")
	flag.BoolVar(&config.verbose, "verbose", false, "")

	flag.StringVar(&config.ftp.destDIR, "ftp-dest-dir", "upload", "")
	flag.StringVar(&config.ftp.uris, "ftp-uris", "1M.rar", "")
	flag.IntVar(&config.ftp.concurrent, "ftp-concurrent", 1, "")
	flag.IntVar(&config.ftp.count, "ftp-count", 1, "")
	flag.IntVar(&config.ftp.parallels, "ftp-parallels", 5, "")
	flag.IntVar(&config.ftp.activeTransfer, "ftp-act-transfer", 0, "-1: disable, 0:unset, 1:enable")
	flag.IntVar(&config.ftp.ePSVDisable, "ftp-epsv-disable", 0,
		"-1: epsv-disable=false, 0: unset, 1:epsv-disable=true")
	flag.DurationVar(&config.ftp.duration, "ftp-duration", 0, "")
	flag.DurationVar(&config.ftp.timeout, "ftp-timeout", time.Second*30, "")

	flag.StringVar(&config.ping.host, "ping-host", "", "separated by comma")
	flag.IntVar(&config.ping.count, "ping-count", 1, "")
	flag.IntVar(&config.ping.size, "ping-size", 8, "bytes")
	flag.DurationVar(&config.ping.interval, "ping-interval", time.Millisecond*200, "")
	flag.DurationVar(&config.ping.timeout, "ping-timeout", 0, "")
	flag.DurationVar(&config.ping.duration, "ping-duration", 0, "")
	flag.BoolVar(&config.ping.privileged, "ping-privileged", false, "non-privileged ICMP")

	flag.StringVar(&config.sperf.servers, "speed-servers",
		"https://www.speedtest.net/api/js/servers?engine=js&https_functional=0", "")
	flag.StringVar(&config.sperf.referer, "speed-referer", "https://www.speedtest.net/",
		"")
	flag.IntVar(&config.sperf.topN, "speed-top-n", 2, "")
	flag.IntVar(&config.sperf.parallels, "speed-parallels", 3, "")
	flag.Int64Var(&config.sperf.blockSize, "speed-block-size", 32<<10, "")
	flag.Int64Var(&config.sperf.transBytes, "speed-transmit-bytes", 32<<20, "")
	flag.DurationVar(&config.sperf.duration, "speed-duration", time.Second*30, "")
	flag.DurationVar(&config.sperf.timeout, "speed-timeout", time.Second*15, "")

	// 	speedtest := NewSpeedtest()
	flag.BoolVar(&speedtest.CliFlags.Json, "speedtest-json", false,
		"Suppress verbose output, "+
			"only show basic information in JSON format")
	flag.BoolVar(&speedtest.CliFlags.Xml, "speedtest-xml", false,
		"Suppress verbose output, "+
			"only show basic information in XML format")
	flag.BoolVar(&speedtest.CliFlags.Csv, "speedtest-csv", false,
		"Suppress verbose output, "+
			"only show basic information in CSV format")
	flag.BoolVar(&speedtest.CliFlags.Simple, "speedtest-simple", true,
		"Suppress verbose output, "+
			"only show basic information")
	flag.BoolVar(&speedtest.CliFlags.List, "speedtest-list", false,
		"Display a list of speedtest.net servers sorted by distance")
	flag.IntVar(&speedtest.CliFlags.Server, "speedtest-server", 0,
		"Specify a server ID to test against")
	flag.StringVar(&speedtest.CliFlags.Source, "speedtest-source", "",
		"Source IP address to bind to")
	flag.Int64Var(&speedtest.CliFlags.Timeout, "speedtest-timeout", 10,
		"Timeout in seconds")

	flag.Parse()

	if config.ping.timeout <= 0 {
		config.ping.timeout = time.Millisecond * 10000 * time.Duration(config.ping.count)
	}
	config.sperf.guid = uuid()
}

var config struct {
	verbose          bool
	host             string
	downloadUser     string
	downloadPassword string
	uploadUser       string
	uploadPassword   string
	file             string
	assetsDir        string
	method           string
	ftpAddr          string
	httpAddr         string
	ftp              struct {
		destDIR        string
		uris           string
		count          int
		timeout        time.Duration
		duration       time.Duration
		concurrent     int
		parallels      int
		activeTransfer int
		ePSVDisable    int
	}
	ping struct {
		host       string
		count      int
		size       int
		interval   time.Duration
		timeout    time.Duration
		duration   time.Duration
		privileged bool
	}
	sperf struct {
		guid       string
		servers    string
		referer    string
		topN       int
		parallels  int
		blockSize  int64
		transBytes int64
		duration   time.Duration
		timeout    time.Duration
	}
	downloadGauge atomic.Value
	uploadGauge   atomic.Value
}

// 218.203.61.198
// heilongjiangdl
// hlj!@#$%hlj
// heilongjiangul
// hlj!@#$
