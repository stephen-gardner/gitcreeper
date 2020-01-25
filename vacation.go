package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gitcreeper/intra"
)

type (
	VacationTime struct {
		time.Time
	}
	Vacation struct {
		Start VacationTime `json:"start_time"`
		End   VacationTime `json:"end_time"`
	}
)

const portalVacationFormat = "2006-01-02"

func (vt *VacationTime) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(string(data), "\"")
	if raw == "null" {
		vt.Time = time.Time{}.UTC()
		return nil
	}
	date, err := time.ParseInLocation(portalVacationFormat, raw, time.Local)
	if err == nil {
		vt.Time = date.UTC()
	}
	return err
}

func maxTime(t1, t2 time.Time) time.Time {
	if t1.Sub(t2) < 0 {
		return t2
	}
	return t1
}

func (vacation Vacation) countApplicableDays(lastUpdate, midnight time.Time) int {
	// Count only overlapping date ranges
	if vacation.End.Sub(lastUpdate) < 0 || midnight.Sub(vacation.Start.Time) < 0 {
		return 0
	}
	last := maxTime(lastUpdate, vacation.Start.Time)
	expiry := maxTime(midnight, vacation.End.Time)
	// Portal vacation date ranges are inclusive
	return 1 + int(math.Ceil(expiry.Sub(last).Hours()/24.0))
}

func getVacations(login string) ([]Vacation, error) {
	URL, err := url.Parse(config.VacationsEndpoint)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Set("token", os.Getenv("PORTAL_TOKEN"))
	params.Set("login", login)
	URL.RawQuery = params.Encode()
	resp, err := http.Get(URL.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		err := errors.New(fmt.Sprintf("Portal error [Response: %d] %s", resp.StatusCode, string(data)))
		return nil, err
	}
	var vacations []Vacation
	err = json.Unmarshal(data, &vacations)
	return vacations, err
}

// Return extra vacation time to extend project expiration date
// Vacation time is averaged from applicable vacation days for all team members
func calcVacationTime(team *intra.Team, lastUpdate *time.Time, midnight time.Time) time.Duration {
	var last time.Time
	if lastUpdate == nil {
		last = team.LockedAt
	} else {
		// Update day can't be part of the vacation window
		last = lastUpdate.Add(24 * time.Hour)
	}
	days := 0
	for _, user := range team.Users {
		vacations, err := getVacations(user.Login)
		if err != nil {
			outputErr(err, false)
			continue
		}
		for _, v := range vacations {
			days += v.countApplicableDays(last, midnight)
		}
	}
	return (time.Duration(days) * 24 * time.Hour) / time.Duration(len(team.Users))
}
