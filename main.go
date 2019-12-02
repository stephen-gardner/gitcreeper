package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os/exec"

	"golang.org/x/oauth2/clientcredentials"
)

type Config struct {
	KLogin            string
	KeytabPath        string
	ClonePath         string
	StartDate         string
	CampusID          string
	CursusID          string
	DaysUntilStagnant int
	ProjectWhitelist  []int
}

const intraTimeFormat = "2006-01-02T15:04:05.000Z"
const gitTimeFormat = "Mon Jan 2 15:04:05 2006 -0700"

var config Config

func getClient(ctx context.Context, scopes ...string) *http.Client {
	oauth := clientcredentials.Config{
		ClientID:     "025fe928cedf48c95ec5d98a30b5ad4862c200f58d53398f5c8f2a1609b798e1",
		ClientSecret: "03af0c8cacd386fd0b2c6efcd9fa33d444fac016a24a0676a110754ef9452f5a",
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

func isWhitelisted(projectID int) bool {
	for _, ID := range config.ProjectWhitelist {
		if ID == projectID {
			return true
		}
	}
	return false
}

func main() {
	data, err := ioutil.ReadFile("config.json")
	if err == nil {
		err = json.Unmarshal(data, &config)
	}
	if err != nil {
		log.Fatal(err)
	}
	if err = exec.Command(
		"/bin/sh",
		"-c", fmt.Sprintf("kinit -kt '%s' %s", config.KeytabPath, config.KLogin),
	).Run(); err != nil {
		log.Fatalf("Error authenticating via Kerberos: %s\n", err)
	}
	stagnantTeams := getStagnantTeams()
	for _, team := range stagnantTeams {
		res := ""
		if team.lastUpdate != nil {
			res = fmt.Sprintf("stagnant [last commit: %s]", team.lastUpdate.String())
		} else {
			res += "stagnant [no commits]"
		}
		proj, _ := getProject(team.ProjectID)
		fmt.Printf("%d <%s> (%s) is %s\n", team.TeamID, proj.Name, team.getIntraIDs(), res)
	}
}
