package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gitcreeper/intra"

	"github.com/getsentry/sentry-go"
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
		sentry.CaptureException(err)
		return "Unknown Project"
	} else {
		return project.Name
	}
}

// Checks if most recent commit in master branch is older than expirationDate
func checkStagnant(team *intra.Team, expirationDate time.Time) (bool, *time.Time, error) {
	output(
		"Checking <%d> %s (%s)... ",
		team.ID,
		getProjectName(team),
		strings.Join(getIntraIDs(team), ", "),
	)
	lastUpdate, err := getLastUpdate(team)
	if err != nil {
		output("ERROR")
		return false, nil, err
	}
	var stagnant bool
	var lastUpdateStr string
	if lastUpdate == nil {
		stagnant = team.LockedAt.Sub(expirationDate) <= 0
		lastUpdateStr = "never"
	} else {
		stagnant = lastUpdate.Sub(expirationDate) <= 0
		lastUpdateStr = lastUpdate.Local().String()
	}
	status := "OK"
	if stagnant {
		status = "STAGNANT"
	}
	output("%s [last update: %s]", status, lastUpdateStr)
	return stagnant, lastUpdate, nil
}
