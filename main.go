package main

import (
	"context"
	"encoding/json"
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
	gitTimeFormat   = "Mon Jan 2 15:04:05 2006 -0700"
	logTimeFormat   = "2006/01/02 15:04:05"
)

var config Config
var projectWhitelist = make(map[int]bool)

// Return teams that may be stagnant according to config
func getEligibleTeams(expirationDate time.Time) (res intra.Teams) {
	output("Getting eligible teams from 42 Intra... ")
	// Some teams may belong to more than one cursus
	eligibleTeams := make(map[int]bool)
	lockedRange := config.ProjectStartingRange.Format(intraTimeFormat) + "," + expirationDate.Format(intraTimeFormat)
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
			sentry.CaptureException(err)
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
	nStagnant := 0
	nWarned := 0
	for i := range teams {
		team := &teams[i]
		stagnant, lastUpdate, err := checkStagnant(team, expirationDate)
		if stagnant {
			if prelaunch {
				err = sendEmail(team, lastUpdate, prelaunchEmail)
			} else {
				if err = closeTeam(team, midnight); err == nil {
					err = sendEmail(team, lastUpdate, closedEmail)
				}
			}
			nStagnant++
		} else if !prelaunch {
			// Check for teams that will stagnate within 24 hours
			var adjusted time.Time
			if lastUpdate == nil {
				adjusted = team.LockedAt
			} else {
				adjusted = lastUpdate.UTC()
			}
			adjusted = adjusted.Add(-(24 * time.Hour))
			if adjusted.Sub(expirationDate) <= 0 {
				output("*")
				err = sendEmail(team, lastUpdate, warningEmail)
				nWarned++
			}
		}
		if lastUpdate != nil && lastUpdate.UTC().Sub(midnight) > 0 {
			output(" CHEAT")
		}
		output("\n")
		if err != nil {
			sentry.CaptureException(err)
			continue
		}
	}
	output(
		"\nOK: %d (%.2f%%) WARNED: %d (%.2f%%) STAGNANT: %d (%.2f%%) TOTAL: %d\n",
		len(teams)-nStagnant, 100*(float64(len(teams)-nStagnant)/float64(len(teams))),
		nWarned, 100*(float64(nWarned)/float64(len(teams))),
		nStagnant, 100*(float64(nStagnant)/float64(len(teams))),
		len(teams),
	)
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
		sentry.CaptureException(err)
		sentry.Flush(5 * time.Second)
		return
	}
	if err := sshConnect(); err != nil {
		sentry.CaptureException(err)
		sentry.Flush(5 * time.Second)
		return
	}
	defer conn.Close()
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UTC()
	expirationDate := midnight.Add(- (time.Duration(config.DaysUntilStagnant) * 24 * time.Hour))
	teams := getEligibleTeams(expirationDate)
	processTeams(teams, midnight, expirationDate, midnight.Sub(config.StartClosingAt) < 0)
	output("%s Creeping complete!\n", time.Now().Format(logTimeFormat))
	if config.SlackLogging {
		if err := postLogs(midnight); err != nil {
			sentry.CaptureException(err)
		}
	}
	sentry.Flush(5 * time.Second)
}
