package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gitcreeper/intra"
)

const (
	CHEAT         = "CHEAT"
	OK            = "OK"
	STAGNANT      = "STAGNANT"
	WARNED        = "WARNED"
	gitTimeFormat = "Mon Jan 2 15:04:05 2006 -0700"
)

func getIntraIDs(team *intra.Team) []string {
	intraIDs := make([]string, len(team.Users))
	for i := range team.Users {
		intraIDs[i] = team.Users[i].Login
	}
	return intraIDs
}

func getLastUpdate(team *intra.Team) (*time.Time, error) {
	path := strings.Split(strings.Split(team.RepoURL, ":")[1], "/")
	path[len(path)-1] = team.RepoUUID
	cmd := fmt.Sprintf(
		"git -C %s/%s log | grep 'Date:' | head -n1",
		config.RepoPath,
		strings.Join(path, "/"),
	)
	out, err := sshRunCommand(cmd)
	if err != nil {
		return nil, err
	}
	// Repository is empty
	if len(out) == 0 {
		return nil, nil
	}
	dateStr := strings.Trim(strings.SplitN(string(out), ":", 2)[1], " \n")
	parsed, err := time.Parse(gitTimeFormat, dateStr)
	if err != nil {
		return nil, err
	}
	lastUpdate := parsed.UTC()
	return &lastUpdate, nil
}

func getProjectName(team *intra.Team) string {
	project := &intra.Project{}
	err := project.GetProject(context.Background(), false, team.ProjectID)
	if err != nil {
		outputErr(err, false)
		return "Unknown Project"
	} else {
		return project.Name
	}
}

// Checks if most recent commit in master branch is older than expirationDate
func checkStagnant(team *intra.Team, midnight, expirationDate time.Time) (string, *time.Time, error) {
	output(
		"Checking\t<%d>\t%s\t(%s)...\t",
		team.ID,
		getProjectName(team),
		strings.Join(getIntraIDs(team), ", "),
	)
	lastUpdate, err := getLastUpdate(team)
	if err != nil {
		output("ERROR\n")
		return "", nil, err
	}
	vacationTime := time.Duration(0)
	if config.AllowVacations {
		vacationTime = calcVacationTime(team, lastUpdate, midnight)
		expirationDate = expirationDate.Add(-vacationTime)
	}
	var last time.Time
	var lastUpdateStr string
	if lastUpdate == nil {
		last = team.LockedAt
		lastUpdateStr = "Never"
	} else {
		last = *lastUpdate
		lastUpdateStr = lastUpdate.Local().Format(time.RFC1123)
	}
	var status string
	if last.Sub(expirationDate) <= 0 {
		status = STAGNANT
	} else if last.Add(-24*time.Hour).Sub(expirationDate) <= 0 {
		status = WARNED
	} else if last.Sub(time.Now().UTC()) > 0 {
		status = CHEAT
	} else {
		status = OK
	}
	output("%s\t[Last update: %s", status, lastUpdateStr)
	if vacationTime != 0 {
		output(" + %.1f vacation days", vacationTime.Hours()/24.0)
	}
	output("]\n")
	return status, lastUpdate, nil
}
