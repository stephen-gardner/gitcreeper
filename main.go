package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gitcreeper/intra"
)

type Config struct {
	CampusDomain         string
	CampusID             int
	CursusIDs            []int
	ProjectStartingRange string
	DaysUntilStagnant    int
	DaysToCorrect        int
	RepoAddress          string
	RepoPort             int
	RepoUser             string
	RepoPrivateKeyPath   string
	RepoPath             string
	EmailServerAddress   string
	EmailFromAddress     string
	ProjectWhitelist     []int
}

const intraTimeFormat = "2006-01-02T15:04:05.000Z"
const gitTimeFormat = "Mon Jan 2 15:04:05 2006 -0700"

var config Config
var projectWhitelist = make(map[int]bool)

// Return teams that may be stagnant according to config
func getEligibleTeams(expirationDate time.Time) (res intra.Teams) {
	fmt.Printf("Getting eligible teams from 42 Intra... ")
	// Some teams may belong to more than one cursus
	eligibleTeams := make(map[int]*intra.Team)
	for _, cursusID := range config.CursusIDs {
		params := url.Values{}
		params.Set("filter[primary_campus]", strconv.Itoa(config.CampusID))
		params.Set("filter[active_cursus]", strconv.Itoa(cursusID))
		params.Set("filter[closed]", "false")
		params.Set("range[created_at]", config.ProjectStartingRange+","+expirationDate.Format(intraTimeFormat))
		params.Set("page[size]", "100")
		var teams intra.Teams
		if err := intra.GetAllTeams(context.Background(), params, &teams); err != nil {
			log.Println(err)
		}
		// Check if team is on the whitelist and that it has a local repository
		for i := range teams {
			team := &teams[i]
			_, whitelisted := projectWhitelist[team.ProjectID]
			if !whitelisted || !strings.Contains(team.RepoURL, config.CampusDomain) {
				continue
			}
			eligibleTeams[team.ID] = team
		}
	}
	res = make(intra.Teams, len(eligibleTeams))
	i := 0
	for _, team := range eligibleTeams {
		res[i] = *team
		i++
	}
	fmt.Printf("%d teams retrieved.\n", len(res))
	return
}

func processTeams(teams intra.Teams, expirationDate time.Time) {
	fmt.Printf("Processing...\n\n")
	nStagnant := 0
	for _, team := range teams {
		stagnant, lastUpdate, err := checkStagnant(&team, expirationDate)
		if stagnant {
			if err = closeTeam(&team); err == nil {
				err = sendEmail(&team, lastUpdate, false)
			}
			nStagnant++
		}
		if err != nil {
			log.Println(err)
			continue
		}
	}
	fmt.Printf(
		"\nOK: %d (%.2f%%) STAGNANT: %d (%.2f%%) TOTAL: %d\n",
		len(teams)-nStagnant, 100*(float64(len(teams)-nStagnant)/float64(len(teams))),
		nStagnant, 100*(float64(nStagnant)/float64(len(teams))),
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
	expirationDate := time.Now().UTC().Add(- (time.Duration(config.DaysUntilStagnant) * 24 * time.Hour))
	teams := getEligibleTeams(expirationDate)
	processTeams(teams, expirationDate)
	log.Println("Creeping complete!")
}
