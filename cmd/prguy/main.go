package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os/exec"
	"reflect"
	"strings"
	"time"

	"fyne.io/systray"
	"github.com/ncruces/zenity"
	"github.com/skratchdot/open-golang/open"
)

const GH_CLIENT_ID = "Ov23liJtErJem2rhR36t"

var demoFlag = flag.Bool("demo", false, "Run the app in demo mode")

func main() {
	flag.Parse()
	fmt.Println("Demo mode:", *demoFlag)
	onExit := func() {
	}

	systray.Run(onReady, onExit)
}

func errorOut(title string, message string) {
	zenity.Error(message, zenity.Title(title), zenity.ErrorIcon)
}

func setupMenu(ctx context.Context, cancel context.CancelFunc) {
	systray.SetTemplateIcon([]byte("🕴️"), []byte("🕴️"))
	systray.SetTitle("🕴️")
	config := Config{}

	go func() {
		if !config.exists() {
			mDoSetup := systray.AddMenuItem("GitHub setup", "Authenticate yourself to be able to see your pull requests.")

			mQuitOrig := systray.AddMenuItem("Quit", "Quit the app")

			for {
				select {
				case <-ctx.Done():
					systray.ResetMenu()
					return

				case <-mQuitOrig.ClickedCh:
					fmt.Println("Requesting quit")
					systray.Quit()
				case <-mDoSetup.ClickedCh:
					fmt.Println("Performing setup")
					err := startGithubDeviceAuth(cancel)
					if err == nil {
						fmt.Println("Setup complete")
						cancel()
						return
					}
				}
			}
		} else {
			cfg := Config{}
			var channels []chan struct{}
			var channelsMap = make(map[chan struct{}]pullRequest)

			cfg.load()
			systray.ResetMenu()
			statusItem := systray.AddMenuItem("Fetching PRs from Github...", "")
			statusItem.Disable()

			fmt.Println("Fetching PRs...")
			myPRs, otherPRs, err := listUserPRs(cfg.OAuthToken, *demoFlag)
			if err != nil {
				fmt.Println("Error fetching PRs:", err)
				time.Sleep(35 * time.Second)
				return
			}

			systray.ResetMenu()

			if len(myPRs.Keys()) == 0 {
				systray.AddMenuItem("No PRs out from you – yet", "")
			} else {
				for _, repo := range myPRs.Keys() {
					prs, _ := myPRs.Get(repo)
					prList := prs.([]pullRequest)
					if len(prList) == 0 {
						continue
					}

					systray.AddMenuItem(repo, "").Disable()
					for _, pr := range prList {
						systrayItem := renderPR(pr)
						channels = append(channels, systrayItem.ClickedCh)
						channelsMap[systrayItem.ClickedCh] = pr
					}
				}
			}

			systray.AddSeparator()

			if len(myPRs.Keys()) != 0 && len(otherPRs.Keys()) == 0 {
				systray.AddMenuItem("No PRs to review, no news is good news.", "")
			} else if len(otherPRs.Keys()) != 0 {
				for _, repo := range otherPRs.Keys() {
					prs, _ := otherPRs.Get(repo)
					prList := prs.([]pullRequest)
					if len(prList) == 0 {
						continue
					}

					systray.AddMenuItem(repo, "").Disable()
					for _, pr := range prList {
						systrayItem := renderPR(pr)
						channels = append(channels, systrayItem.ClickedCh)
						channelsMap[systrayItem.ClickedCh] = pr
					}
				}
			}

			systray.AddSeparator()

			mQuitOrig := systray.AddMenuItem("Quit", "Quit the app")
			channels = append(channels, mQuitOrig.ClickedCh)

			for {
				i, ok := selectChannels(channels)
				if !ok {
					continue
				}

				if i == len(channels)-1 {
					// quit
					fmt.Println("Requesting quit")
					systray.Quit()
				} else {
					// open the PR
					pr := channelsMap[channels[i]]
					open.Run(pr.url)
				}
			}
		}
	}()
}

func onReady() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop() // Ensure the ticker is stopped when we're done with it

		ctx, cancel := context.WithCancel(context.Background())
		setupMenu(ctx, cancel)
		for {
			select {
			case <-ctx.Done():
				ctx, cancel = context.WithCancel(context.Background())
				fmt.Println("Refreshing menu after auth")
				setupMenu(ctx, cancel)
			case <-ticker.C:
				cancel()
				ctx, cancel = context.WithCancel(context.Background())
				fmt.Println("Refreshing menu")
				setupMenu(ctx, cancel)
			}
		}
	}()
}

