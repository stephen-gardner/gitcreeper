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
		fmt.Printf("STAGNANT")
	} else {
		fmt.Printf("OK")
	}
	fmt.Printf(" [last update: %s]\n", lastUpdate)
	return stagnant, &lastUpdate, nil
}

func composeWarningEmail(body *bytes.Buffer, tmplVars map[string]string) error {
	tmpl, err := template.ParseFiles("templates/warning_email.html")
	if err != nil {
		return err
	}
	err = tmpl.Execute(body, struct {
		From              string
		To                string
		ProjectName       string
		LastCommitDate    string
		TimeElapsed       string
		DaysUntilStagnant int
	}{
		From:              config.EmailFromAddress,
		To:                tmplVars["to"],
		ProjectName:       tmplVars["projectName"],
		LastCommitDate:    tmplVars["lastUpdate"],
		TimeElapsed:       tmplVars["timeElapsed"],
		DaysUntilStagnant: config.DaysUntilStagnant,
	})
	if err != nil {
		return err
	}
	bytes.ReplaceAll(body.Bytes(), []byte("\n"), []byte("\r\n"))
	return nil
}

func sendEmail(team *intra.Team, lastUpdate *time.Time, warn bool) error {
	to := make([]string, len(team.Users))
	for i := range team.Users {
		to[i] = fmt.Sprintf("%s@student.%s", team.Users[i].Login, config.CampusDomain)
	}
	tmplVars := map[string]string{
		"to":          strings.Join(to, ","),
		"projectName": getProjectName(team),
	}
	if lastUpdate != nil {
		tmplVars["lastUpdate"] = lastUpdate.String()
		tmplVars["timeElapsed"] = strconv.Itoa(int(time.Now().Sub(*lastUpdate).Hours()/24)) + " days ago"
	} else {
		tmplVars["lastUpdate"] = "NEVER"
		tmplVars["timeElapsed"] = "never"
	}
	body := &bytes.Buffer{}
	if warn {
		if err := composeWarningEmail(body, tmplVars); err != nil {
			return err
		}
	}
	return smtp.SendMail(config.EmailServerAddress, nil, config.EmailFromAddress, to, body.Bytes())
}
