package main

// Download all 3DS transactions from CA Arcot site

import (
	"flag"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/runner"
	"log"
	"os"
	"runtime"
	"time"
)

const MaxDownloadAttempts = 3

type Env struct {
	url, username, password, period string
	proxy, out                      string
	loginDelay, downloadWaitTime    int
	headless, prod, debug           bool
}

var transactionFile string

func cwd() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	return dir
}

func parseParameters() Env {
	env := Env{}

	flag.BoolVar(&env.prod, "prod", false, "production mode")
	flag.BoolVar(&env.debug, "v", false, "print debug logs")
	flag.BoolVar(&env.headless, "headless", false, "use Chrome headless mode")
	flag.IntVar(&env.loginDelay, "w", 5, "seconds to wait after login")
	flag.StringVar(&env.username, "u", "scb3ds_global2", "username")
	flag.StringVar(&env.password, "p", "yahoo1234!", "password")
	flag.StringVar(&env.period, "period", "60", "")
	flag.StringVar(&env.proxy, "proxy", "", "proxy server")
	flag.StringVar(&env.out, "o", "", "convert CSV input to JSON format and write to this file.")

	flag.Parse()

	if env.prod {
		env.url = "https://secure5.arcot.com/vpas/admin/"
	} else {
		env.url = "https://preview5.arcot.com/vpas/admin/"
	}

	if env.debug {
		log.Printf("env = %+v", env)
	}

	return env
}

func main() {
	env := parseParameters()

	if env.headless && runtime.GOOS == "windows" {
		env.headless = false
		log.Println("Headless mode on Windows is not supported")
		// with Chrome 64.0.3282.119  on Windows 10, launching Chrome in headless mode
		// will be left running after this program exits. we disable the support for now
	}

	options := []runner.CommandLineOption{
		runner.Flag("headless", env.headless),
		runner.Flag("disable-gpu", env.headless),
	}

	if env.proxy != "" {
		options = append(options, runner.ProxyServer("http://"+env.proxy))
	}

	if env.debug {
		log.Println("Options:", options)
	}

	chromeOptions := chromedp.WithRunnerOptions(options...)
	for i := 0; i < MaxDownloadAttempts; i++ {
		runWithChrome(fetchTransactionList(env), chromeOptions, env.debug)

		if transactionFile != "" {
			log.Println("Downloaded ", transactionFile)
			break
		}

		if i < MaxDownloadAttempts-1 {
			log.Println("No file downloaded, retrying...")
			time.Sleep(10 * time.Second)
		}
	}

	if transactionFile != "" && env.out != "" {
		err := writeJson(transactionFile, env.out)
		if err != nil {
			log.Fatal(err)
		}
	}
}
