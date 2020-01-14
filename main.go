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
	StartClosingAt       string
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

func processTeams(teams intra.Teams, midnight, expirationDate time.Time, prelaunch bool) {
	fmt.Printf("Processing...\n\n")
	nStagnant := 0
	nWarned := 0
	for _, team := range teams {
		stagnant, lastUpdate, err := checkStagnant(&team, expirationDate)
		if stagnant {
			if prelaunch {
				err = sendEmail(&team, lastUpdate, prelaunchEmail)
			} else {
				if err = closeTeam(&team, midnight); err == nil {
					err = sendEmail(&team, lastUpdate, closedEmail)
				}
			}
			nStagnant++
		} else if !prelaunch && lastUpdate != nil {
			adjusted := lastUpdate.UTC().Add(-(24 * time.Hour))
			if adjusted.Sub(expirationDate) <= 0 {
				fmt.Printf("*")
				err = sendEmail(&team, lastUpdate, warningEmail)
				nWarned++
			}
		}
		fmt.Printf("\n")
		if err != nil {
			log.Println(err)
			continue
		}
	}
	fmt.Printf(
		"\nOK: %d (%.2f%%) WARNED: %d (%.2f%%) STAGNANT: %d (%.2f%%) TOTAL: %d\n",
		len(teams)-nStagnant, 100*(float64(len(teams)-nStagnant)/float64(len(teams))),
		nWarned, 100*(float64(nWarned)/float64(len(teams))),
		nStagnant, 100*(float64(nStagnant)/float64(len(teams))),
		len(teams),
	)
}

// TODO: Slack report

func main() {
	log.Println("GitCreeper started...")
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
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UTC()
	expirationDate := midnight.Add(- (time.Duration(config.DaysUntilStagnant) * 24 * time.Hour))
	startClosingAt, err := time.Parse(intraTimeFormat, config.StartClosingAt)
	if err != nil {
		log.Fatal(err)
	}
	teams := getEligibleTeams(expirationDate)
	processTeams(teams, midnight, expirationDate, midnight.Sub(startClosingAt) < 0)
	log.Println("Creeping complete!")
}
