// Download all 3DS transactions from CA Arcot site
package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/fsnotify/fsnotify"
	"log"
	"os"
	"strings"
	"time"
)

// wait before considering a download has failed
const WaitDuration = 120 * time.Second
const DownloadSuffix = "crdownload"

type Env struct {
	url, username, password, period string
	loginDelay                      int
	debug                           bool
}

func parseParameters() Env {
	env := Env{}

	prod := flag.Bool("prod", false, "production mode")
	flag.IntVar(&env.loginDelay, "w", 0, "seconds to wait after login")
	flag.StringVar(&env.username, "u", "scb3ds_global2", "username")
	flag.StringVar(&env.password, "p", "yahoo1234!", "password")
	flag.StringVar(&env.period, "period", "30", "")
	flag.BoolVar(&env.debug, "debug", false, "print debug logs")

	flag.Parse()

	var defaultDelay int
	if *prod {
		env.url, defaultDelay = "https://secure5.arcot.com/vpas/admin/", 15
	} else {
		env.url, defaultDelay = "https://preview5.arcot.com/vpas/admin/", 5
	}

	if env.loginDelay == 0 {
		env.loginDelay = defaultDelay
	}

	if env.debug {
		log.Printf("env = %+v", env)
	}

	return env
}

func runWithChrome(taskFunc func(context.Context, *chromedp.CDP) bool, debug bool) {
	var err error

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create chrome instance
	var c *chromedp.CDP
	if debug {
		c, err = chromedp.New(ctx, chromedp.WithLog(log.Printf))
	} else {
		c, err = chromedp.New(ctx)
	}
	if err != nil {
		log.Fatal(err)
	}

	// make sure Chrome is closed when exit
	defer func() {
		log.Printf("Shutdown chrome")
		c.Shutdown(ctx)
		c.Wait()
	}()

	if taskFunc(ctx, c) {
		newFile, err := waitForNewFile(WaitDuration, true)
		if err == nil {
			log.Printf("Got new file %s", newFile)
		} else {
			log.Printf("%v", err)
		}
	}
}

func fetchFromArcot(env Env) func(context.Context, *chromedp.CDP) bool {
	return func(ctx context.Context, c *chromedp.CDP) bool {
		var html string
		c.Run(ctx, chromedp.Tasks{
			page.SetDownloadBehavior(page.SetDownloadBehaviorBehaviorAllow).WithDownloadPath(cwd()),
			chromedp.Navigate(env.url + "index.jsp?bank=i18n/en_US&locale=en_US"),
			chromedp.WaitVisible(`//input[@name="adminname"]`),
			chromedp.SendKeys(`//input[@name="adminname"]`, env.username),
			chromedp.SendKeys(`//input[@name="password"]`, env.password),
			chromedp.Submit(`//input[@name="Submit"]`),
			chromedp.Sleep(time.Duration(env.loginDelay) * time.Second),
			chromedp.EvaluateAsDevTools("document.getElementsByName('topFrame')[0].contentWindow.document.body.outerHTML;", &html),
		})

		if !strings.Contains(html, "adminlogout") {
			log.Printf("Login failed")
			return false
		}

		log.Printf("Login success")
		c.Run(ctx, chromedp.Tasks{
			chromedp.Navigate(env.url + "report/ReportByIssuerAndDate.jsf?reportId=AllTransactions&bank=i18n/en_US&locale=en_US&loggedinlevel=2&auth=1"),
			chromedp.WaitVisible(`//input[@name="reportForm:btnExport"]`),
			chromedp.SetValue(`//select[@name="reportForm:j_id_2o"]`, env.period),
			chromedp.Click(`//input[@name="reportForm:btnExport"]`, chromedp.NodeVisible),
			chromedp.ActionFunc(func(ctxt context.Context, h cdp.Executor) error {
				log.Printf("Download submitted")
				return nil
			}),
		})

		return true
	}

}

func cwd() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	return dir
}
func waitForNewFile(duration time.Duration, debug bool) (string, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("%v", err)
		return "", err
	}
	defer watcher.Close()

	timer := time.NewTimer(duration)
	defer timer.Stop()

	err = watcher.Add(cwd())
	if err != nil {
		log.Printf("%v", err)
		return "", err
	}

	started := false
	for {
		select {
		case event := <-watcher.Events:
			if debug {
				log.Println("fsnotify:", event)
			}

			// Chrome starts a download with a temporarily file name
			// and rename the file when download is complete
			if event.Op&fsnotify.Rename == fsnotify.Rename {
				if file:= event.Name; strings.HasSuffix(file, DownloadSuffix) {
					log.Printf("Download %s completed", file)
					if testCSV(strings.TrimSuffix(file, DownloadSuffix)) {
						return file, nil
					} else {
						return file, errors.New("file is not a valid CSV file")
					}
				}
			} else if event.Op&fsnotify.Write == fsnotify.Write {
				if strings.HasSuffix(event.Name, DownloadSuffix) {
					timer.Reset(duration)
					started = true
				}
			}
		case err := <-watcher.Errors:
			log.Println("error:", err)
		case <-timer.C:
			if started {
				return "", errors.New("downloaded stalled")
			} else {
				return "", errors.New("no file downloaded")
			}
		}
	}
}

func testCSV(fileName string) bool {
	f, err := os.Open(fileName)
	if err != nil {
		log.Printf("%v", err)
		return false
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err == nil {
		log.Printf("%s has %d records", fileName, len(records))
		return true
	} else {
		log.Printf("%v", err)
		return false
	}
}

func main() {
	env := parseParameters()
	runWithChrome(fetchFromArcot(env), env.debug)
}