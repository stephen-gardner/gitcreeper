package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/smtp"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type (
	TeamUser struct {
		ID             int    `json:"id"`
		Login          string `json:"login"`
		URL            string `json:"url"`
		Leader         bool   `json:"leader"`
		Occurrence     int    `json:"occurrence"`
		Validated      bool   `json:"validated"`
		ProjectsUserID int    `json:"projects_user_id"`
	}
	Team struct {
		ID               int        `json:"id"`
		Name             string     `json:"name"`
		URL              string     `json:"url"`
		FinalMark        int        `json:"final_mark"`
		ProjectID        int        `json:"project_id"`
		CreatedAt        time.Time  `json:"created_at"`
		UpdatedAt        time.Time  `json:"updated_at"`
		Status           string     `json:"status"`
		TerminatingAt    time.Time  `json:"terminating_at"`
		Users            []TeamUser `json:"users"`
		Locked           bool       `json:"locked?"`
		Validated        bool       `json:"validated?"`
		Closed           bool       `json:"closed?"`
		RepoURL          string     `json:"repo_url"`
		RepoUUID         string     `json:"repo_uuid"`
		LockedAt         time.Time  `json:"locked_at"`
		ClosedAt         time.Time  `json:"closed_at"`
		ProjectSessionID int        `json:"project_session_id"`
	}
	Teams []Team
)

func (team *Team) getIntraIDs() []string {
	intraIDs := make([]string, len(team.Users))
	for i := range team.Users {
		intraIDs[i] = team.Users[i].Login
	}
	return intraIDs
}

func (team *Team) getProject(ctx context.Context, bypassCache bool) (*Project, error) {
	project := &Project{}
	err := getProject(ctx, bypassCache, team.ProjectID, project)
	return project, err
}

// Checks if most recent commit in master branch is older than expirationDate
func (team *Team) checkStagnant(expirationDate time.Time) (bool, *time.Time, error) {
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
		return false, nil, err
	}
	// Empty repository--no commits
	if len(out) == 0 {
		return true, nil, nil
	}
	dateStr := strings.Trim(strings.SplitN(string(out), ":", 2)[1], " \n")
	parsed, err := time.Parse(gitTimeFormat, dateStr)
	return err == nil && parsed.Sub(expirationDate) <= 0, &parsed, err
}

func (team *Team) sendEmail(lastUpdate *time.Time) error {
	to := make([]string, len(team.Users))
	for i := range team.Users {
		to[i] = team.Users[i].Login + "@student.42.us.org"
	}
	project, err := team.getProject(context.Background(), false)
	if err != nil {
		return err
	}
	tmpl, err := template.ParseFiles("templates/email.html")
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
		ProjectName:       project.Name,
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

func getTeam(ctx context.Context, bypassCache bool, ID int, team *Team) error {
	endpoint := getEndpoint("teams/"+strconv.Itoa(ID), map[string]string{})
	if t, present := intraCache[endpoint]; !bypassCache && present {
		*team = t.(Team)
		return nil
	}
	var teams Teams
	err := getAllTeams(ctx, map[string]string{"filter[id]": strconv.Itoa(ID)}, &teams)
	if err == nil {
		*team = teams[0]
	}
	return err
}

func getAllTeams(ctx context.Context, params map[string]string, teams *Teams) error {
	client := getClient(ctx, "public", "projects")
	pageNumber := 1
	if num, ok := params["page[number]"]; ok {
		pageNumber, _ = strconv.Atoi(num)
	}
	for {
		var page []Team
		params["page[number]"] = strconv.Itoa(pageNumber)
		endpoint := getEndpoint("teams", params)
		err := runIntraRequest(client, "GET", endpoint, &page)
		if err != nil {
			return err
		}
		if len(page) == 0 {
			break
		}
		for _, team := range page {
			intraCache[team.URL] = team
		}
		*teams = append(*teams, page...)
		pageNumber++
	}
	return nil
}
