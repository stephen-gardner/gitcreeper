package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"
	"time"
)

type Config struct {
	IntraClientID      string
	IntraClientSecret  string
	RepoServer         string
	KLogin             string
	KeytabPath         string
	ClonePath          string
	EmailServerAddress string
	EmailFromAddress   string
	StartDate          string
	CampusID           string
	CursusID           string
	DaysUntilStagnant  int
	ProjectWhitelist   []int
}

const intraTimeFormat = "2006-01-02T15:04:05.000Z"
const gitTimeFormat = "Mon Jan 2 15:04:05 2006 -0700"

var config Config

func isWhitelisted(projectID int) bool {
	for _, ID := range config.ProjectWhitelist {
		if ID == projectID {
			return true
		}
	}
	return false
}

func getStagnantTeams() []Team {
	var stagnantTeams []Team
	expirationDate := time.Now().Add(- (time.Duration(config.DaysUntilStagnant) * 24 * time.Hour))
	teams, err := getAllTeams(
		map[string]string{
			"filter[primary_campus]": config.CampusID,
			"filter[active_cursus]":  config.CursusID,
			"filter[closed]":         "false",
			"range[created_at]":      config.StartDate + "," + expirationDate.Format(intraTimeFormat),
			"page[size]":             "100",
		},
	)
	if err != nil {
		log.Println(err)
		if len(teams) == 0 {
			return stagnantTeams
		}
	}
	for _, team := range teams {
		if !isWhitelisted(team.ProjectID) || !strings.HasPrefix(team.RepoURL, config.RepoServer) {
			continue
		}
		proj, err := team.getProject()
		if err != nil {
			log.Printf("Error retrieving project info for ID %d: %s\n", team.ProjectID, err)
			continue
		}
		fmt.Printf(
			"Checking %d <%s> (%s)...\n",
			team.TeamID,
			proj.Name,
			strings.Join(team.getIntraIDs(), ", "),
		)
		if err = team.cloneRepo(); err == nil {
			stagnant := false
			stagnant, err = team.isStagnant(expirationDate)
			if err == nil && stagnant {
				stagnantTeams = append(stagnantTeams, team)
			}
			team.deleteClone()
		}
		if err != nil {
			log.Println(err)
		}
	}
	return stagnantTeams
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
	fmt.Printf("%d teams have stagnated; sending emails...\n", len(stagnantTeams))
	for _, team := range stagnantTeams {
		if err := team.sendEmail(); err != nil {
			log.Println(err)
		}
	}
}
