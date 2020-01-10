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
)

type Config struct {
	IntraClientID      string
	IntraClientSecret  string
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

func getStagnantTeams() {
	fmt.Println("Getting eligible teams from 42 Intra...")
	cursus := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(config.CursusIDs)), ","), "[]")
	expirationDate := time.Now().Add(- (time.Duration(config.DaysUntilStagnant) * 24 * time.Hour))
	var teams Teams
	if err := getAllTeams(
		context.Background(),
		map[string]string{
			"filter[primary_campus]": strconv.Itoa(config.CampusID),
			"filter[active_cursus]":  cursus,
			"filter[closed]":         "false",
			"range[created_at]":      config.StartDate + "," + expirationDate.Format(intraTimeFormat),
			"page[size]":             "100",
		},
		&teams,
	); err != nil {
		log.Println(err)
		if len(teams) == 0 {
			return
		}
	}
	count := 0
	for _, team := range teams {
		_, whitelisted := projectWhitelist[team.ProjectID]
		if !whitelisted || !strings.Contains(team.RepoURL, config.RepoDomain) {
			continue
		}
		proj, err := team.getProject(context.Background(), false)
		if err != nil {
			log.Printf("Error retrieving project info for ID %d: %s\n", team.ProjectID, err)
			continue
		}
		fmt.Printf(
			"Checking <%d> %s (%s)... ",
			team.ID,
			proj.Name,
			strings.Join(team.getIntraIDs(), ", "),
		)
		stagnant, lastUpdate, err := team.checkStagnant(expirationDate)
		if err == nil {
			if stagnant {
				fmt.Printf("STAGNANT")
				count++
				//err = team.sendEmail(lastUpdate)
			} else {
				fmt.Printf("OK")
			}
			lastUpdateStr := "never"
			if lastUpdate != nil {
				lastUpdateStr = lastUpdate.String()
			}
			fmt.Printf(" [last update: %s]\n", lastUpdateStr)
		}
		if err != nil {
			log.Println(err)
		}
	}
	fmt.Println(count, "teams have stagnated")
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
	getStagnantTeams()
}
