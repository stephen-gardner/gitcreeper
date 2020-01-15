package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"gitcreeper/intra"
)

func getIntraIDs(team *intra.Team) []string {
	intraIDs := make([]string, len(team.Users))
	for i := range team.Users {
		intraIDs[i] = team.Users[i].Login
	}
	return intraIDs
}

func getProjectName(team *intra.Team) string {
	project := &intra.Project{}
	err := project.GetProject(context.Background(), false, team.ProjectID)
	if err != nil {
		log.Printf("Error retrieving project info for ID %d: %s\n", team.ProjectID, err)
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
	path := strings.Split(strings.Split(team.RepoURL, ":")[1], "/")
	path[len(path)-1] = team.RepoUUID
	cmd := fmt.Sprintf(
		"ssh %s@%s -p %d -i %s \"git -C %s/%s log 2>/dev/null | grep 'Date:' | head -n1\"",
		config.RepoUser,
		config.RepoAddress,
		config.RepoPort,
		config.RepoPrivateKeyPath,
		config.RepoPath,
		strings.Join(path, "/"),
	)
	out, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		output("ERROR")
		return false, nil, err
	}
	// Empty repository--no commits
	if len(out) == 0 {
		output("STAGNANT [last update: never]")
		return true, nil, nil
	}
	dateStr := strings.Trim(strings.SplitN(string(out), ":", 2)[1], " \n")
	lastUpdate, err := time.Parse(gitTimeFormat, dateStr)
	if err != nil {
		output("ERROR")
		return false, nil, err
	}
	stagnant := lastUpdate.UTC().Sub(expirationDate) <= 0
	if stagnant {
		output("STAGNANT")
	} else {
		output("OK")
	}
	output(" [last update: %s]", lastUpdate)
	return stagnant, &lastUpdate, nil
}
