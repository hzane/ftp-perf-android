package main

import (
    "net/http"
    "syscall"
)

func shutdown(w http.ResponseWriter, r *http.Request) {
    _ =  syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
}
