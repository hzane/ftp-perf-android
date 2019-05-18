// Package sample is a sample server driver
package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/fclairamb/ftpserver/server"
	"github.com/go-kit/kit/log"
)

// MainDriver defines a very basic ftpserver driver
type MainDriver struct {
	Logger    log.Logger  // Logger
	BaseDir   string      // Base directory from which to serve file
	config    OurSettings // Our settings
	nbClients int32       // Number of clients
}

// ClientDriver defines a very basic client driver
type ClientDriver struct {
	BaseDir string // Base directory from which to server file
}

// Account defines a user/pass password
type Account struct {
	User string // Username
	Pass string // Password
	Dir  string // Directory
}

// OurSettings defines our settings
type OurSettings struct {
	Server         server.Settings // Server settings (shouldn't need to be filled)
	Users          []Account       // Credentials
	MaxConnections int32           // Maximum number of clients that are allowed to connect at the same time
}

// GetSettings returns some general settings around the server setup
func (driver *MainDriver) GetSettings() (*server.Settings, error) {
	driver.config.MaxConnections = 10
	driver.config.Users = append(driver.config.Users, Account{User: "hzane", Pass: "Refresh", Dir: "."})
	driver.config.Server.IdleTimeout = 900

	if len(driver.config.Users) == 0 {
		return nil, errors.New("you must have at least one user defined")
	}

	return &driver.config.Server, nil
}

// GetTLSConfig returns a TLS Certificate to use
func (driver *MainDriver) GetTLSConfig() (*tls.Config, error) {
	return nil, nil
}

// Live generation of a self-signed certificate
// This implementation of the driver doesn't load a certificate from a file on purpose. But it any proper implementation
// should most probably load the certificate from a file using tls.LoadX509KeyPair("cert_pub.pem", "cert_priv.pem").
func (driver *MainDriver) getCertificate() (*tls.Certificate, error) {
	return nil, nil
}

// WelcomeUser is called to send the very first welcome message
func (driver *MainDriver) WelcomeUser(cc server.ClientContext) (string, error) {
	nbClients := atomic.AddInt32(&driver.nbClients, 1)
	if nbClients > driver.config.MaxConnections {
		return "Cannot accept any additional client", fmt.Errorf("too many clients: %d > % d", driver.nbClients, driver.config.MaxConnections)
	}

	cc.SetDebug(true)
	// This will remain the official name for now
	return fmt.Sprintf(
		"Welcome on ftpserver, you're on dir %s, your ID is %d, your IP:port is %s, we currently have %d clients connected",
		driver.BaseDir,
		cc.ID(),
		cc.RemoteAddr(),
		nbClients),
		nil
}

// AuthUser authenticates the user and selects an handling driver
func (driver *MainDriver) AuthUser(cc server.ClientContext, user, pass string) (server.ClientHandlingDriver, error) {
	return &ClientDriver{BaseDir: "."}, nil
}

// UserLeft is called when the user disconnects, even if he never authenticated
func (driver *MainDriver) UserLeft(cc server.ClientContext) {
	atomic.AddInt32(&driver.nbClients, -1)
}

// ChangeDirectory changes the current working directory
func (driver *ClientDriver) ChangeDirectory(cc server.ClientContext, directory string) error {
	directory = path.Clean(directory)
	if directory == "/" || directory == "" {
		return nil
	}
	return fmt.Errorf("%s not found", directory)
}

// MakeDirectory creates a directory
func (driver *ClientDriver) MakeDirectory(cc server.ClientContext, directory string) error {
	return nil
}

