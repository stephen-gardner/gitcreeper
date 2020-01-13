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
	"strings"

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

func getEndpoint(path string, params url.Values) string {
	baseURL, err := url.Parse("https://api.intra.42.fr/v2/")
	if err != nil {
		log.Println(err)
		return ""
	}
	baseURL.Path += path
	baseURL.RawQuery = params.Encode()
	return baseURL.String()
}

func runRequest(client *http.Client, method, endpoint string, formData url.Values) (int, []byte, error) {
	req, err := http.NewRequest(method, endpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return 0, nil, err
	}
	if formData != nil {
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		err := errors.New(fmt.Sprintf("Intra error [Response: %d] %s", resp.StatusCode, string(data)))
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, data, err
}

func getAll(client *http.Client, endpoint string, params url.Values) ([][]byte, error) {
	var res [][]byte
	pageNumber := 1
	singlePage := false
	if _, ok := params["page[number]"]; ok {
		pageNumber, _ = strconv.Atoi(params.Get("page[number]"))
		singlePage = true
	}
	for {
		params.Set("page[number]", strconv.Itoa(pageNumber))
		endpoint := getEndpoint(endpoint, params)
		_, page, err := runRequest(client, http.MethodGet, endpoint, nil)
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
