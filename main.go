// Download all 3DS transactions from CA Arcot site
package main

import (
	"context"
	"errors"
	"flag"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/fsnotify/fsnotify"
	"github.com/robfig/cron"
	"log"
	"os"
	"strings"
	"time"
)

const DownloadSuffix = "crdownload"

type Env struct {
	url, username, password, period, cron string
	loginDelay, downloadWaitTime          int
	prod, debug                           bool
}

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
	flag.IntVar(&env.loginDelay, "w", 5, "seconds to wait after login")
	flag.IntVar(&env.downloadWaitTime, "d", 120, "seconds to wait for downloading a file")
	flag.StringVar(&env.username, "u", "scb3ds_global2", "username")
	flag.StringVar(&env.password, "p", "yahoo1234!", "password")
	flag.StringVar(&env.period, "period", "30", "")
	flag.StringVar(&env.cron, "cron", "", "cron expression")
	flag.BoolVar(&env.debug, "v", false, "print debug logs")

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

func runWithChrome(taskFunc func(context.Context, *chromedp.CDP) error, debug bool) {
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

	err = taskFunc(ctx, c)
	if err != nil {
		// don't use Fatal because we need to deter functions to run
		log.Printf("%v", err)
		return
	}
}

func fetchFromArcot(env Env) func(context.Context, *chromedp.CDP) error {
	// drop down to select 30, 60, 90 minutes for download
	// in Arcot's site, test and production uses different name for the same control
	var periodDropdown string
	if env.prod {
		periodDropdown = `//select[@name="reportForm:j_id_2o"]`
	} else {
		periodDropdown = `//select[@name="reportForm:j_id_2n"]`
	}

	return func(ctx context.Context, c *chromedp.CDP) error {
		var html string
		return c.Run(ctx, chromedp.Tasks{
			page.SetDownloadBehavior(page.SetDownloadBehaviorBehaviorAllow).WithDownloadPath(cwd()),
			// login
			chromedp.Navigate(env.url + "index.jsp?bank=i18n/en_US&locale=en_US"),
			chromedp.WaitVisible(`//input[@name="adminname"]`),
			chromedp.SendKeys(`//input[@name="adminname"]`, env.username),
			chromedp.SendKeys(`//input[@name="password"]`, env.password),
			chromedp.Submit(`//input[@name="Submit"]`),
			chromedp.Sleep(time.Duration(env.loginDelay) * time.Second),
			// detect if a frameset is displayed in the browser
			chromedp.EvaluateAsDevTools("document.getElementsByName('topFrame')[0].contentWindow.document.body.outerHTML;", &html),
			chromedp.ActionFunc(func(ctxt context.Context, h cdp.Executor) error {
				// when login fails, eval above javascript will cause an "Uncaught exception"
				// if the code reaches here login must have succeeded
				log.Printf("Login success")
				return nil
			}),
			// launch the All Transaction download page in full page mode
			chromedp.Navigate(env.url + "report/ReportByIssuerAndDate.jsf?reportId=AllTransactions&bank=i18n/en_US&locale=en_US&loggedinlevel=2&auth=1"),
			chromedp.WaitVisible(`//input[@name="reportForm:btnExport"]`),
			// enter download period and click submit button
			chromedp.SetValue(periodDropdown, env.period),
			chromedp.Click(`//input[@name="reportForm:btnExport"]`, chromedp.NodeVisible),
			chromedp.ActionFunc(func(ctxt context.Context, h cdp.Executor) error {
				log.Printf("Download submitted")
				return nil
			}),
			chromedp.Sleep(1 * time.Second),
			// check if "no matching records found" is displayed on the screen
			chromedp.Evaluate(`document.getElementsByClassName("reportFilterHeadingTable")[0].outerHTML`, &html),
			chromedp.ActionFunc(func(ctxt context.Context, h cdp.Executor) error {
				if strings.Contains(html, "No matching records found") {
					println("no data available")
					return nil
				}

				newFile, err := waitForDownload(time.Duration(env.downloadWaitTime)*time.Second, env.debug)
				if err != nil {
					return err
				}
				log.Printf("Downloaded file %s", newFile)
				return nil
			}),
			// sleep a bit for the Chrome to realize download has finished
			// otherwise it'll prompt Cancel download or continue when we try
			// to close the browser
			chromedp.Sleep(1 * time.Second),
			// logout
			chromedp.Navigate(env.url + "adminlogout.jsp?bank=i18n/en_US&locale=en_US&loggedinlevel=2&auth=1"),
			chromedp.ActionFunc(func(ctxt context.Context, h cdp.Executor) error {
				log.Printf("Logout")
				return nil
			}),
		})
	}
	return nil
}

func waitForDownload(duration time.Duration, debug bool) (string, error) {
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

	log.Printf("Waiting for file download")
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
				if strings.HasSuffix(event.Name, DownloadSuffix) {
					file := strings.TrimSuffix(event.Name, DownloadSuffix)
					return file, nil
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

func main() {
	env := parseParameters()
	if env.cron == "" {
		runWithChrome(fetchFromArcot(env), env.debug)
	} else {
		c := cron.New()
		c.AddFunc(env.cron, func() {
			runWithChrome(fetchFromArcot(env), env.debug)
		})
		c.Start()
	}
}
