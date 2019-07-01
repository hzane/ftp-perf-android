::#!/usr/bin/env bash
@echo off

IF NOT DEFINED ANDROID_TOOLCHAIN  set ANDROID_TOOLCHAIN=c:/arm64-bc

set TARGET_HOST=aarch64-linux-android

set GOARCH=arm64
set GOOS=android
set CGO_ENABLED=1
set CC=%TARGET_HOST%-gcc
set CXX=%TARGET_HOST%-g++

where /q %CC%
if %errorlevel% ==1 (
    set PATH=%PATH%;%ANDROID_TOOLCHAIN%/bin
)

where /q %CC%
if %errorlevel% ==1 (
    echo "cannot find gcc. may ANDROID_TOOLCHAIN be wrong"
)
@echo on
go build -o ftp-perf-arm64
