package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/oauth2/clientcredentials"
)

type (
	Project struct {
		ProjectID int    `json:"id"`
		Name      string `json:"name"`
		Exam      bool   `json:"exam"`
	}

	Team struct {
		TeamID     int        `json:"id"`
		ProjectID  int        `json:"project_id"`
		RepoURL    string     `json:"repo_url"`
		Users      []User     `json:"users"`
		lastUpdate *time.Time `json:"-"`
	}

	User struct {
		UserID         int    `json:"id"`
		Login          string `json:"login"`
		ProjectsUserID int    `json:"projects_user_id"`
	}
)

var projects = make(map[int]*Project)

func getClient(ctx context.Context, scopes ...string) *http.Client {
	oauth := clientcredentials.Config{
		ClientID:     config.IntraClientID,
		ClientSecret: config.IntraClientSecret,
		TokenURL:     "https://api.intra.42.fr/oauth/token",
		Scopes:       scopes,
	}
	return oauth.Client(ctx)
}

func getEndpoint(path string, options map[string]string) string {
	baseURL, err := url.Parse("https://api.intra.42.fr/v2/")
	if err != nil {
		log.Println(err)
		return ""
	}
	baseURL.Path += path
	params := url.Values{}
	for key, value := range options {
		params.Add(key, value)
	}
	baseURL.RawQuery = params.Encode()
	return baseURL.String()
}

func getProject(ID int) (*Project, error) {
	if p, ok := projects[ID]; ok {
		return p, nil
	}
	client := getClient(context.Background(), "public", "projects")
	endpoint := getEndpoint("projects/"+strconv.Itoa(ID), map[string]string{})
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("Intra error [Response: %d]", resp.StatusCode))
	}
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	p := &Project{}
	err = json.Unmarshal(respData, &p)
	if err == nil {
		projects[ID] = p
	}
	return p, err
}

func getTeamsPage(client *http.Client, params map[string]string, pageNumber int) ([]Team, error) {
	params["page[number]"] = strconv.Itoa(pageNumber)
	endpoint := getEndpoint("teams", params)
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("Intra error [Response: %d]", resp.StatusCode))
	}
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var teams []Team
	err = json.Unmarshal(respData, &teams)
	return teams, err
}

func getAllTeams(params map[string]string) ([]Team, error) {
	client := getClient(context.Background(), "public", "projects")
	pageNumber := 1
	if num, ok := params["page[number]"]; ok {
		pageNumber, _ = strconv.Atoi(num)
	}
	var teams []Team
	for {
		res, err := getTeamsPage(client, params, pageNumber)
		if err != nil {
			return teams, err
		}
		if len(res) == 0 {
			break
		}
		teams = append(teams, res...)
		pageNumber++
	}
	return teams, nil
}
