package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gitcreeper/intra"

	"github.com/getsentry/sentry-go"
)

type Config struct {
	CampusDomain         string
	CampusID             int
	CursusIDs            []int
	StartClosingAt       time.Time
	ProjectStartingRange time.Time
	DaysUntilStagnant    int
	DaysToCorrect        int
	AllowVacations       bool
	VacationsEndpoint    string
	RepoAddress          string
	RepoPort             int
	RepoUser             string
	RepoPrivateKeyPath   string
	RepoPath             string
	EmailServerAddress   string
	EmailFromAddress     string
	SlackLogging         bool
	SlackOutputChannel   string
	ProjectWhitelist     []int
}

const (
	intraTimeFormat = "2006-01-02T15:04:05.000Z"
	logTimeFormat   = "2006/01/02 15:04:05"
)

var config Config
var projectWhitelist = make(map[int]bool)

// Return teams that may be stagnant according to config
func getEligibleTeams(expirationDate time.Time) (res intra.Teams) {
	output("Getting eligible teams from 42 Intra... ")
	// Some teams may belong to more than one cursus
	eligibleTeams := make(map[int]bool)
	// Need to get teams one day younger than the expirationDate to send warnings to those with empty repositories
	lockedRange := fmt.Sprintf(
		"%s,%s",
		config.ProjectStartingRange.Format(intraTimeFormat),
		expirationDate.Add(24*time.Hour).Format(intraTimeFormat),
	)
	for _, cursusID := range config.CursusIDs {
		params := url.Values{}
		params.Set("filter[primary_campus]", strconv.Itoa(config.CampusID))
		params.Set("filter[active_cursus]", strconv.Itoa(cursusID))
		params.Set("filter[closed]", "false")
		params.Set("range[locked_at]", lockedRange)
		params.Set("sort", "project_id")
		params.Set("page[size]", "100")
		teams := &intra.Teams{}
		if err := teams.GetAllTeams(context.Background(), params); err != nil {
			outputErr(err, false)
		}
		// Check if team is on the whitelist and that it has a local repository
		for _, team := range *teams {
			_, present := eligibleTeams[team.ID]
			_, whitelisted := projectWhitelist[team.ProjectID]
			if present || !whitelisted || !strings.Contains(team.RepoURL, config.CampusDomain) {
				continue
			}
			res = append(res, team)
			eligibleTeams[team.ID] = true
		}
	}
	output("%d teams retrieved.\n", len(res))
	return
}

func closeTeam(team *intra.Team, midnight time.Time) error {
	patched := *team
	patched.ClosedAt = midnight
	patched.TerminatingAt = patched.ClosedAt.Add(time.Duration(config.DaysToCorrect) * 24 * time.Hour)
	params := url.Values{}
	params.Set("team[closed_at]", patched.ClosedAt.Format(intraTimeFormat))
	params.Set("team[terminating_at]", patched.TerminatingAt.Format(intraTimeFormat))
	_, _, err := patched.PatchTeam(context.Background(), params, true)
	if err != nil {
		return err
	}
	*team = patched
	return nil
}

func processTeams(teams intra.Teams, midnight, expirationDate time.Time, prelaunch bool) {
	output("Processing...\n\n")
	ok, nStagnant, nWarned, nCheat := 0, 0, 0, 0
	for i := range teams {
		team := &teams[i]
		status, lastUpdate, err := checkStagnant(team, midnight, expirationDate)
		switch status {
		case STAGNANT:
			if prelaunch {
				err = sendEmail(team, lastUpdate, prelaunchEmail)
			} else if err = closeTeam(team, midnight); err == nil {
				err = sendEmail(team, lastUpdate, closedEmail)
			}
			nStagnant++
		case WARNED:
			if prelaunch {
				break
			}
			err = sendEmail(team, lastUpdate, warningEmail)
			nWarned++
		case CHEAT:
			nCheat++
		case OK:
			ok++
		}
		if err != nil {
			outputErr(err, false)
			continue
		}
	}
	output("\n")
	stats := []struct {
		label string
		count int
	}{
		{"OK", ok},
		{"WARNED", nWarned},
		{"STAGNANT", nStagnant},
		{"CHEAT", nCheat},
	}
	for _, stat := range stats {
		output("%8s %4d (%.2f%%)\n", stat.label, stat.count, 100*float64(stat.count)/float64(len(teams)))
	}
	output("\n")
}

func loadConfig(path string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	for _, ID := range config.ProjectWhitelist {
		projectWhitelist[ID] = true
	}
	output("%s GitCreeper started...\n", time.Now().Format(logTimeFormat))
	return nil
}

func main() {
	if err := loadConfig("config.json"); err != nil {
		outputErr(err, true)
	}
	if err := sshConnect(); err != nil {
		outputErr(err, true)
	}
	defer sshConn.Close()
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UTC()
	expirationDate := midnight.Add(-time.Duration(config.DaysUntilStagnant) * 24 * time.Hour)
	teams := getEligibleTeams(expirationDate)
	processTeams(teams, midnight, expirationDate, midnight.Sub(config.StartClosingAt) < 0)
	output("%s Creeping complete!\n", time.Now().Format(logTimeFormat))
	if config.SlackLogging {
		if err := postLogs(midnight); err != nil {
			outputErr(err, false)
		}
	}
	sentry.Flush(5 * time.Second)
}
