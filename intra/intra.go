package intra

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"golang.org/x/oauth2/clientcredentials"
)

var intraCache = make(map[string]interface{})

func getClient(ctx context.Context, scopes ...string) *http.Client {
	oauth := clientcredentials.Config{
		ClientID:     os.Getenv("INTRA_CLIENT_ID"),
		ClientSecret: os.Getenv("INTRA_CLIENT_SECRET"),
		TokenURL:     "https://api.intra.42.fr/oauth/token",
		Scopes:       scopes,
	}
	return oauth.Client(ctx)
}

func getEndpoint(path string, options map[string]string) string {
	baseURL, err := url.Parse("https://api.intra.42.fr/v2/")
	if err != nil {
		log.Println(err)
		return ""
	}
	baseURL.Path += path
	params := url.Values{}
	for key, value := range options {
		params.Add(key, value)
	}
	baseURL.RawQuery = params.Encode()
	return baseURL.String()
}

func runRequest(client *http.Client, method, endpoint string) ([]byte, error) {
	req, err := http.NewRequest(method, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("Intra error [Response: %d]", resp.StatusCode))
	}
	return ioutil.ReadAll(resp.Body)
}

func getAll(client *http.Client, endpoint string, params map[string]string) ([][]byte, error) {
	var res [][]byte
	pageNumber := 1
	singlePage := false
	if num, ok := params["page[number]"]; ok {
		pageNumber, _ = strconv.Atoi(num)
		singlePage = true
	}
	for {
		params["page[number]"] = strconv.Itoa(pageNumber)
		endpoint := getEndpoint(endpoint, params)
		page, err := runRequest(client, "GET", endpoint)
		if err != nil {
			return res, err
		}
		if singlePage {
			return [][]byte{page}, nil
		}
		if string(page) == "[]" {
			break
		}
		res = append(res, page)
		pageNumber++
	}
	return res, nil
}
