package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

var logBuffer strings.Builder

func output(format string, args ...interface{}) {
	out := fmt.Sprintf(format, args...)
	if config.SlackLogging {
		logBuffer.WriteString(out)
	}
	_, _ = os.Stdout.WriteString(strings.ReplaceAll(out, "\t", " "))
}

func getFormattedOutput() string {
	buff := &strings.Builder{}
	tw := tabwriter.NewWriter(buff, 0, 8, 1, '\t', tabwriter.AlignRight)
	lines := strings.Split(logBuffer.String(), "\n")
	header := true
	for _, line := range lines {
		if !strings.HasPrefix(line, "Checking") {
			_, _ = fmt.Fprintf(tw, "%s\n", line)
			continue
		}
		if header {
			_, _ = tw.Write([]byte("TEAM ID\tPROJECT\tLOGIN\tSTATUS\tLAST COMMIT\n"))
			_, _ = tw.Write([]byte("=======\t=======\t=====\t======\t===========\n"))
			header = false
			continue
		}
		cols := strings.Split(line, "\t")
		cols[1] = cols[1][1 : len(cols[1])-1]
		cols[3] = cols[3][1 : len(cols[3])-4]
		cols[5] = cols[5][14 : len(cols[5])-1]
		_, _ = fmt.Fprintf(tw, "%s\n", strings.Join(cols[1:], "\t"))
	}
	_ = tw.Flush()
	out := buff.String()
	return out[:len(out)-1]
}

func postLogs(midnight time.Time) error {
	params := url.Values{}
	params.Set("token", os.Getenv("SLACK_TOKEN"))
	params.Set("channels", config.SlackOutputChannel)
	params.Set("content", getFormattedOutput())
	params.Set("title", "GitCreeper Report "+midnight.Local().Format(logTimeFormat))
	resp, err := http.PostForm("https://slack.com/api/files.upload", params)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
