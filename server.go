package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/fclairamb/ftpserver/server"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

func mainSRV(addr string) {
	var ftpServer *server.FtpServer
	// Setting up the logger
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))

	// Loading the driver
	driver, err := NewSampleDriver(addr)

	if err != nil {
		_ = level.Error(logger).Log("msg", "Could not load the driver", "err", err)
		return
	}

	// Overriding the driver default silent logger by a sub-logger (component: driver)
	driver.Logger = log.With(logger, "component", "driver")

	// Instantiating the server by passing our driver implementation
	ftpServer = server.NewFtpServer(driver)

	// Overriding the server default silent logger by a sub-logger (component: server)
	ftpServer.Logger = log.With(logger, "component", "server")

	// Preparing the SIGTERM handling
	go signalHandler(ftpServer)

	if err := ftpServer.ListenAndServe(); err != nil {
		_ = level.Error(logger).Log("msg", "Problem listening", "err", err)
	}
}

func signalHandler(ftpServer *server.FtpServer) {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGTERM)
	for {
		switch <-ch {
		case syscall.SIGTERM:
			ftpServer.Stop()
			break
		}
	}
}