func startGithubDeviceAuth(cancel context.CancelFunc) error {
	formData := url.Values{
		"client_id": []string{GH_CLIENT_ID},
		"scope":     []string{"notifications repo"},
	}

	encodedForm := formData.Encode()
	req, err := http.NewRequest("POST",
		"https://github.com/login/device/code",
		strings.NewReader(encodedForm))
	if err != nil {
		errorOut("Error building request to the Github API", err.Error())
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		errorOut("Github API error", err.Error())
		return err
	}

	if resp.StatusCode != 200 {
		return err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errorOut("Error reading response body", err.Error())
		return err
	}

	// Parse the form-encoded response
	parsedResponse, err := url.ParseQuery(string(body))
	if err != nil {
		errorOut("Github API error", err.Error())
		return err
	}

	device_code, ok := parsedResponse["device_code"]
	if !ok {
		errorOut("Missing device code!",
			"The Github API did not return us a device code, please retry in a bit.")
		return err
	}

	user_code, ok := parsedResponse["user_code"]
	if !ok {
		errorOut("Missing user code!",
			"The Github API did not return us a user code, please retry in a bit.")
		return err
	}

	verification_uri, ok := parsedResponse["verification_uri"]
	if !ok {
		errorOut("Missing verification uri!",
			"The Github API did not return us a verification uri, please retry in a bit.")
		return err
	}

	writeToClipboard(user_code[0])
	zenity.Info("We just copied an authentication code in your clipboard. You will need to give that code to Github",
		zenity.NoIcon,
		zenity.Title("Github Setup"),
		zenity.OKLabel("Log into Github"))

	err = open.Run(verification_uri[0])
	if err != nil {
		errorOut("Error opening the browser", err.Error())
		return err
	}

	err = pollGithubDeviceAuth(device_code[0], cancel)
	if err != nil {
		return err
	}

	return nil
}

func pollGithubDeviceAuth(deviceCode string, cancel context.CancelFunc) error {
	duration := 15 * time.Minute
	startTime := time.Now()
	for {
		time.Sleep(10 * time.Second)

		// Check if the time has passed
		if time.Since(startTime) > duration {
			errorOut("Timeout", "The device code has expired. Please retry the auth process.")
			return errors.New("auth_timeout")
		}

		fmt.Println("Polling Github API...")
		formData := url.Values{
			"client_id":   []string{GH_CLIENT_ID},
			"device_code": []string{deviceCode},
			"grant_type":  []string{"urn:ietf:params:oauth:grant-type:device_code"},
		}

		encodedForm := formData.Encode()
		req, err := http.NewRequest("POST",
			"https://github.com/login/oauth/access_token",
			strings.NewReader(encodedForm))

		if err != nil {
			errorOut("Error building request to the Github API", err.Error())
			continue
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			errorOut("Github API error", err.Error())
			continue
		}

		if resp.StatusCode == 429 {
			continue
		} else if resp.StatusCode != 200 {
			return errors.New("api_error")
		}

		// Read the response body
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			errorOut("Error reading response body", err.Error())
			continue
		}

		// Parse the form-encoded response
		parsedResponse, err := url.ParseQuery(string(body))
		if err != nil {
			errorOut("Error parsing Github response", err.Error())
			continue
		}

		access_token, ok := parsedResponse["access_token"]
		if !ok {
			// sometimes the github API does not error but also does not return the access token.
			// in this case we just retry.
			continue
		}

		_, ok = parsedResponse["scope"]
		if !ok {
			errorOut("Missing scopes!",
				"The Github API did not return us the scopes, please retry in a bit.")
			continue
		}

		// Save the access token and scope
		fmt.Println("Saving access token and scope")
		c := Config{OAuthToken: access_token[0]}
		c.save()
		cancel()
		return nil
	}
}

func writeToClipboard(text string) error {
	copyCmd := exec.Command("pbcopy")
	in, err := copyCmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := copyCmd.Start(); err != nil {
		return err
	}
	if _, err := in.Write([]byte(text)); err != nil {
		return err
	}
	if err := in.Close(); err != nil {
		return err
	}
	return copyCmd.Wait()
}

func selectChannels(chans []chan struct{}) (int, bool) {
	// straight from https://go.dev/play/p/wCchjGndBC
	var cases []reflect.SelectCase
	for _, ch := range chans {
		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch),
			Send: reflect.Value{},
		})
	}

	i, _, ok := reflect.Select(cases)
	return i, ok
}

func renderPR(pr pullRequest) *systray.MenuItem {
	var pr_status string
	var build_status string

	if pr.mergeable {
		pr_status = "✅"
	} else {
		pr_status = "❌"
	}

	if pr.buildStatus == buildSuccess {
		build_status = "🟢"
	} else if pr.buildStatus == buildFailure {
		build_status = "🔴"
	} else {
		build_status = "🔵"
	}

	title := fmt.Sprintf("%s%s %s", pr_status, build_status, pr.title)
	return systray.AddMenuItem(title, "")
}
