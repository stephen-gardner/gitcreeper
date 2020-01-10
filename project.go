package main

import (
	"context"
	"strconv"
)

type (
	Project struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		Exam bool   `json:"exam"`
	}
	Projects []Project
)

func getProjectEndpoint(ID int, params map[string]string) string {
	return getEndpoint("projects/"+strconv.Itoa(ID), params)
}

func getProject(ctx context.Context, bypassCache bool, ID int, project *Project) error {
	endpoint := getProjectEndpoint(ID, map[string]string{})
	if proj, present := intraCache[endpoint]; !bypassCache && present {
		*project = proj.(Project)
		return nil
	}
	client := getClient(ctx, "public", "projects")
	err := runIntraRequest(client, "GET", endpoint, project)
	if err == nil {
		intraCache[endpoint] = *project
	}
	return err
}
