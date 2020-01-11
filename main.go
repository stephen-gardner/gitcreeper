package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"

	"gitcreeper/intra"
)

type Config struct {
	RepoDomain         string
	RepoAddress        string
	RepoPort           int
	RepoUser           string
	RepoPrivateKeyPath string
	RepoPath           string
	EmailServerAddress string
	EmailFromAddress   string
	StartDate          string
	DaysUntilStagnant  int
	CampusID           int
	CursusIDs          []int
	ProjectWhitelist   []int
}

const intraTimeFormat = "2006-01-02T15:04:05.000Z"
const gitTimeFormat = "Mon Jan 2 15:04:05 2006 -0700"

var config Config
var projectWhitelist = make(map[int]bool)

func getEligibleTeams(expirationDate time.Time) (teams intra.Teams) {
	fmt.Printf("Getting eligible teams from 42 Intra...\n")
	cursus := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(config.CursusIDs)), ","), "[]")
	teams, err := intra.GetAllTeams(
		context.Background(),
		map[string]string{
			"filter[primary_campus]": strconv.Itoa(config.CampusID),
			"filter[active_cursus]":  cursus,
			"filter[closed]":         "false",
			"range[created_at]":      config.StartDate + "," + expirationDate.Format(intraTimeFormat),
			"page[size]":             "100",
		},
	)
	if err != nil {
		log.Println(err)
	}
	return
}

func processTeams(teams intra.Teams, expirationDate time.Time) {
	fmt.Printf("Processing...\n\n")
	count := 0
	for _, team := range teams {
		_, whitelisted := projectWhitelist[team.ProjectID]
		if !whitelisted || !strings.Contains(team.RepoURL, config.RepoDomain) {
			continue
		}
		stagnant, _, err := checkStagnant(&team, expirationDate)
		if err != nil {
			log.Println(err)
			continue
		}
		if stagnant {
			//err = sendEmail(&team, lastUpdate)
			count++
		}
	}
	fmt.Printf("\n%d total teams have stagnated\n", count)
}

func main() {
	data, err := ioutil.ReadFile("config.json")
	if err == nil {
		err = json.Unmarshal(data, &config)
	}
	if err != nil {
		log.Fatal(err)
	}
	for _, ID := range config.ProjectWhitelist {
		projectWhitelist[ID] = true
	}
	expirationDate := time.Now().Add(- (time.Duration(config.DaysUntilStagnant) * 24 * time.Hour))
	teams := getEligibleTeams(expirationDate)
	processTeams(teams, expirationDate)
}
