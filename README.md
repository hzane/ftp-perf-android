# FTP-Perf-Android

Some codes are copied from `github.com/secsy/goftp`

## 限制

- 暂时不支持主动模式，因为目前没有实现公网IP地址探测，也没有实现TCP穿透，这些在Android上都不好实现
- CDS端采用替换原来的实现方案的方法，而没有另外增加一套界面，这是因为我对界面很不擅长，不容易实现
- 目前支持多并发下载或者上传，这里的并发就是原先的`多线程` 
- FTP下载或者上传均支持断点续传
- 目前的`ping` 实现不能绕过 `selinux` 安全策略，所以为了在`非root`的设备上能够使用`ping`，目前手机端的`ping`使用原生的`ping`替代
- 目前手机端没有`apk`，程序放置在`/data/local/tmp/`中，所以慎用系统清理程序
- `FTP`上传功能并没有上传实际的文件，只是上传了和所选文件大小相同的数据，因为文件从`PC端`传到`手机端`很耗时，也没有地方放，`sdcard`的存取速度也跟不上
- 目前手机端支持单文件多并发上传，实际上FTP协议本身不能支持这种功能，所以实际上传到FTP服务器的数据会出现乱序，但数据大小是正确的

## 安装

- 设备打开adb调试
- ftp-perf-arm64.tar在当前目录



```bash
adb push ftp-perf-arm64.tar /sdcard/
# ignore chown errors
adb shell "tar xvf /sdcard/ftp-perf-arm64.tar -C /data/local/tmp/"
adb shell "chmod a+x /data/local/tmp/ftp-perf-arm64"
adb shell "ls -l /data/local/tmp/ftp-perf-arm64"
echo "DONE"
```

