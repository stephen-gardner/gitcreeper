package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"os/exec"
	"strconv"
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
	project, err := intra.GetProject(context.Background(), false, team.ProjectID)
	if err != nil || project == nil {
		log.Printf("Error retrieving project info for ID %d: %s\n", team.ProjectID, err)
		return "Unknown Project"
	} else {
		return project.Name
	}
}

// Checks if most recent commit in master branch is older than expirationDate
func checkStagnant(team *intra.Team, expirationDate time.Time) (bool, *time.Time, error) {
	fmt.Printf(
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
		fmt.Printf("ERROR\n")
		return false, nil, err
	}
	// Empty repository--no commits
	if len(out) == 0 {
		fmt.Printf("STAGNANT [last update: never]\n")
		return true, nil, nil
	}
	dateStr := strings.Trim(strings.SplitN(string(out), ":", 2)[1], " \n")
	lastUpdate, err := time.Parse(gitTimeFormat, dateStr)
	if err != nil {
		fmt.Printf("ERROR\n")
		return false, nil, err
	}
	stagnant := lastUpdate.Sub(expirationDate) <= 0
	if stagnant {
		fmt.Printf("STAGNANT ")
	} else {
		fmt.Printf("OK ")
	}
	fmt.Printf("[last update: %s]\n", lastUpdate)
	return stagnant, &lastUpdate, nil
}

func sendEmail(team *intra.Team, lastUpdate *time.Time) error {
	to := make([]string, len(team.Users))
	for i := range team.Users {
		to[i] = team.Users[i].Login + "@student.42.us.org"
	}
	tmpl, err := template.ParseFiles("templates/warning_email.html")
	if err != nil {
		return err
	}
	var body bytes.Buffer
	var lastUpdateStr string
	var timeElapsed string
	if lastUpdate != nil {
		lastUpdateStr = lastUpdate.String()
		timeElapsed = strconv.Itoa(int(time.Now().Sub(*lastUpdate).Hours()/24)) + " days ago"
	} else {
		lastUpdateStr = "NEVER"
		timeElapsed = "never"
	}
	err = tmpl.Execute(&body, struct {
		From              string
		To                string
		ProjectName       string
		LastCommitDate    string
		TimeElapsed       string
		DaysUntilStagnant int
	}{
		From:              config.EmailFromAddress,
		To:                strings.Join(to, ","),
		ProjectName:       getProjectName(team),
		LastCommitDate:    lastUpdateStr,
		TimeElapsed:       timeElapsed,
		DaysUntilStagnant: config.DaysUntilStagnant,
	})
	if err != nil {
		return err
	}
	bytes.ReplaceAll(body.Bytes(), []byte("\n"), []byte("\r\n"))
	err = smtp.SendMail(config.EmailServerAddress, nil, config.EmailFromAddress, to, body.Bytes())
	return err
}
