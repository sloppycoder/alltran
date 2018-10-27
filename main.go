package main

// Download all 3DS transactions from CA Arcot site

import (
	"flag"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/runner"
	_ "github.com/joho/godotenv/autoload"
	"log"
	"os"
	"runtime"
	"time"
)

const MaxDownloadAttempts = 3

type Env struct {
	url, username, password string
	period                  string
	proxy                   string
	headless, debug, trace  bool
}

var influxdbUrl string

func cwd() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	return dir
}

func getHttpProxyFromEnv() string {
	proxy := os.Getenv("HTTP_PROXY")
	if proxy == "" {
		proxy = os.Getenv("http_proxy")
	}
	return proxy
}

func parseParameters() Env {
	env := Env{}
	env.url = os.Getenv("ARCOT_URL")
	env.username = os.Getenv("ARCOT_USERNAME")
	env.password = os.Getenv("ARCOT_PASSWORD")
	influxdbUrl = os.Getenv("INFLUXDB_URL")

	flag.BoolVar(&env.debug, "v", false, "print debug logs")
	flag.BoolVar(&env.trace, "vv", false, "print trace logs")
	flag.BoolVar(&env.headless, "headless", false, "use Chrome headless mode")
	flag.StringVar(&env.period, "period", "60", "")
	flag.StringVar(&env.proxy, "proxy", getHttpProxyFromEnv(), "proxy server")
	flag.StringVar(&influxdbUrl, "influxdb", influxdbUrl, "InfluxDB URL to send transaction records to. Empty value disables the upload.")
	flag.StringVar(&transactionFile, "csv", "", "CSV file to process. Specify file here will bypass the download logic")
	flag.Parse()

	if env.trace {
		env.debug = true
	}

	if env.debug {
		denv := env
		denv.password = "****"
		log.Printf("env = %+v", denv)
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

	if transactionFile == "" {
		chromeOptions := chromedp.WithRunnerOptions(options...)
		for i := 0; i < MaxDownloadAttempts; i++ {
			runWithChrome(fetchTransactionList(env), chromeOptions, env.trace)

			if transactionFile != "" {
				log.Println("Downloaded ", transactionFile)
				break
			}

			if i < MaxDownloadAttempts-1 {
				log.Println("No file downloaded, retrying...")
				time.Sleep(10 * time.Second)
			}
		}
	}

	if transactionFile != "" && influxdbUrl != "" {
		err := csvToInfluxDB(transactionFile, influxdbUrl, "tds")
		if err != nil {
			log.Fatal(err)
		}
	}
}
