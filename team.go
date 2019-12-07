package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func getRepoClonePath(teamID int) string {
	return filepath.Join(config.ClonePath, strconv.Itoa(teamID))
}

func (team *Team) cloneRepo() error {
	var err error
	clonePath := getRepoClonePath(team.TeamID)
	if _, err = os.Stat(clonePath); err != nil && os.IsNotExist(err) {
		cmd := fmt.Sprintf("git clone %s %s", team.RepoURL, clonePath)
		err = exec.Command("/bin/sh", "-c", cmd).Run()
	}
	return err
}

func (team *Team) deleteClone() {
	clonePath := getRepoClonePath(team.TeamID)
	if _, err := os.Stat(clonePath); err != nil {
		return
	}
	if err := os.RemoveAll(clonePath); err != nil {
		log.Println(err)
	}
}

func (team *Team) getIntraIDs() []string {
	intraIDs := make([]string, len(team.Users))
	for i := range team.Users {
		intraIDs[i] = team.Users[i].Login
	}
	return intraIDs
}

func (team *Team) getProject() (*Project, error) {
	return getProject(team.ProjectID)
}

// Checks if most recent commit in master branch is older than expirationDate
func (team *Team) isStagnant(expirationDate time.Time) (bool, error) {
	clonePath := getRepoClonePath(team.TeamID)
	if _, err := os.Stat(clonePath); err != nil {
		return false, err
	}
	cmd := fmt.Sprintf("git -C '%s' log 2>/dev/null | grep 'Date:' | head -n1", clonePath)
	out, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		return false, err
	}
	// Empty repository--no commits
	if len(out) == 0 {
		return true, nil
	}
	dateStr := strings.Trim(strings.SplitN(string(out), ":", 2)[1], " \n")
	parsed, err := time.Parse(gitTimeFormat, dateStr)
	if err == nil && parsed.Sub(expirationDate) <= 0 {
		team.lastUpdate = &parsed
		return true, nil
	}
	return false, err
}

func (team *Team) sendEmail() error {
	to := make([]string, len(team.Users))
	for i := range team.Users {
		to[i] = team.Users[i].Login + "@student.42.us.org"
	}
	project, err := team.getProject()
	if err != nil {
		return err
	}
	tmpl, err := template.ParseFiles("templates/email.html")
	if err != nil {
		return err
	}
	var body bytes.Buffer
	var lastUpdate string
	var timeElapsed string
	if team.lastUpdate != nil {
		lastUpdate = team.lastUpdate.String()
		timeElapsed = strconv.Itoa(int(time.Now().Sub(*team.lastUpdate).Hours()/24)) + " days ago"
	} else {
		lastUpdate = "NEVER"
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
		ProjectName:       project.Name,
		LastCommitDate:    lastUpdate,
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
