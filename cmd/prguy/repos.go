package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/iancoleman/orderedmap"
	"io/ioutil"
	"net/http"
	"time"
)

type buildStatus int

var errNoAccessToPR = errors.New("No access to PR")

const (
	buildPending buildStatus = iota
	buildSuccess
	buildFailure
	buildCanceled
)

type pullRequest struct {
	url         string
	title       string
	mergeable   bool
	buildStatus buildStatus
}

func listUserPRs(token string, isDemo bool) (*orderedmap.OrderedMap, *orderedmap.OrderedMap, error) {
	myPRs := orderedmap.New()
	otherPRs := orderedmap.New()

	if isDemo {
		myPRs.Set("michael/repo", []pullRequest{
			{url: "blah", title: "Unbreak prod", mergeable: true, buildStatus: buildFailure},
			{url: "blah", title: "Fix the tests michael broke", mergeable: true, buildStatus: buildPending}})

		otherPRs.Set("michael/repo", []pullRequest{
			{url: "blah", title: "Revert michael's 'fix'", mergeable: true, buildStatus: buildFailure},
		})

		return myPRs, otherPRs, nil
	}

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

		fmt.Println("Fetching PR data", prAPIUrl)
		prData, err := fetchOneRESTObject(prAPIUrl, token)
		if err == errNoAccessToPR {
			// Skip PRs we don't have access to
			continue
		} else if err != nil {
			return nil, nil, err
		}

		var prSHA string
		var repoFullName string
		prStatus := buildPending
		if _, ok := prData["head"]; ok {
			headInfo := prData["head"].(map[string]interface{})
			if _, ok := headInfo["sha"]; ok {
				prSHA = headInfo["sha"].(string)
			}

			if _, ok := headInfo["repo"]; ok {
				repoInfo := headInfo["repo"].(map[string]interface{})
				repoFullName = repoInfo["full_name"].(string)
			}
		}

		merged := prData["merged"].(bool)
		if merged {
			continue
		}

		if prSHA != "" && repoFullName != "" {
			fmt.Println("Fetching build status", repoFullName, prSHA)
			prStatus, err = getBuildStatus(repoFullName, prSHA, token)
			if err != nil {
				prStatus = buildPending
			}
		}

		prUrl := prData["_links"].(map[string]interface{})["html"].(map[string]interface{})["href"].(string)
		if _, ok := PRsSeen[prUrl]; ok {
			continue
		}

		PRsSeen[prUrl] = true
		mergeableState := prData["mergeable_state"].(string)
		mergeable := mergeableState == "clean" || mergeableState == "has_hooks"
		if reason == "author" {
			if _, ok := myPRs.Get(repoFullName); !ok {
				myPRs.Set(repoFullName, []pullRequest{})
			}

			prContents := pullRequest{
				url:         prUrl,
				title:       subject["title"].(string),
				mergeable:   mergeable,
				buildStatus: prStatus,
			}
			contents, _ := myPRs.Get(repoFullName)
			var PRsList = contents.([]pullRequest)
			PRsList = append(PRsList, prContents)
			myPRs.Set(repoFullName, PRsList)
		} else {
			if _, ok := otherPRs.Get(repoFullName); !ok {
				otherPRs.Set(repoFullName, []pullRequest{})
			}

			prContents := pullRequest{url: prUrl,
				title:       subject["title"].(string),
				mergeable:   mergeable,
				buildStatus: prStatus,
			}
			contents, _ := otherPRs.Get(repoFullName)
			var PRsList = contents.([]pullRequest)
			PRsList = append(PRsList, prContents)
			otherPRs.Set(repoFullName, PRsList)
		}

		time.Sleep(1 * time.Second)
	}

	return myPRs, otherPRs, nil
}

func getApprovalStatus(prUrl string, token string) (bool, error) {
	return false, nil
}

func getBuildStatus(repoFullName string, sha string, token string) (buildStatus, error) {
	possibleUrls := []string{
		fmt.Sprintf("https://api.github.com/repos/%s/commits/%s/check-runs",
			repoFullName, sha),
		fmt.Sprintf("https://api.github.com/repos/%s/commits/%s/status",
			repoFullName, sha),
	}

	// There's two different APIs for statuses, so we try both
	for _, url := range possibleUrls {
		resp, err := fetchOneRESTObject(url, token)
		if err != nil {
			continue
		}

		if _, ok := resp["check_runs"]; ok {
			for _, checkRun := range resp["check_runs"].([]interface{}) {
				checkRunMap := checkRun.(map[string]interface{})
				if checkRunMap["status"].(string) == "completed" {
					if checkRunMap["conclusion"].(string) == "success" {
						return buildSuccess, nil
					} else {
						return buildFailure, nil
					}
				}
			}
		} else if _, ok := resp["state"]; ok {
			state := resp["state"].(string)
			switch state {
			case "success":
				return buildSuccess, nil
			case "failure":
				return buildFailure, nil
			case "pending":
				continue
			case "error":
				return buildFailure, nil
			}
		}
	}

	return buildPending, errors.New("Unknown build status")
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
