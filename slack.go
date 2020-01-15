package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var logBuffer strings.Builder

func output(format string, args ...interface{}) {
	out := fmt.Sprintf(format, args...)
	if config.SlackLogging {
		logBuffer.WriteString(out)
	}
	_, _ = os.Stdout.WriteString(out)
}

func postLogs(midnight time.Time) error {
	params := url.Values{}
	params.Set("token", os.Getenv("SLACK_TOKEN"))
	params.Set("channels", config.SlackOutputChannel)
	params.Set("content", logBuffer.String())
	params.Set("title", "GitCreeper Report "+midnight.Local().Format(logTimeFormat))
	resp, err := http.PostForm("https://slack.com/api/files.upload", params)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
