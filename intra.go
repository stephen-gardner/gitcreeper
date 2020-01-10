package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2/clientcredentials"
)

var intraCache = make(map[string]interface{})
var rateLimit = time.Tick(time.Millisecond * 750)

func getClient(ctx context.Context, scopes ...string) *http.Client {
	oauth := clientcredentials.Config{
		ClientID:     config.IntraClientID,
		ClientSecret: config.IntraClientSecret,
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

func runIntraRequest(client *http.Client, method, endpoint string, obj interface{}) error {
	//<-rateLimit
	req, err := http.NewRequest(method, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("Intra error [Response: %d]", resp.StatusCode))
	}
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(respData, obj)
	return err
}
