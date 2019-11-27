package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
)

type Project struct {
	ProjectID int    `json:"id"`
	Name      string `json:"name"`
	Exam      bool   `json:"exam"`
}

var projects = make(map[int]*Project)

func getProject(ID int) (*Project, error) {
	if p, ok := projects[ID]; ok {
		return p, nil
	}
	client := getClient(context.Background(), "public", "projects")
	endpoint := getEndpoint("projects/"+strconv.Itoa(ID), map[string]string{})
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
	p := &Project{}
	err = json.Unmarshal(respData, &p)
	if err == nil {
		projects[ID] = p
	}
	return p, err
}