// ListFiles lists the files of a directory
func (driver *ClientDriver) ListFiles(cc server.ClientContext) ([]os.FileInfo, error) {
	files := make([]os.FileInfo, 0)
	files = append(files,
		virtualFileInfo{
			name: "1B.zero",
			mode: os.FileMode(0666),
			size: 1,
		},
		virtualFileInfo{
			name: "1K.zero",
			mode: os.FileMode(0666),
			size: 100 << 20,
		},
		virtualFileInfo{
			name: "10K.zero",
			mode: os.FileMode(0666),
			size: 100 << 20,
		},
		virtualFileInfo{
			name: "100K.zero",
			mode: os.FileMode(0666),
			size: 100 << 20,
		},
		virtualFileInfo{
			name: "1M.zero",
			mode: os.FileMode(0666),
			size: 100 << 20,
		},
		virtualFileInfo{
			name: "10M.zero",
			mode: os.FileMode(0666),
			size: 100 << 20,
		},
		virtualFileInfo{
			name: "100M.zero",
			mode: os.FileMode(0666),
			size: 100 << 20,
		},
		virtualFileInfo{
			name: "500M.zero",
			mode: os.FileMode(0666),
			size: 500 << 20,
		},
		virtualFileInfo{
			name: "1G.zero",
			mode: os.FileMode(0666),
			size: 1 << 30,
		},
		virtualFileInfo{
			name: "2G.zero",
			mode: os.FileMode(0666),
			size: 2 << 30,
		},
	)
	return files, nil
}

// OpenFile opens a file in 3 possible modes: read, write, appending write (use appropriate flags)
func (driver *ClientDriver) OpenFile(cc server.ClientContext, path string, flag int) (server.FileStream, error) {
	return &virtualFile{size: sizeFromName(path)}, nil
}

// GetFileInfo gets some info around a file or a directory
func (driver *ClientDriver) GetFileInfo(cc server.ClientContext, fpath string) (os.FileInfo, error) {
	switch fpath {
	case "/virtual":
		return &virtualFileInfo{name: "virtual", size: 4096, mode: os.ModeDir}, nil
	default:
		return &virtualFileInfo{name: path.Base(fpath), size: int64(sizeFromName(fpath)), mode: 0644}, nil
	}
}

// CanAllocate gives the approval to allocate some data
func (driver *ClientDriver) CanAllocate(cc server.ClientContext, size int) (bool, error) {
	return true, nil
}

// ChmodFile changes the attributes of the file
func (driver *ClientDriver) ChmodFile(cc server.ClientContext, path string, mode os.FileMode) error {
	return fmt.Errorf("not supported")
}

// DeleteFile deletes a file or a directory
func (driver *ClientDriver) DeleteFile(cc server.ClientContext, path string) error {
	return fmt.Errorf("not supported")
}

// RenameFile renames a file or a directory
func (driver *ClientDriver) RenameFile(cc server.ClientContext, from, to string) error {
	return fmt.Errorf("not supported")
}

// NewSampleDriver creates a sample driver
func NewSampleDriver(addr string) (*MainDriver, error) {
	drv := &MainDriver{
		Logger:  log.NewNopLogger(),
		BaseDir: ".",
	}
	drv.config.Server.ListenAddr = addr
	return drv, nil
}

// 100MB mb kb KB GB  k m g
func sizeFromName(s string) int64 {
	s = strings.TrimSuffix(path.Base(s), path.Ext(s))

	var v datasize.ByteSize
	if err := v.UnmarshalText([]byte(s)); err == nil {
		return int64(v.Bytes())
	}
	return 0
}

// The virtual file is an example of how you can implement a purely virtual file
type virtualFile struct {
	offset int64 // Reading offset
	size   int64
}

func (f *virtualFile) Close() error {
	return nil
}

func (f *virtualFile) Read(buffer []byte) (n int, err error) {
	n, left := len(buffer), f.size-f.offset
	if left < 0 {
		left = 0
	}
	if int64(n) > left {
		n = int(left)
	}
	f.offset += int64(n)
	if n == 0 {
		err = io.EOF
	}
	return
}

func (f *virtualFile) Seek(n int64, w int) (int64, error) {
	switch w {
	case 0:
		f.offset = n
	case 1:
		f.offset += n
	case 2:
		f.offset = f.size + n
	}
	return 0, nil
}

func (f *virtualFile) Write(buffer []byte) (int, error) {
	f.offset += int64(len(buffer))
	return len(buffer), nil
}

type virtualFileInfo struct {
	name string
	size int64
	mode os.FileMode
}

func (f virtualFileInfo) Name() string {
	return f.name
}

func (f virtualFileInfo) Size() int64 {
	return f.size
}

func (f virtualFileInfo) Mode() os.FileMode {
	return f.mode
}

func (f virtualFileInfo) IsDir() bool {
	return f.mode.IsDir()
}

func (f virtualFileInfo) ModTime() time.Time {
	return time.Now().UTC()
}

func (f virtualFileInfo) Sys() interface{} {
	return nil
}
