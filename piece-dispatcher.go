package main

import (
    "io"
    "sync"

    "github.com/secsy/goftp"
)

// PieceDispatcher ...
type PieceDispatcher struct {
    lock       sync.Mutex
    dummy      *FTPData
    file       string
    fileLength uint64
    parallels  int
    start      uint64
}

func (pd *PieceDispatcher) slice() (begin, sz uint64) {
    pd.lock.Lock()
    defer pd.lock.Unlock()

    // 如果定时器到时，停止后续的下载
    if pd.dummy.g.isTimeUp() {
        begin, sz = pd.fileLength, 0
        return
    }
    sz = pd.fileLength - pd.start
    // 如果不需要并发，也就不需要分片下载
    if sz > 128<<10 && pd.parallels > 1 {
        sz = sz / 2 / uint64(pd.parallels)
    }
    begin = pd.start
    pd.start = begin + sz
    return
}

func transferWorker(client *goftp.Client, pd *PieceDispatcher, method string) {
    var dest io.Writer
    var src io.Reader
    var transfer = client.TransferRangeDownload
    if method == "upload" {
        src = pd.dummy
        transfer = client.TransferRangeUpload
    }
    if method == "download" {
        dest = pd.dummy
        transfer = client.TransferRangeDownload
    }
    for begin, sz := pd.slice(); sz > 0; {
        n, err := transfer(pd.file, dest, src, int64(begin), int64(sz))
        begin, sz = pd.slice()
        _, _ = n, err
    }
}

func pieceDispatcher(file string, len uint64, parallels int, dummy *FTPData) *PieceDispatcher {
    ret := &PieceDispatcher{
        parallels:  parallels,
        fileLength: len,
        start:      0,
        file:       file,
        dummy:      dummy,
    }
    return ret
}

