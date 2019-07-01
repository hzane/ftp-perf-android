package main

import (
    "fmt"
    "io"
    "regexp"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/paulbellamy/ratecounter"
)

// Measurement ...
type Measurement struct {
	Err        error   `json:"err,omitempty"`
	StartTime  int64   `json:"start-time,omitempty"`
	FinishTime int64   `json:"finish-time,omitempty"`
	Bandwidth  int64   `json:"bandwidth,omitempty"`   //
	UserStatus int     `json:"user-status,omitempty"` // 0: unset,
	TransBytes uint64  `json:"trans-bytes,omitempty"` // 0: unset
	FileLength uint64  `json:"file-length,omitempty"` // 0: unset
	Permillage uint64  `json:"permillage,omitempty"`  //
	Speed      float64 `json:"speed,omitempty"`       //
	Unit       string  `json:"unit,omitempty"`        //
	MaxBPS     int64   `json:"max-bps,omitempty"`     //
	BPS        int64   `json:"bps,omitempty"`         // bps
	AverageBPS int64   `json:"average-bps,omitempty"` //
	Started    int     `json:"started,omitempty"`     // 0: unset, 1: Started
	Ended      int     `json:"ended,omitempty"`       // : unset
	TimeUp     int     `json:"timeup,omitempty"`      // : unset
	Status     string  `json:"status,omitempty"`      // last Status
	band       int64
}

// Gauge ...
type Gauge struct {
	Measurement
	lock             sync.Mutex
	counter          *ratecounter.RateCounter
	logger           io.Writer
	duration         time.Duration
	timeSessionStart time.Time
}

func (g *Gauge) measure() (v Measurement) {
	g.lock.Lock()
	v = g.Measurement
	g.lock.Unlock()
	return
}

func (g *Gauge) login(code int, line string) error {
	g.lock.Lock()
	defer g.lock.Unlock()

	g.UserStatus = code
	return g.Err
}

func (g *Gauge) fileSize(sz uint64, line string) error {
	g.lock.Lock()
	defer g.lock.Unlock()
	// g.FileLength = sz
	return g.Err
}

func (g *Gauge) start(line string) error {
	g.lock.Lock()
	defer g.lock.Unlock()

	g.StartTime, g.Started = time.Now().Unix(), 1

	if g.timeSessionStart.Unix() < 0 {
		g.timeSessionStart = time.Now()
	}
	return g.Err
}

func (g *Gauge) finish(line string) error {
	g.lock.Lock()
	defer g.lock.Unlock()

	g.FinishTime, g.Status = time.Now().Unix(), line
	return g.Err
}

func (g *Gauge) end(line string) (err error) {
	if line == "" {
		line = "ends"
	}
	err = g.Err
	g.lock.Lock()
	g.Ended = 1
	g.lock.Unlock()

	_, _ = fmt.Fprintln(g.logger, "goftp: "+line)
	return
}

func (g *Gauge) terminate() (err error) {
	g.lock.Lock()
	if g.Err == nil {
		g.Err = io.ErrUnexpectedEOF
	}
	err = g.Err
	g.lock.Unlock()

	return
}

func (g *Gauge) status(line string) (err error) {
	g.lock.Lock()
	err = g.Err
	g.Status = line
	g.lock.Unlock()

    if g.isTimeUp() {
        if m, _ := regexp.MatchString(`#\d+\serror\s`, line); m{
            return
        }
    }
	_, _ = fmt.Fprintln(g.logger, line)
	return
}
func (g *Gauge) isTimeUp() bool {
    g.lock.Lock()
    defer g.lock.Unlock()

    if g.timeSessionStart.Unix()<0{
        return false
    }
	deadline := g.timeSessionStart.Add(g.duration)
	ret := g.duration > 0 && deadline.Before(time.Now())
	return ret
}

