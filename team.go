package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type User struct {
	UserID         int    `json:"id"`
	Login          string `json:"login"`
	ProjectsUserID int    `json:"projects_user_id"`
}

type Team struct {
	TeamID     int        `json:"id"`
	ProjectID  int        `json:"project_id"`
	RepoURL    string     `json:"repo_url"`
	Users      []User     `json:"users"`
	lastUpdate *time.Time `json:"-"`
	intraIDs   string     `json:"-"`
	path       string     `json:"-"`
	stagnant   bool       `json:"-"`
}

func (team *Team) getPath() string {
	if team.path == "" {
		team.path = filepath.Join(config.ClonePath, strconv.Itoa(team.TeamID))
	}
	return team.path
}

func (team *Team) getIntraIDs() string {
	if team.intraIDs == "" {
		intraIDs := make([]string, len(team.Users))
		for i := range team.Users {
			intraIDs[i] = team.Users[i].Login
		}
		team.intraIDs = strings.Join(intraIDs, ", ")
	}
	return team.intraIDs
}

func (team *Team) cloneRepo() error {
	var err error
	if _, err = os.Stat(team.getPath()); err != nil && os.IsNotExist(err) {
		cmd := fmt.Sprintf("git clone %s %s", team.RepoURL, team.getPath())
		err = exec.Command("/bin/sh", "-c", cmd).Run()
	}
	return err
}

func (team *Team) deleteClone() {
	if _, err := os.Stat(team.getPath()); err != nil {
		return
	}
	if err := os.RemoveAll(team.getPath()); err != nil {
		log.Println(err)
	}
}

// Checks if most recent commit in master branch is older than expirationDate
func (team *Team) checkStagnant(expirationDate time.Time) error {
	if _, err := os.Stat(team.getPath()); err != nil {
		return err
	}
	cmd := fmt.Sprintf("git -C '%s' log 2>/dev/null | grep 'Date:' | head -n1", team.getPath())
	out, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		return err
	}
	// Empty repository--no commits
	if len(out) == 0 {
		team.stagnant = true
		return nil
	}
	dateStr := strings.Trim(strings.SplitN(string(out), ":", 2)[1], " \n")
	parsed, err := time.Parse(gitTimeFormat, dateStr)
	if err == nil && parsed.Sub(expirationDate) <= 0 {
		team.lastUpdate = &parsed
		team.stagnant = true
	}
	return err
}

func getTeams(client *http.Client, endpoint string) ([]Team, error) {
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("Intra error [Response: %d]", resp.StatusCode))
	}
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var teams []Team
	err = json.Unmarshal(respData, &teams)
	return teams, err
}

func getAllTeams(params map[string]string) ([]Team, error) {
	client := getClient(context.Background(), "public", "projects")
	pageNumber := int64(1)
	if num, ok := params["page[number]"]; ok {
		pageNumber, _ = strconv.ParseInt(num, 10, 64)
	}
	var teams []Team
	for {
		params["page[number]"] = strconv.FormatInt(pageNumber, 10)
		endpoint := getEndpoint("teams", params)
		res, err := getTeams(client, endpoint)
		if err != nil {
			return teams, err
		}
		if len(res) == 0 {
			break
		}
		teams = append(teams, res...)
		pageNumber++
	}
	return teams, nil
}

func getStagnantTeams() []Team {
	var stagnantTeams []Team
	expirationDate := time.Now().Add(- (time.Duration(config.DaysUntilStagnant) * 24 * time.Hour))
	teams, err := getAllTeams(
		map[string]string{
			"filter[primary_campus]": config.CampusID,
			"filter[active_cursus]":  config.CursusID,
			"filter[closed]":         "false",
			"range[created_at]":      config.StartDate + "," + expirationDate.Format(intraTimeFormat),
			"page[size]":             "100",
		},
	)
	if err != nil {
		log.Println(err)
		if len(teams) == 0 {
			return stagnantTeams
		}
	}
	for i := range teams {
		team := &teams[i]
		if !isWhitelisted(team.ProjectID) || team.RepoURL == "" {
			continue
		}
		proj, err := getProject(team.ProjectID)
		if err != nil {
			log.Printf("Error retrieving project info for ID %d: %s", team.ProjectID, err)
			continue
		}
		fmt.Printf("Checking %d <%s> (%s)...\n", team.TeamID, proj.Name, team.getIntraIDs())
		if err = team.cloneRepo(); err == nil {
			err = team.checkStagnant(expirationDate)
			if team.stagnant {
				stagnantTeams = append(stagnantTeams, *team)
			}
			team.deleteClone()
		}
		if err != nil {
			log.Println(err)
		}
	}
	return stagnantTeams
}
