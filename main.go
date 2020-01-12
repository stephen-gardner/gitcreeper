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

// Return teams that may be stagnant according to config
func getEligibleTeams(expirationDate time.Time) (res intra.Teams) {
	fmt.Printf("Getting eligible teams from 42 Intra... ")
	// Some teams may belong to more than one cursus
	eligibleTeams := make(map[int]intra.Team)
	for _, cursusID := range config.CursusIDs {
		teams, err := intra.GetAllTeams(
			context.Background(),
			map[string]string{
				"filter[primary_campus]": strconv.Itoa(config.CampusID),
				"filter[active_cursus]":  strconv.Itoa(cursusID),
				"filter[closed]":         "false",
				"range[created_at]":      config.StartDate + "," + expirationDate.Format(intraTimeFormat),
				"page[size]":             "100",
			},
		)
		if err != nil {
			log.Println(err)
		}
		// Check if team is on the whitelist and that it has a local repository
		for _, team := range teams {
			_, whitelisted := projectWhitelist[team.ProjectID]
			if !whitelisted || !strings.Contains(team.RepoURL, config.RepoDomain) {
				continue
			}
			eligibleTeams[team.ID] = team
		}
	}
	res = make(intra.Teams, len(eligibleTeams))
	i := 0
	for _, team := range eligibleTeams {
		res[i] = team
		i++
	}
	fmt.Printf("%d teams retrieved.\n", len(res))
	return
}

func processTeams(teams intra.Teams, expirationDate time.Time) {
	fmt.Printf("Processing...\n\n")
	count := 0
	for _, team := range teams {
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
	fmt.Printf(
		"\nOK: %d (%.2f%%)\tSTAGNANT: %d (%.2f%%)\tTOTAL: %d\n",
		len(teams)-count,
		100*(float64(len(teams)-count)/float64(len(teams))),
		count,
		100*(float64(count)/float64(len(teams))),
		len(teams),
	)
}

func main() {
	log.Println("Gitcreeper started...")
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
	log.Println("Gitcreeper complete!")
}
