package main

import (
    "bytes"
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "io/ioutil"
    "math/rand"
    "net"
    "net/http"
    "net/http/cookiejar"
    "sync"
    "time"

    "golang.org/x/net/publicsuffix"
)

type model struct {
    URL             string `json:"url,omitempty"`
    LAT             string `json:"lat,omitempty"`
    LON             string `json:"lon,omitempty"`
    Name            string `json:"name,omitempty"`
    Country         string `json:"country,omitempty"`
    CC              string `json:"cc,omitempty"`
    Sponsor         string `json:"sponsor,omitempty"`
    ID              string `json:"id,omitempty"`
    Host            string `json:"host,omitempty"`
    HTTPSFunctional int    `json:"https_functional,omitempty"`
    Preferred       int    `json:"preferred,omitempty"`
    Distance        int    `json:"distance,omitempty"`
    rtt             time.Duration
}

func mainSpeedTest() {
    client := NewHTTPClient(config.sperf.timeout)
    var servers []model
    err := GetOBJ(client, config.sperf.servers, config.sperf.referer, &servers)
    if err != nil {
        panic(err)
    }
    var candis []model
    for _, server := range servers {
        if server.Country == "China" {
            candis = append(candis, server)
        }
    }
    candis = TopN(client, candis)
    for _, server := range candis {
        fmt.Println(server.Country, server.Host, server.rtt)
    }

    DownloadBench(client, config.sperf.transBytes, config.sperf.duration, candis)
    UploadBench(client, config.sperf.transBytes, config.sperf.duration, candis)
}
func UploadBench(client *http.Client,
    xfer int64,
    timeo time.Duration,
    servers []model) {
    pipe := make(chan model, len(servers)*config.sperf.parallels)
    wg := &sync.WaitGroup{}
    wg.Add(1)
    go func() {
        defer wg.Done()
        defer close(pipe)
        if timeo == 0 {
            timeo = time.Hour * 24
        }
        if xfer <= 0 {
            xfer = 1 << 40
        }
        since, idx := time.Now(), 0
        for time.Now().Sub(since) < timeo && xfer > 0 {
            pipe <- servers[idx]
            idx = (idx + 1) % len(servers)
            xfer = xfer - config.sperf.blockSize
        }
    }()
    dummy := make([]byte, config.sperf.blockSize)
    for i := 0; i < config.sperf.parallels*config.sperf.topN; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for server := range pipe {
                _, err := Hello(client, server)
                if err != nil {
                    time.Sleep(time.Second) // avoid high cpu, when errors
                    continue
                }
                uri := URI("upload", server)
                request, _ := http.NewRequest("POST",
                    uri,
                    bytes.NewReader(dummy))
                request.Header.Set("content-type", "application/octet-stream")
                request.Header.Set("referer", config.sperf.referer)
                request.Header.Set("Sec-Fetch-Mode", "cors")
                since := time.Now()
                resp, err := client.Do(request)
                if err != nil {
                    fmt.Println(uri, err)
                    time.Sleep(time.Second) // avoid high cpu, when errors
                    continue
                }
                n, err := io.Copy(ioutil.Discard, resp.Body)
                rtt := time.Now().Sub(since)
                _ = resp.Body.Close()
                fmt.Println("upload", server.Host, n, rtt, err)
            }
        }()
    }
    wg.Wait()
}
func DownloadBench(client *http.Client,
    xfer int64,
    timeo time.Duration,
    servers []model) {

    pipe := make(chan model, len(servers)*config.sperf.parallels)
    wg := &sync.WaitGroup{}
    wg.Add(1)
    go func() {
        defer wg.Done()
        defer close(pipe)
        if timeo == 0 {
            timeo = time.Hour * 24
        }
        if xfer <= 0 {
            xfer = 1 << 40
        }
        since, idx := time.Now(), 0
        elapsed := time.Now().Sub(since)
        for elapsed < timeo && xfer > 0 {
            server := servers[idx]
            pipe <- server
            idx = (idx + 1) % len(servers)
            xfer = xfer - config.sperf.blockSize
            elapsed = time.Now().Sub(since)
        }
    }()

    for i := 0; i < config.sperf.parallels*config.sperf.topN; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for server := range pipe {
                _, err := Hello(client, server)
                if err != nil {
                    time.Sleep(time.Second) // avoid high cpu, when errors
                    continue
                }
                rtt, err := Dummy(client, DownloadURI(server))
                fmt.Println("download", server.Host, rtt, err)
            }
        }()
    }
    wg.Wait()
}

