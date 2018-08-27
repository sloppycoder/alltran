package main

import (
	"context"
	"errors"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/fsnotify/fsnotify"
	"log"
	"runtime"
	"strings"
	"time"
)

const ProgramMaxRuntime = 300 * time.Second

func runWithChrome(taskFunc func(context.Context, *chromedp.CDP) error, chromeOption chromedp.Option, debug bool) {
	var err error

	ctx, cancel := context.WithTimeout(context.Background(), ProgramMaxRuntime)
	defer cancel()

	// create chrome instance
	var c *chromedp.CDP
	if debug {
		c, err = chromedp.New(ctx, chromeOption, chromedp.WithLog(log.Printf))
	} else {
		c, err = chromedp.New(ctx, chromeOption)
	}
	if err != nil {
		log.Fatal(err)
	}

	// make sure Chrome is closed when exit
	defer func() {
		log.Printf("Shutdown chrome")
		// shutdown chrome
		err = c.Shutdown(ctx)
		if err != nil {
			log.Fatal(err)
		}

		// wait for chrome to finish
		// Wait() will hang on Windows and Linux with Chrome headless mode
		// we'll need to exit the program when this happens
		ch := make(chan error)
		go func() {
			c.Wait()
			ch <- nil
		}()

		select {
		case err = <-ch:
			log.Println("chrome closed")
		case <-time.After(10 * time.Second):
			log.Println("chrome didn't shutdown within 10s")
		}
	}()

	err = taskFunc(ctx, c)
	if err != nil {
		// don't use Fatal because we need to deter functions to run
		log.Printf("%v", err)
		return
	}
}

func logout(ctx context.Context, c *chromedp.CDP, env Env) error {
	return c.Run(ctx, chromedp.Tasks{
		chromedp.Navigate(env.url + "adminlogout.jsp?bank=i18n/en_US&locale=en_US&loggedinlevel=2&auth=1"),
		chromedp.ActionFunc(func(ctxt context.Context, h cdp.Executor) error {
			log.Printf("Logged out")
			return nil
		}),
	})
}

func login(ctx context.Context, c *chromedp.CDP, env Env) error {
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
			log.Printf("Logged in")
			return nil
		}),
	})
}

func downloadAllTransactions(ctx context.Context, c *chromedp.CDP, env Env) error {
	var html string

	exportButton := `//input[@name="reportForm:btnExport"]`
	return c.Run(ctx, chromedp.Tasks{
		// launch the All Transaction download page in full page mode
		chromedp.Navigate(env.url + "report/ReportByIssuerAndDate.jsf?reportId=AllTransactions&bank=i18n/en_US&locale=en_US&loggedinlevel=2&auth=1"),
		chromedp.WaitVisible(exportButton),
		// enter download period and click submit button
		chromedp.SetValue(`//select[@name="reportForm:j_id_2n"]`, env.period),
		chromedp.Click(exportButton, chromedp.NodeVisible),
		chromedp.ActionFunc(func(ctxt context.Context, h cdp.Executor) error {
			log.Printf("Select %s minutes and click Export button", env.period)
			return nil
		}),
		//chromedp.WaitVisible(exportButton),
		chromedp.Sleep(2 * time.Second),
		// check if "no matching records found" is displayed on the screen
		chromedp.Evaluate(`document.getElementsByClassName("reportFilterHeadingTable")[0].outerHTML`, &html),
		chromedp.ActionFunc(func(ctxt context.Context, h cdp.Executor) error {
			if strings.Contains(html, "No matching records found") {
				return errors.New("no data available")
			}

			err := waitForDownload(env.debug)
			if err != nil {
				log.Println(err)
			}

			return nil
		}),
		// sleep a bit for the Chrome to realize download has finished
		// otherwise it'll prompt Cancel download or continue when we try
		// to close the browser
		chromedp.Sleep(1 * time.Second),
	})
}

func fetchTransactionList(env Env) func(context.Context, *chromedp.CDP) error {

	return func(ctx context.Context, c *chromedp.CDP) error {
		err := login(ctx, c, env)
		if err != nil {
			return err
		}

		defer logout(ctx, c, env)

		return downloadAllTransactions(ctx, c, env)
	}
}

func waitForDownload(debug bool) error {
	wait := 10 * time.Second
	if runtime.GOOS == "windows" {
		// on Windows. fanotify event fires very infrequently
		wait = 20 * time.Second
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	err = watcher.Add(cwd())
	if err != nil {
		return err
	}

	log.Printf("Waiting for file download")
	for {
		select {
		case event := <-watcher.Events:
			if debug {
				log.Println("fsnotify:", event)
			}

			if strings.Contains(event.Name, "AllTransactions") {
				transactionFile = event.Name
			}
		case err := <-watcher.Errors:
			return err
		case <-time.After(wait):
			// wait for some time. if any event received in last 15s, keep waiting
			// if not, return, assuming nothing more is going to be written
			log.Printf("...download stopped for %s...", transactionFile)
			return nil
		}
	}
}
