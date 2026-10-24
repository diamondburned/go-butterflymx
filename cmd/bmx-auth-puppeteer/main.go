package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"golang.org/x/oauth2"
	"libdb.so/go-butterflymx"
)

func main() {
	var (
		browser     = ""
		headless    = false
		userDataDir = ""
		timeout     = 5 * time.Minute
	)

	flag.StringVar(&browser, "browser", browser, "path to browser executable (default: auto-detected from CHROMIUM_EXECUTABLE or PATH)")
	flag.BoolVar(&headless, "headless", headless, "run browser in headless mode")
	flag.StringVar(&userDataDir, "user-data-dir", userDataDir, "path to browser user data directory")
	flag.DurationVar(&timeout, "timeout", timeout, "timeout waiting for authorization (0 to disable)")

	log.SetFlags(0)
	flag.Parse()

	execPath, err := findBrowserExec(browser)
	if err != nil {
		log.Fatalf("failed to locate browser: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	flow := butterflymx.NewAuthFlowClient()
	flowStart := flow.Start()

	authURL := flowStart.URL()
	log.Println("Spawning browser to authorize ButterflyMX application...")

	var authCtx context.Context
	var cancelTimeout context.CancelFunc
	if timeout > 0 {
		authCtx, cancelTimeout = context.WithTimeout(ctx, timeout)
		defer cancelTimeout()
	} else {
		authCtx = ctx
	}

	redirectedURL, err := runAuthBrowser(authCtx, authURL, browserOptions{
		BrowserExec: execPath,
		Headless:    headless,
		UserDataDir: userDataDir,
	})
	if err != nil {
		if ctx.Err() != nil {
			log.Fatalf("aborted: %v", ctx.Err())
		}
		if timeout > 0 && authCtx.Err() == context.DeadlineExceeded {
			log.Fatalf("timed out waiting for authorization after %v", timeout)
		}
		log.Fatalf("authorization failed: %v", err)
	}

	log.Println()
	log.Println("Intercepted authorization redirect successfully.")

	token, err := flow.Finish(ctx, flowStart, redirectedURL)
	if err != nil {
		log.Fatalf("failed to finish oauth2 auth flow: %v", err)
	}

	log.Println()
	log.Println("Successfully obtained OAuth2 token:")
	fmt.Println("oauth2_token:", token.AccessToken)

	loginClient := butterflymx.NewDenizenLoginClient(oauth2.StaticTokenSource(token))

	apiToken, err := loginClient.APIToken(ctx, true)
	if err != nil {
		log.Fatalf("failed to get API token: %v", err)
	}

	log.Println()
	log.Println("Successfully obtained API token:")
	fmt.Println("api_token:", apiToken)
}

type browserOptions struct {
	BrowserExec string
	Headless    bool
	UserDataDir string
}

func findBrowserExec(custom string) (string, error) {
	if custom != "" {
		return exec.LookPath(custom)
	}
	if env := os.Getenv("CHROMIUM_EXECUTABLE"); env != "" {
		if path, err := exec.LookPath(env); err == nil {
			return path, nil
		}
	}
	candidates := []string{
		"chromium",
		"google-chrome",
		"google-chrome-stable",
		"chromium-browser",
		"chrome",
	}
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no chromium executable found in CHROMIUM_EXECUTABLE or PATH (checked: %s)", strings.Join(candidates, ", "))
}

// runAuthBrowser launches a browser to the authorization URL, monitors network
// events, and intercepts the redirected URL with the custom ButterflyMX scheme.
func runAuthBrowser(ctx context.Context, authURL string, opts browserOptions) (string, error) {
	execPath := opts.BrowserExec
	if execPath == "" {
		var err error
		execPath, err = findBrowserExec("")
		if err != nil {
			return "", fmt.Errorf("failed to locate browser: %w", err)
		}
	}

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", opts.Headless),
		chromedp.ExecPath(execPath),
	)

	if opts.UserDataDir != "" {
		allocOpts = append(allocOpts, chromedp.UserDataDir(opts.UserDataDir))
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer cancelAlloc()

	taskCtx, cancelTask := chromedp.NewContext(allocCtx)
	defer cancelTask()

	redirectURLChan := make(chan string, 1)

	checkURL := func(u string) {
		if strings.HasPrefix(u, "com.butterflymx.oauth://") {
			select {
			case redirectURLChan <- u:
			default:
			}
		}
	}

	chromedp.ListenTarget(taskCtx, func(ev any) {
		switch ev := ev.(type) {
		case *network.EventRequestWillBeSent:
			checkURL(ev.Request.URL)
			if ev.RedirectResponse != nil {
				if loc, ok := ev.RedirectResponse.Headers["Location"].(string); ok {
					checkURL(loc)
				} else if loc, ok := ev.RedirectResponse.Headers["location"].(string); ok {
					checkURL(loc)
				}
			}
		case *network.EventResponseReceived:
			if loc, ok := ev.Response.Headers["Location"].(string); ok {
				checkURL(loc)
			} else if loc, ok := ev.Response.Headers["location"].(string); ok {
				checkURL(loc)
			}
		case *fetch.EventRequestPaused:
			checkURL(ev.Request.URL)
			go func() {
				cdpExecutor := cdp.WithExecutor(taskCtx, chromedp.FromContext(taskCtx).Target)
				if strings.HasPrefix(ev.Request.URL, "com.butterflymx.oauth://") {
					_ = fetch.FailRequest(ev.RequestID, network.ErrorReasonAccessDenied).Do(cdpExecutor)
				} else {
					_ = fetch.ContinueRequest(ev.RequestID).Do(cdpExecutor)
				}
			}()
		}
	})

	navErr := chromedp.Run(taskCtx,
		network.Enable(),
		fetch.Enable().WithPatterns([]*fetch.RequestPattern{
			{URLPattern: "*butterflymx.oauth*"},
		}),
		chromedp.Navigate(authURL),
	)

	// Check if redirect occurred during navigation.
	select {
	case u := <-redirectURLChan:
		return u, nil
	default:
		if navErr != nil {
			return "", fmt.Errorf("browser error during navigation: %w", navErr)
		}
	}

	log.Println()
	log.Println("Waiting for login and authorization redirect...")
	select {
	case u := <-redirectURLChan:
		return u, nil
	case <-taskCtx.Done():
		return "", fmt.Errorf("browser was closed before authorization completed")
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
