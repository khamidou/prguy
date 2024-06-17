package main

import (
	"fmt"
    "os/exec"
    "time"
    "errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/getlantern/systray"
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

func onReady() {
	systray.SetTemplateIcon([]byte("🕴️"), []byte("🕴️"))
	systray.SetTitle("PR Guy")
	//systray.SetTooltip("")
	mDoSetup := systray.AddMenuItem("GitHub setup", "Authenticate yourself to be able to see your pull requests.")
	mQuitOrig := systray.AddMenuItem("Quit", "Quit the app")

	go func() {
		for {
			select {
			case <-mQuitOrig.ClickedCh:
				fmt.Println("Requesting quit")
				systray.Quit()
				fmt.Println("Finished quitting")
			case <-mDoSetup.ClickedCh:
				fmt.Println("Performing setup")
				startGithubDeviceAuth()
			}
		}
	}()
}

func startGithubDeviceAuth() {
	formData := url.Values{
		"client_id": []string{GH_CLIENT_ID},
		"scope":     []string{"notifications"},
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
		errorOut("Github API error",
			fmt.Sprint("Got a '%s'error from the Github API. Please retry in a bit.",
                       resp.Status)
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
	fmt.Println("body", string(body))
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

    fmt.Println("user_code", user_code[0])
    writeToClipboard(user_code[0])
	zenity.Error("We just copied an authentication code in your clipboard. You will need to give that code to Github",
		zenity.Title("Github Setup"),
		zenity.OKLabel("Log into Github"),)

	err = open.Run(verification_uri[0])
	if err != nil {
		errorOut("Error opening the browser", err.Error())
		return
	}

	err = pollGithubDeviceAuth(device_code[0])
	if err != nil {
		return
	}
}

func pollGithubDeviceAuth(deviceCode string) error {
	duration := 15 * time.Minute
	startTime := time.Now()
	for {
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
			"https://github.com/login/device/code",
			strings.NewReader(encodedForm))

		if err != nil {
			errorOut("Error building request to the Github API", err.Error())
			return errors.New("request_error")
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			errorOut("Github API error", err.Error())
			return errors.New("api_error")
		}

        if resp.StatusCode == 429 {
            time.Sleep(15 * time.Second)
            continue
        } else if resp.StatusCode != 200 {
			errorOut("Github API error",
				"Got a "+string(resp.Status)+
					"error from the Github API. "+
					"Please retry in a bit.")
			return errors.New("api_error")
		}

		// Read the response body
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			errorOut("Error reading response body", err.Error())
			return errors.New("response_error")
		}

		// Parse the form-encoded response
		parsedResponse, err := url.ParseQuery(string(body))
		if err != nil {
			errorOut("Error parsing Github response", err.Error())
			return errors.New("response_error")
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
			return errors.New("response_error")
		}

		fmt.Println("access_token", access_token)
		fmt.Println("scope", scope)

        time.Sleep(5 * time.Second)
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
