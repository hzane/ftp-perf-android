set GOARCH=arm64
set GOOS=linux
go build -o ftp-perf-arm64
adb push ftp-perf-arm64 /data/data
