#!/usr/bin/env bash

tar -cf ftp-perf-arm64.tar ftp-perf-arm64 assets
retval=$?

if [ $retval -eq 0 ]; then
    adb push ftp-perf-arm64.tar /sdcard
    retval=$?
fi

if [ $retval -eq 0 ]; then
    adb shell 'su -c pkill ftp-perf-arm64'
    retval=$?
fi


if [ $retval -eq 0 ]; then
    adb shell 'su -c tar xvf /sdcard/ftp-perf-arm64.tar -C /data/data/'
    retval=$?
fi

if [ $retval -eq 0 ]; then
    adb shell 'su -c chmod a+x /data/data/ftp-perf-arm64'
    retval=$?
fi

if [ $retval -eq 0 ]; then
    adb shell 'su -c ls -l /data/data/ftp-perf-arm64'
    retval=$?
fi


if [ $retval -eq 0 ]; then
    adb shell "su -c \"nohup /data/data/ftp-perf-arm64 --http-addr=:18103 &\""
    retval=$?
fi

if [ $retval -eq 0 ]; then
    adb forward tcp:18103 tcp:18103
    retval=$?
fi

if [ $retval -eq 0 ]; then
    echo 'open http://localhost:18103/static/gauge.html to check everything is ok'
else
    echo 'something wrong, please run this script line by line.'
fi
