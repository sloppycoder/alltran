## Download 3DS transaction from Arcot admin site

### TL;DR

Make sure Chrome browser is installed.
Ceate a .env file to store URL, username and password

```
ARCOT_URL=https://secure5.arcot.com/vpas/admin/
ARCOT_URL=https://preview5.arcot.com/vpas/admin/
ARCOT_USERNAME=scb3ds_global2
ARCOT_PASSWORD=yahoo1234!
```

Then

```

# download last 30 minutes transaction from preview site
./alltran

# display all command line options
./alltran -h
```

### Build
This program is written using [Go programming language](http://golang.org). It uses the new [chromedp](https://github.com/chromedp/chromedp) driver. No JRE, Selenium or chrome driver required.

```

go get

# build binary for your OS
go build

# for Windows binary, set 2 environment variables, bash example below
GOOS=windows GOBARCH=amd64 go build

```

## Don't have a Linux machine to test?
Spin a CentOS 7 VM with Chrome by running ```vagrant up```