// URI ...
func URI(method string, server model) string {
    scheme := "http"
    if server.HTTPSFunctional > 0 {
        scheme = "https"
    }
    uri := fmt.Sprintf("%s://%s/%s?nocache=%s&guid=%s",
        scheme,
        server.Host,
        method,
        uuid(),
        config.sperf.guid)
    return uri
}
func DownloadURI(server model) string {
    scheme := "http"
    if server.HTTPSFunctional > 0 {
        scheme = "https"
    }
    uri := fmt.Sprintf("%s://%s/%s?nocache=%s&size=%d&guid=%s",
        scheme,
        server.Host,
        "download",
        uuid(),
        config.sperf.blockSize,
        config.sperf.guid)
    return uri
}
func Dummy(client *http.Client, uri string) (rtt time.Duration, err error) {
    request, err := http.NewRequest("GET", uri, nil)
    if err != nil {
        return
    }
    request.Header.Set("referer", config.sperf.referer)
    request.Header.Set("Sec-Fetch-Mode", "cors")
    since := time.Now()
    resp, err := client.Do(request)
    if err != nil {
        return
    }
    defer func() { _ = resp.Body.Close() }()
    _, err = io.Copy(ioutil.Discard, resp.Body)
    rtt = time.Now().Sub(since)
    return
}
func Hello(client *http.Client, server model) (rtt time.Duration, err error) {
    return Dummy(client, URI("hello", server))
}
func TopN(client *http.Client, candis []model) []model {
    lock := &sync.Mutex{}

    wg := &sync.WaitGroup{}

    pipe := make(chan model, len(candis))
    wg.Add(1)
    go func() {
        defer wg.Done()
        for _, server := range candis {
            pipe <- server
        }
        close(pipe)
    }()

    var result []model
    for i := 0; i < config.sperf.parallels*config.sperf.topN; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for server := range pipe {
                if rtt, err := Hello(client, server); err == nil {
                    server.rtt = rtt
                    fmt.Println(server.Country, server.Host, server.rtt)
                    lock.Lock()
                    result = append(result, server)
                    lock.Unlock()
                }
            }
        }()
    }
    wg.Wait()

    if len(result) > config.sperf.topN {
        result = result[:config.sperf.topN]
    }
    return result
}

func uuid() string {
    b := make([]byte, 16)
    _, _ = rand.Read(b)
    return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func GetOBJ(client *http.Client, uri, referer string, ret interface{}) (err error) {
    req, err := http.NewRequest("GET", uri, nil)
    if err != nil {
        return
    }
    req.Header.Set("Sec-Fetch-Mode", "cors")

    if referer != "" {
        req.Header.Set("referer", referer)
    }
    resp, err := client.Do(req)
    if err != nil {
        return
    }
    defer func() { _ = resp.Body.Close() }()
    err = json.NewDecoder(resp.Body).Decode(ret)
    return
}

// GetJSON ...
func GetJSON(client *http.Client, uri, referer string) (docs []map[string]interface{},
    err error) {
    err = GetOBJ(client, uri, referer, &docs)
    return
}

// NewHTTPClient ...
func NewHTTPClient(timeout time.Duration) *http.Client {
    var trans = &http.Transport{
        DialContext: (&net.Dialer{
            Timeout: 5 * time.Second,
        }).DialContext,
        TLSHandshakeTimeout: 5 * time.Second,
    }
    jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
    ret := &http.Client{
        Jar:       jar,
        Transport: trans,
    }
    if timeout > 0 {
        ret.Timeout = timeout
    }
    return ret
}
func init() {

    flag.Parse()
}

const winc = `Mozilla/5.0 (Windows NT 6.1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.%d.%d.%d Safari/%d.%d`

func winchrome() string {
    v1, v2, v3, v4 := rand.Intn(40)+27, rand.Intn(10), rand.Intn(4000), rand.Intn(1000)
    s1, s2 := rand.Intn(30)+510, rand.Intn(50)
    return fmt.Sprintf(winc, v1, v2, v3, v4, s1, s2)
}

// https://vipspeedtest4.wuhan.net.cn:8080/download?nocache=85cdd0c7-394a-409d-8262-14b1e850fe43&size=25000000&guid=d93d40a0-0db9-43dd-9c02-f8dc7b07ee7d
// https://vipspeedtest4.wuhan.net.cn:8080/upload?nocache=102c068d-012e-453b-911b-50f27c2641ee&guid=d93d40a0-0db9-43dd-9c02-f8dc7b07ee7d
// https://vipspeedtest4.wuhan.net.cn:8080/hello?nocache=e2762424-b1bc-4ade-8479-f2fdfa44fad9&guid=d93d40a0-0db9-43dd-9c02-f8dc7b07ee7d
// wss://vipspeedtest4.wuhan.net.cn:8080/ws
// wss://vipspeedtest1.wuhan.net.cn:8080/ws
// wss://112.122.10.26.prod.hosts.ooklaserver.net:8080/ws
// wss://5g.shunicomtest.com.prod.hosts.ooklaserver.net:8080/ws
// wss://speedtest.jnltwy.com.prod.hosts.ooklaserver.net:8080/ws
// wss://kr12.host.speedtest.net.prod.hosts.ooklaserver.net:8080/ws
// wss://speedtest1.jlinfo.jl.cn.prod.hosts.ooklaserver.net:8080/ws
// wss://221.199.9.35.prod.hosts.ooklaserver.net:8080/ws
// wss://speedtest.kdatacenter.com.prod.hosts.ooklaserver.net:8080/ws
// wss://speedtest.utahbroadband.com:8080/ws
