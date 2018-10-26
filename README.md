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

[Dep](https://github.com/golang/dep) is required.

```

dep ensure

# build binary for your OS
go build

# for Windows binary, set 2 environment variables, bash example below
GOOS=windows GOBARCH=amd64 go build

```

## Don't have a Linux machine to test?
Spin a CentOS 7 VM with Chrome by running ```vagrant up```
