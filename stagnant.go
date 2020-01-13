package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"net/url"
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
	project := intra.Project{}
	err := intra.GetProject(context.Background(), false, team.ProjectID, &project)
	if err != nil {
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
	stagnant := lastUpdate.UTC().Sub(expirationDate) <= 0
	if stagnant {
		fmt.Printf("STAGNANT")
	} else {
		fmt.Printf("OK")
	}
	fmt.Printf(" [last update: %s]\n", lastUpdate)
	return stagnant, &lastUpdate, nil
}

func composeEmail(tmplPath string, body *bytes.Buffer, vars map[string]string) error {
	tmpl, err := template.ParseFiles(tmplPath)
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
		DaysToCorrect     int
	}{
		From:              config.EmailFromAddress,
		To:                vars["to"],
		ProjectName:       vars["projectName"],
		LastCommitDate:    vars["lastUpdate"],
		TimeElapsed:       vars["timeElapsed"],
		DaysUntilStagnant: config.DaysUntilStagnant,
		DaysToCorrect:     config.DaysToCorrect,
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
	vars := map[string]string{
		"to":          strings.Join(to, ","),
		"projectName": getProjectName(team),
	}
	if lastUpdate != nil {
		vars["lastUpdate"] = lastUpdate.String()
		vars["timeElapsed"] = strconv.Itoa(int(time.Now().Sub(*lastUpdate).Hours()/24)) + " days ago"
	} else {
		vars["lastUpdate"] = "NEVER"
		vars["timeElapsed"] = "never"
	}
	body := &bytes.Buffer{}
	var tmplPath string
	if warn {
		tmplPath = "templates/warning_email.html"
	} else {
		tmplPath = "templates/closed_email.html"
	}
	if err := composeEmail(tmplPath, body, vars); err != nil {
		return err
	}
	return smtp.SendMail(config.EmailServerAddress, nil, config.EmailFromAddress, to, body.Bytes())
}

func closeTeam(team *intra.Team) error {
	patched := *team
	patched.ClosedAt = time.Now().UTC()
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
