
@echo off

tar -cf ftp-perf-arm64.tar ftp-perf-arm64 assets

if %ERRORLEVEL% EQU 0 (
    adb push ftp-perf-arm64.tar /sdcard/
)

if %ERRORLEVEL% EQU 0 (
    adb shell 'su -c pkill ftp-perf-arm64'
    type nul>nul
)

if %ERRORLEVEL% EQU 0 (
    adb shell "tar xvf /sdcard/ftp-perf-arm64.tar -C /data/local/tmp/"
    type nul>nul
    rem adb shell "su -c tar xvf /sdcard/ftp-perf-arm64.tar -C /data/data/"
)


if %ERRORLEVEL% EQU 0 (
    rem adb shell "su -c chmod a+x /data/data/ftp-perf-arm64"
    adb shell "chmod a+x /data/local/tmp/ftp-perf-arm64"
    type nul>nul
)

if %ERRORLEVEL% EQU 0 (
    adb shell "ls -l /data/local/tmp/ftp-perf-arm64"
)

if %ERRORLEVEL% EQU 0 (
    rem adb shell "su -c 'nohup /data/local/tmp/ftp-perf-arm64 --http-addr=:18103 &'"
)

if %ERRORLEVEL% EQU 0 (
    echo "ftp-perf-arm64 is ready"
) else (
    echo "something wrong, please run this script line by line."
)
