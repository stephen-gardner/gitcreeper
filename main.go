package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os/exec"

	"golang.org/x/oauth2/clientcredentials"
)

const campusID = "7"
const cursusID = "1"
const intraTimeFormat = "2006-01-02T15:04:05.000Z"
const startDate = "2019-11-15T00:00:00.000Z"
const daysUntilStagnant = 7
const cloneDest = "/tmp/creeped"

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

func main() {
	err := exec.Command(
		"/bin/sh",
		"-c", "kinit -kt '/Users/stephen/keytabs/alain.keytab' alain@42.US.ORG",
	).Run()
	if err != nil {
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
