package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/oauth2"
)

// RateLimitSpecs is an instance of `rateLimitSpecs` that holds RL details received from the API response's headers.
var RateLimitSpecs rateLimitSpecs

// GithubEvents is the model to marshal the received JSON API data with.
type GithubEvents []struct { // https://mholt.github.io/json-to-go/
	ID    string `json:"id"`
	Type  string `json:"type"`
	Actor struct {
		ID           int    `json:"id"`
		DisplayLogin string `json:"display_login"`
		AvatarURL    string `json:"avatar_url"`
	} `json:"actor"`
	Repo struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"repo"`
	CreatedAt time.Time `json:"created_at"`
	Org       struct {
		ID         int    `json:"id"`
		Login      string `json:"login"`
		GravatarID string `json:"gravatar_id"`
		URL        string `json:"url"`
		AvatarURL  string `json:"avatar_url"`
	} `json:"org,omitempty"`
}

// rateLimitSpecs defines the fields from the API response's headers that concern Rate Limiting.
type rateLimitSpecs struct {
	Limit          int
	Remaining      int
	ResetTimestamp int64
	PollInterval   int
}

// APIError is the struct that is used to report API status errors. It is used to differ from 304 and 403 status codes.
type APIError struct {
	msg    string
	status int
}

func (e *APIError) Error() string {
	return e.msg + " (" + strconv.Itoa(e.status) + ")"
}

// parseHeader converts a header string to int and handles errors in conversion.
func parseHeader(header http.Header, fieldName string) int {
	if header.Get(fieldName) != "" {
		content, err := strconv.Atoi(header.Get(fieldName))
		if err != nil {
			log.Fatalf("Unable to parse header \"%s\"'s content: %s", fieldName, err.Error())
		}
		return content
	}
	return 0
}

// parseLongHeader converts a long header string to int64 and handles errors in conversion.
func parseLongHeader(header http.Header, fieldName string) int64 {
	if header.Get(fieldName) != "" {
		content, err := strconv.ParseInt(header.Get(fieldName), 10, 64)
		if err != nil {
			log.Fatalf("Unable to parse long header \"%s\"'s content: %s", fieldName, err.Error())
		}
		return content
	}
	return 0
}

// unauthenticatedGet performs an unauthenticated call to the GitHub's API.
// If given an `http.Client`, it will be used to perform the call instead of the standard one.
// This caveat is used to deduplicate code for the `authenticatedGet` function.
func unauthenticatedGet(pages int, page int, client *http.Client) (GithubEvents, error) {
	var httpClient *http.Client
	if client != nil {
		httpClient = client
	} else {
		httpClient = &http.Client{
			Timeout: time.Second * 30,
		}
	}
	r, err := httpClient.Get("https://api.github.com/events?page=" + strconv.Itoa(page))
	if err != nil {
		log.Fatalf("Error in requesting data from API: %s", err.Error())
	}
	defer r.Body.Close()

	RateLimitSpecs.Limit = parseHeader(r.Header, "x-ratelimit-limit")
	RateLimitSpecs.Remaining = parseHeader(r.Header, "x-ratelimit-remaining")
	RateLimitSpecs.ResetTimestamp = parseLongHeader(r.Header, "x-ratelimit-reset")
	RateLimitSpecs.PollInterval = parseHeader(r.Header, "x-poll-interval") * pages

	if r.StatusCode == http.StatusNotModified { // (304) No new content
		return nil, &APIError{"no new content", 304}
	} else if r.StatusCode == http.StatusForbidden { // (403) Rate limit reached
		return nil, &APIError{"rate limited", 403}
	}

	body, _ := ioutil.ReadAll(r.Body)
	var events GithubEvents
	json.Unmarshal(body, &events)

	return events, nil
}

// authenticatedGet performs an authenticated call to the GitHub's API using the user-supplied token.
// A custom call to `unauthenticatedGet` is made under the hood.
func authenticatedGet(pages int, page int, token string) (GithubEvents, error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	return unauthenticatedGet(pages, page, tc)
}

// GetHubData returns a success/failure boolean (respectively `true`/`false`) along with the marshalled API data.
// See the `GithubEvents` struct.
func GetHubData(pages int, page int, token string) (GithubEvents, error) {
	if token != "" {
		return authenticatedGet(pages, page, token)
	}
	return unauthenticatedGet(pages, page, nil)
}

// GetSpecsFromEventType returns the integer to use as group ID in the frontend graph.
// Each group is used to specifically colour a different type of event.
func GetSpecsFromEventType(eventType string) (int, string) {
	switch eventType { // https://developer.github.com/v3/activity/events/types/
	case "CommitCommentEvent":
		return 1, "Comment to commit" // TODO: should be appended to commit node, not to repo
	case "CreateEvent":
		return 2, "New repo created"
	case "DeleteEvent":
		return 3, "Something has been deleted"
	case "ForkEvent": // Fired on the parent repo!
		return 4, "Repo has been forked"
	case "GollumEvent":
		return 5, "Wiki page edited"
	case "IssueCommentEvent":
		return 6, "Issue has been commented"
	case "IssuesEvent":
		return 7, "An issue has changed"
	case "MemberEvent":
		return 8, "New collaborator added"
	case "PublicEvent":
		return 9, "Repo made public!"
	case "PullRequestEvent":
		return 10, "New pull request"
	case "PullRequestReviewCommentEvent":
		return 11, "PR's code has been commented"
	case "PushEvent":
		return 12, "New commit pushed"
	case "ReleaseEvent":
		return 13, "New release created"
	case "WatchEvent":
		return 14, "Repo has been starred"
	default:
		return 99, "Unknown event"
	}
}
