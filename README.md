## Download 3DS transaction from Arcot admin site

### TL;DR

Make sure Chrome browser is installed

```
# download last 30 minutes transaction from preview site
./alltran

# download last 30 minutes transaction from production site
./alltran -prod -u userid -p passoword

# display all command line options
./alltran -h
```

### Build
This program is written using [Go programming language](http://golang.org). It uses the new [chromedp](https://github.com/chromedp/chromedp) driver. No JRE, Selenium or chrome driver required.

```
go get -u github.com/chromedp/chromedp
go get -u github.com/fsnotify/fsnotify
go get -u golang.org/x/sys/...

# build binary for your OS
go build

# for Windows binary, set 2 environment variables, bash example below
GOOS=windows GOBARCH=amd64 go build

```

### Known issues
1. Chrome will prompt whether to cancel the download or continue even after the complete CSV file has been downloaded. Select Cancel should be safe and Chrome browser will close automatically.
