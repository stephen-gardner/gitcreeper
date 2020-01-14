package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"gitcreeper/intra"
)

const (
	prelaunchEmail = "prelaunch"
	warningEmail   = "warning"
	closedEmail    = "closed"
)

func composeEmail(emailType string, body *bytes.Buffer, vars map[string]string) error {
	tmpl, err := template.ParseFiles(
		"templates/email.html",
		fmt.Sprintf("templates/%s.html", emailType),
	)
	if err != nil {
		return err
	}
	var title string
	if emailType == warningEmail {
		title = "%s Nearing Update Deadline"
	} else {
		title = "Insufficient Progress on %s"
	}
	err = tmpl.Execute(body, struct {
		From              string
		To                string
		Title             string
		ProjectName       string
		LastCommitDate    string
		TimeElapsed       string
		DaysUntilStagnant int
		DaysToCorrect     int
	}{
		From:              config.EmailFromAddress,
		To:                vars["to"],
		Title:             fmt.Sprintf(title, vars["projectName"]),
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

func sendEmail(team *intra.Team, lastUpdate *time.Time, emailType string) error {
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
	if err := composeEmail(emailType, body, vars); err != nil {
		return err
	}
	return smtp.SendMail(config.EmailServerAddress, nil, config.EmailFromAddress, to, body.Bytes())
}
