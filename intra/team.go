package intra

import (
	"context"
	"encoding/json"
	"strconv"
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

func GetTeam(ctx context.Context, bypassCache bool, ID int) (*Team, error) {
	endpoint := getEndpoint("teams/"+strconv.Itoa(ID), map[string]string{})
	if t, present := intraCache[endpoint]; !bypassCache && present {
		team := t.(Team)
		return &team, nil
	}
	teams, err := GetAllTeams(ctx, map[string]string{"filter[id]": strconv.Itoa(ID)})
	if err != nil {
		return nil, err
	}
	return &teams[0], nil
}

func GetAllTeams(ctx context.Context, params map[string]string) (Teams, error) {
	data, err := getAll(getClient(ctx, "public"), "teams", params)
	if err != nil {
		return nil, err
	}
	var teams Teams
	for _, dataPage := range data {
		var page Teams
		if err := json.Unmarshal(dataPage, &page); err != nil {
			return nil, err
		}
		for _, team := range page {
			intraCache[team.URL] = team
		}
		teams = append(teams, page...)
	}
	return teams, nil
}