func (g *Gauge) progress(n int) error {
	g.counter.Incr(int64(n))
	g.lock.Lock()

	g.TransBytes += uint64(n)
	g.BPS = g.counter.Rate() << 3

	if g.FileLength > 0 {
		g.Permillage = g.TransBytes * 1000 / g.FileLength
	}
	if x := time.Now().Unix() - g.StartTime; x > 0 {
		g.AverageBPS = int64(g.TransBytes<<3) / x
	}
	if g.MaxBPS < g.BPS {
		g.MaxBPS = g.BPS
	}

	k := g.AverageBPS >> 10
	if k > g.band {
		g.band = k
		g.Bandwidth = calculateBandwidth(k)
	}
	if k>>10 > 0 {
		g.Speed = float64(g.BPS) / 1000 / 1000
		g.Unit = "mbps"
	} else {
		g.Speed = float64(g.BPS) / 1000
		g.Unit = "kbps"
	}
    g.lock.Unlock()

	_, _ = fmt.Fprintf(g.logger, "measure:\t%d\t%d\t%.2f\t%s\t%d\t%d\n",
		g.BPS, g.AverageBPS, g.Speed, g.Unit, g.Permillage,
		g.TransBytes)

	if g.isTimeUp() {
		g.Err = fmt.Errorf("time is up")
	}

	return g.Err
}

func calculateBandwidth(k int64) (bw int64) {
	if k/1000 > 0 {
		k = k / 1000
		switch k / 100 { // 100m
		case 0:
		case 1:
			bw = 220
		case 2, 3, 4, 5, 6:
			bw = 550
		default:
			bw = 1100
		}

		switch k / 10 {
		case 0:
			bw = 11
		case 1, 2, 3:
			bw = 22
		case 4, 5, 6, 7, 8, 9, 10, 11:
			bw = 110
		case 12:
			bw = 220
		}
	} else {
		switch k / 100 {
		case 0:
			bw = 110
		case 1, 2, 3, 4:
			bw = 550
		default:
			bw = 1100
		}
	}
	return
}

// FTPLogger ...
type FTPLogger struct {
	g *Gauge
}

// Write ...
func (l *FTPLogger) Write(p []byte) (n int, err error) {
	n = len(p)
	line := strings.ToLower(string(p))
	line = strings.TrimRight(line, "\r\n")

	if !strings.Contains(line, "goftp: ") {
		return
	}

	err = l.g.status(line)

	if strings.Contains(line, " closing") {
		err = l.g.finish(line)
		return
	}
	if !strings.Contains(line, " got ") {
		return
	}

	if strings.Contains(line, " 530-") { // login incorrect
		err = l.g.login(530, line)
	}
	if strings.Contains(line, " 230-") { // logged in
		err = l.g.login(230, line)
	}
	if strings.Contains(line, " 421-") { // service unavailable
		err = l.g.login(421, line)
	}
	if strings.Contains(line, " 500-") { // syntax error
		err = l.g.login(500, line)
	}
	if strings.Contains(line, " 501-") { // parameter error
		err = l.g.login(501, line)
	}
	if strings.Contains(line, " 150-") { // file open
		err = l.g.start(line)
	}
	if strings.Contains(line, " 213-") { // size
		pos := strings.Index(line, "got 213-")
		x := line[pos+8:]
		if sz, err := strconv.ParseUint(x, 10, 64); err == nil {
			err = l.g.fileSize(sz, line)
		}
	}
	return
}

// FTPData ...
type FTPData struct {
	g      *Gauge
	size   uint64 // for upload
	offset uint64
}

// Write ...
func (f *FTPData) Write(p []byte) (int, error) {
	f.offset += uint64(len(p))
	err := f.g.progress(len(p))
	return len(p), err
}

// Read ...
func (f *FTPData) Read(p []byte) (n int, err error) {
	left := f.size - f.offset
	n = len(p)
	if uint64(n) > left {
		n = int(left)
	}
	if n <= 0 {
		err = io.EOF
	}
	f.offset += uint64(n)
	if err == nil {
		err = f.g.progress(n)
	}

	return
}
