package main

import (
	"context"
	"errors"
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

func main() {
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
					startGithubDeviceAuth(cancel)
				}
			}
		} else {
			cfg := Config{}
			var channels []chan struct{}
			var channelsMap = make(map[chan struct{}]pullRequest)

			cfg.load()
			systray.ResetMenu()

			fmt.Println("Fetching PRs...")
			myPRs, otherPRs, err := listUserPRs(cfg.OAuthToken)
			if err != nil {
				fmt.Println("Error fetching PRs:", err)
				time.Sleep(35 * time.Second)
				return
			}

			if len(myPRs) == 0 {
				systray.AddMenuItem("No PRs out from you, let's get after it!", "")
			} else {
				for _, pr := range myPRs {
					var pr_status string
					if pr.mergeable {
						pr_status = "✅"
					} else {
						pr_status = "❌"
					}

					title := fmt.Sprintf("%-*s %s", 50, pr.title, pr_status)
					channel := systray.AddMenuItem(title, "")
					channels = append(channels, channel.ClickedCh)
					channelsMap[channel.ClickedCh] = pr
				}
			}

			systray.AddSeparator()

			if len(myPRs) != 0 && len(otherPRs) == 0 {
				systray.AddMenuItem("No PRs to review, no news is good news.", "")
			} else if len(otherPRs) != 0 {
				for _, pr := range otherPRs {
					var pr_status string
					if pr.mergeable {
						pr_status = "✅"
					} else {
						pr_status = "❌"
					}

					title := fmt.Sprintf("%-*s %s", 50, pr.title, pr_status)
					channel := systray.AddMenuItem(title, "")
					channels = append(channels, channel.ClickedCh)
					channelsMap[channel.ClickedCh] = pr
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
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop() // Ensure the ticker is stopped when we're done with it

		ctx, cancel := context.WithCancel(context.Background())
		setupMenu(ctx, cancel)
		for {
			select {
			case <-ticker.C:
				// refresh the menu every 15 minutes
				cancel()
				ctx, cancel = context.WithCancel(context.Background())
				setupMenu(ctx, cancel)
			}
		}
	}()
}

func startGithubDeviceAuth(cancel context.CancelFunc) {
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
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		errorOut("Github API error", err.Error())
		return
	}

	if resp.StatusCode != 200 {
		return
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errorOut("Error reading response body", err.Error())
		return
	}

	// Parse the form-encoded response
	parsedResponse, err := url.ParseQuery(string(body))
	if err != nil {
		errorOut("Github API error", err.Error())
		return
	}

	device_code, ok := parsedResponse["device_code"]
	if !ok {
		errorOut("Missing device code!",
			"The Github API did not return us a device code, please retry in a bit.")
		return
	}

	user_code, ok := parsedResponse["user_code"]
	if !ok {
		errorOut("Missing user code!",
			"The Github API did not return us a user code, please retry in a bit.")
		return
	}

	verification_uri, ok := parsedResponse["verification_uri"]
	if !ok {
		errorOut("Missing verification uri!",
			"The Github API did not return us a verification uri, please retry in a bit.")
		return
	}

	writeToClipboard(user_code[0])
	zenity.Error("We just copied an authentication code in your clipboard. You will need to give that code to Github",
		zenity.Title("Github Setup"),
		zenity.OKLabel("Log into Github"))

	err = open.Run(verification_uri[0])
	if err != nil {
		errorOut("Error opening the browser", err.Error())
		return
	}

	err = pollGithubDeviceAuth(device_code[0], cancel)
	if err != nil {
		return
	}
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

		scope, ok := parsedResponse["scope"]
		if !ok {
			errorOut("Missing scopes!",
				"The Github API did not return us the scopes, please retry in a bit.")
			continue
		}

		// Save the access token and scope
		c := Config{OAuthToken: access_token[0], Scope: scope[0]}
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
