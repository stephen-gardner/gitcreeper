package intra

import (
	"context"
	"encoding/json"
	"net/url"
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

func GetProject(ctx context.Context, bypassCache bool, ID int, project *Project) error {
	IDStr := strconv.Itoa(ID)
	endpoint := getEndpoint("projects/"+IDStr, nil)
	if proj, present := intraCache[endpoint]; !bypassCache && present {
		*project = proj.(Project)
		return nil
	}
	params := url.Values{}
	params.Set("filter[id]", IDStr)
	params.Set("page[number]", "1")
	var projects Projects
	err := GetAllProjects(ctx, params, &projects)
	if err == nil && len(projects) > 0 {
		*project = projects[0]
	}
	return err
}

func GetAllProjects(ctx context.Context, params url.Values, projects *Projects) error {
	data, err := getAll(getClient(ctx, "public"), "projects", params)
	if err != nil {
		return err
	}
	for _, dataPage := range data {
		var page Projects
		if err := json.Unmarshal(dataPage, &page); err != nil {
			return err
		}
		for _, proj := range page {
			intraCache[getEndpoint("projects/"+strconv.Itoa(proj.ID), nil)] = proj
		}
		*projects = append(*projects, page...)
	}
	return nil
}
