## FTP测速软件Android端的开发环境和编译方法

- FTP测速软件(程序名叫`ftp-perf-arm64`)Android端是个普通的`ELF`程序
- 不是我们通常意义上的`Android APP`
- 所以`ftp-perf-arm64`编译方法和我们通常的`APK`很大不同，它没有`APK`

### 安装Windows上的开发环境

1. 安装`Android SDK`

   1. 程序本身对`Android SDK`版本没有特殊要求，但目前没有使用过**28**以外的版本，为了避免麻烦建议也使用这个版本的`SDK`
   2. 设置环境变量 `ANDROID_SDK_ROOT`=SDK Installed DIR

2. 安装`Android NDK`

   1. 目前程序使用`NDK`版本为**20**
   2. 设置环境变量 NDK=NDK Installed DIR
   3. 设置环境变量 ANDROID_NDK_ROOT=%NDK%

3. 安装`Python` (只用过`Anaconda`，为了避免麻烦，建议安装`Anaconda`)

   1. 应该是开箱即用

4. 制作`Stanndalone Toolchain`

   ```bat
   python %NDK%/build/tools/make_standalone_toolchain.py ^
   	--arch arm64 ^
   	--api 28 ^
   	--install-dir c:/arm64-bc
   ```

   2. 设置环境变量 ANDROID_TOOLCHAIN=c:/arm64-bc

5. 安装`golang`

   1. 下载安装

   2. 设置环境变量GOROOT=Your Install DIR

   3. 设置环境变量GOPATH=Your Install DIR ，我们的程序`outside of GOPATH`，单有些依赖包安装到GOPATH会提高编译速度

   4. 更新依赖包，可能有些安装包需要翻墙才能安装

      ```bash
      set HTTPS_PROXY=socks5://xxx.xx.xx:xxxx
      set HTTP_PROXY=socks5://xxx.xx.xx.xxx:xxxx
      go  get -u -v golang.org/x/toools/...
      go get -u -v github.com/secsy/goftp/...
      go get -u -v github.com/paulbellamy/ratecounter
      go get -u -v github.com/c2h5oh/datasize
      go get -u -v github.com/fclairamb/ftpserver
      ```

      

6. 编译程序

   ```
   set TARGET_HOST=aarch64-linux-android
   
   set PATH=%PATH%;%ANDROID_TOOLCHAIN%/bin
   set GOARCH=arm64
   set GOOS=android
   set CGO_ENABLED=1
   set CC=%TARGET_HOST%-gcc
   set CXX=%TARGET_HOST%-g++
   
   where %CC%
   
   :: 必须进入到ftp-perf-arm64目录，第一次编译可能比较慢
   cd c:/repo/cds8-svn/ftp-perf-arm64 
   
   go build -o ftp-perf-arm64
   
   file ftp-perf-arm64
   
   :: ftp-perf-arm64; ELF 64-bit LSB shared object, version 1 (SYSV), dynamically linked (uses shared libs), not stripped
   ```

7. 打包放到手机中

   ```bash
   tar -cf ftp-perf-arm64.tar ftp-perf-arm64 assets
   adb push ftp-perf-arm64.tar /sdcard
   adb shell "su -c pkill ftp-perf-arm64"
   adb shell "su -c tar xvf /sdcard/ftp-perf-arm64.tar -C /data/data/"
   adb shell "su -c chmod a+x /data/data/ftp-perf-arm64"
   ```

   

8. 验证安装是否正确

   ```bash
   adb shell "su -c /data/data/ftp-perf-arm64"
   
   c:\repo\cds8-svn\bin>adb shell "su -c /data/data/ftp-perf-arm64 --file=1K.rar"
   goftp: 0.000 #1 opening control connection to 218.203.61.198:21
   goftp: 0.146 #1 sending command user heilongjiangdl
   goftp: 0.188 #1 got 331-password required for heilongjiangdl
   goftp: 0.189 #1 sending command pass ******
   goftp: 0.371 #1 got 230-last login was: 2019-05-31 13:35:18
   user heilongjiangdl logged in
   goftp: 0.371 #1 sending command feat
   goftp: 0.416 #1 got 211-features:
    mdtm
    mfmt
    tvfs
    lang it-it;bg-bg;ru-ru;zh-cn;es-es;ko-kr;en-us;ja-jp;fr-fr;zh-tw
    mff modify;unix.group;unix.mode;
    mlst modify*;perm*;size*;type*;unique*;unix.group*;unix.mode*;unix.owner*;
    rest stream
    size
   end
   goftp: 0.418 #1 sending command type i
   goftp: 0.458 #1 got 200-type set to i
   goftp: 0.461 #1 sending command size 1k.rar
   goftp: 0.501 #1 got 550-1k.rar: no such file or directory
   goftp: 0.505 #1 unexpected size response: 550 (1k.rar: no such file or directory)
   goftp: 0.508 #1 was ready
   goftp: 0.511 #1 was ready
   goftp: 0.514 #1 sending command type i
   goftp: 0.558 #1 got 200-type set to i
   goftp: 0.558 #1 sending command epsv
   goftp: 0.599 #1 got 229-entering extended passive mode (|||37749|)
   goftp: 0.602 #1 opening data connection to [218.203.61.198]:37749
   goftp: 0.651 #1 sending command retr 1k.rar
   goftp: 0.695 #1 got 550-1k.rar: no such file or directory
   goftp: 0.695 #1 closing
   goftp: ends
   ```

   

9. 验证`PC DEMO`是否工作

   1. `PC DEMO`是一个在`Windows`端启动手机上的`ftp-perf-arm64`程序的，有这样一个界面。如果程序能够正常运行说明，环境和整个程序运行正确

      ![1559281145260](C:\Users\hzane\AppData\Roaming\Typora\typora-user-images\1559281145260.png)

10. 环境准备好，开始开发`CDS`的`ATE`…

