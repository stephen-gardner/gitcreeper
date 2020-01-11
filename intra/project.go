package intra

import (
	"context"
	"encoding/json"
	"strconv"
)

type (
	Project struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
		Exam        bool   `json:"exam"`
	}
	Projects []Project
)

func GetProject(ctx context.Context, bypassCache bool, ID int) (*Project, error) {
	IDStr := strconv.Itoa(ID)
	endpoint := getEndpoint("projects/"+IDStr, nil)
	if p, present := intraCache[endpoint]; !bypassCache && present {
		proj := p.(Project)
		return &proj, nil
	}
	projects, err := GetAllProjects(ctx, map[string]string{"filter[id]": IDStr})
	if err == nil && len(projects) > 0 {
		return &projects[0], nil
	}
	return nil, err
}

func GetAllProjects(ctx context.Context, params map[string]string) (Projects, error) {
	data, err := getAll(getClient(ctx, "public"), "projects", params)
	if err != nil {
		return nil, err
	}
	var projects Projects
	for _, dataPage := range data {
		var page Projects
		if err := json.Unmarshal(dataPage, &page); err != nil {
			return nil, err
		}
		for _, proj := range page {
			intraCache[getEndpoint("projects/"+strconv.Itoa(proj.ID), nil)] = proj
		}
		projects = append(projects, page...)
	}
	return projects, nil
}
