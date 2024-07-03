package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

type buildStatus int

var errNoAccessToPR = errors.New("No access to PR")

const (
	buildPending = iota
	buildSuccess
	buildFailure
	buildCanceled
)

type pullRequest struct {
	url       string
	title     string
	mergeable bool
}

func listUserPRs(token string) ([]pullRequest, []pullRequest, error) {
	githubUrl := "https://api.github.com/notifications?participating=true&all=true"
	req, err := http.NewRequest("GET", githubUrl, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode != 200 {
		msg := fmt.Sprintf("Got a '%s'error from the Github API. Please retry in a bit.",
			resp.Status)
		return nil, nil, errors.New(msg)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	var myPRs []pullRequest
	var otherPRs []pullRequest

	var jsonData []map[string]interface{}
	var PRsSeen = make(map[string]bool)

	err = json.Unmarshal(body, &jsonData)
	if err != nil {
		return nil, nil, err
	}

	for _, pr := range jsonData {
		reason := pr["reason"].(string)
		if reason != "author" && reason != "review_requested" {
			continue
		}

		subject := pr["subject"].(map[string]interface{})
		prAPIUrl := subject["url"].(string)
		fmt.Println("fetching one PR", subject["title"].(string))

		prData, err := fetchOneRESTObject(prAPIUrl, token)
		if err == errNoAccessToPR {
			// Skip PRs we don't have access to
			continue
		} else if err != nil {
			return nil, nil, err
		}

		prUrl := prData["_links"].(map[string]interface{})["html"].(map[string]interface{})["href"].(string)
		if _, ok := PRsSeen[prUrl]; ok {
			continue
		}

		PRsSeen[prUrl] = true
		if reason == "author" {
			myPRs = append(myPRs, pullRequest{
				url:       prUrl,
				title:     subject["title"].(string),
				mergeable: prData["mergeable"].(bool),
			})
		} else {
			otherPRs = append(otherPRs, pullRequest{
				url:       prUrl,
				title:     subject["title"].(string),
				mergeable: prData["mergeable"].(bool),
			})
		}
	}

	return myPRs, otherPRs, nil
}

func fetchOneRESTObject(url string, token string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var jsonData map[string]interface{}
	err = json.Unmarshal(body, &jsonData)
	if err != nil {
		return nil, err
	}

	if _, ok := jsonData["message"]; ok && jsonData["message"] == "Resource protected by organization SAML enforcement. You must grant your OAuth token access to this organization." {
		return nil, errNoAccessToPR
	}

	return jsonData, nil
}
