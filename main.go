package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/publicsuffix"
)

// <bitbar.title>IcePortal</bitbar.title>
// <bitbar.version>v1.0.0</bitbar.version>
// <bitbar.author>Boris Petersen</bitbar.author>
// <bitbar.author.github>transacid</bitbar.author.github>
// <bitbar.desc>Displays the next stop of the ICE.</bitbar.desc>
// <bitbar.image>https://raw.githubusercontent.com/transacid/sunbar/main/screenshot.png</bitbar.image>
// <bitbar.dependencies>go</bitbar.dependencies>
// <bitbar.abouturl>https://github.com/transacid/sunbar</bitbar.abouturl>

// set true when testing local against files
var local = false

var (
	dbWiFis            []string = []string{"WIFI@DB", "WIFIonICE"}
	iceportalStatusURL          = "https://iceportal.de/api1/rs/status"
	iceportalTripURL            = "https://iceportal.de/api1/rs/tripInfo/trip"
)

// var iceportalLoginCheckUrl = "https://login.wifionice.de/cna/wifi/user_info"
// var iceportalLoginUrl = "https://login.wifionice.de/cna/logon"

func main() {
	c := newClient(local)
	if !c.detectWiFi() {
		fmt.Println("")
		os.Exit(0)
	}
	c.getStatus()
	c.getTrip()
	fmt.Println(c.outputBuilder())
}

func newClient(local bool) iceportalClient {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		panic(err)
	}
	client := &http.Client{
		Jar: jar,
	}
	return iceportalClient{client: client, local: local}
}

func (c *iceportalClient) getStatus() error {
	body, err := c.fetch("testdata/status.json", iceportalStatusURL)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, &c.status)
}

func (c *iceportalClient) getTrip() error {
	body, err := c.fetch("testdata/trip.json", iceportalTripURL)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, &c.trip)
}

func (c *iceportalClient) fetch(localPath, url string) ([]byte, error) {
	if c.local {
		return os.ReadFile(localPath)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c iceportalClient) calculateArrival() string {
	currentNextStop := c.trip.Trip.StopInfo.ActualNext
	for _, stop := range c.trip.Trip.Stops {
		if stop.Station.EvaNr == currentNextStop && stop.Timetable.ActualArrivalTime != nil {
			arrivalTime := time.UnixMilli(int64(stop.Timetable.ActualArrivalTime.(float64))).Local()
			return time.Until(arrivalTime).Round(time.Minute).String()
		}
	}
	return ""
}

func (c iceportalClient) outputBuilder() string {
	if len(c.trip.Trip.Stops) == 0 {
		return "No trip data available"
	}

	var class string
	if c.status.WagonClass == "FIRST" {
		class = "1st class"
	} else {
		class = "2nd class"
	}

	startingStation := c.trip.Trip.Stops[0].Station.Name
	destinationStation := c.trip.Trip.StopInfo.FinalStationName
	trainName := fmt.Sprintf("%s%s", c.trip.Trip.TrainType, c.trip.Trip.Vzn)
	series := fmt.Sprintf("Series %s / %s", c.status.Series, c.status.Tzn)
	speed := fmt.Sprintf("%.0f km/h", c.status.Speed)
	stops, delayReasons := c.getStops()
	if len(stops) == 0 {
		return "No trip data available"
	}
	nextStop := stops[0]
	if arrival := c.calculateArrival(); arrival != "" {
		re := regexp.MustCompile(`^(.*)(at [0-9]{2}:[0-9]{2})(\ -\ [0-9]{2}:[0-9]{2})$`)
		sub := fmt.Sprintf("${1} in %s ${2}", arrival)
		nextStop = re.ReplaceAllString(nextStop, sub)
	}
	wifi := c.getWifiStatus()
	delayReason := strings.Join(delayReasons, "\n")
	out := fmt.Sprintf(":train.side.front.car: %s\n---\n***%s***|md=true\n%s → %s\n%s\n%s\n%s\n---\n**Next Stop:**|md=true\n%s\n---\n**Wifi:**|md=true\n%s", nextStop, trainName, startingStation, destinationStation, series, class, speed, strings.Join(stops, "\n"), wifi)
	if delayReason != "" {
		out += fmt.Sprintf("\n**Delay Reasons:**|md=true\n%s", delayReason)
	}
	return out
}

func (c iceportalClient) getWifiStatus() string {
	wifiCurrentStatus := strings.ToLower(c.status.Connectivity.CurrentState)
	wifiNextStatus := strings.ToLower(c.status.Connectivity.NextState)
	wifiStatusRemainingSecondsRawStr := c.status.Connectivity.RemainingTimeSeconds
	wifiStatusRemainingSecondsRaw := strconv.Itoa(wifiStatusRemainingSecondsRawStr)
	out := fmt.Sprintf("Quality: %s\nChanges to %s", wifiCurrentStatus, wifiNextStatus)
	if wifiRemainingString, err := time.ParseDuration(wifiStatusRemainingSecondsRaw + "s"); err == nil {
		out += fmt.Sprintf(" in %s", wifiRemainingString)
	}
	return out
}

func (c iceportalClient) getStops() ([]string, []string) {
	var stopsSlice []string
	var delayResons []string

	currentNextStop := c.trip.Trip.StopInfo.ActualNext
	for _, stop := range c.trip.Trip.Stops {
		if stop.Info.Passed && stop.Info.PositionStatus != "arrived" {
			continue
		}
		isCurrentStop := stop.Station.EvaNr == currentNextStop
		stopStationName := stop.Station.Name
		var arrivalActual string
		if stop.Timetable.ActualArrivalTime != nil {
			arrivalActual = time.UnixMilli(int64(stop.Timetable.ActualArrivalTime.(float64))).Local().Format("15:04")
		} else if stop.Timetable.ScheduledArrivalTime != nil {
			arrivalActual = time.UnixMilli(int64(stop.Timetable.ScheduledArrivalTime.(float64))).Local().Format("15:04")
		} else {
			arrivalActual = "Unkmown"
			if isCurrentStop {
				if stop.Timetable.ActualDepartureTime != nil {
					arrivalActual = time.UnixMilli(int64(stop.Timetable.ActualDepartureTime.(float64))).Local().Format("15:04")
				} else if stop.Timetable.ScheduledDepartureTime != nil {
					arrivalActual = time.UnixMilli(int64(stop.Timetable.ScheduledDepartureTime.(float64))).Local().Format("15:04")
				}
			}
		}

		var arrivalDelay string
		arrivalDelayRaw := stop.Timetable.ArrivalDelay
		if arrivalDelayRaw != "" {
			arrivalDelay = fmt.Sprintf(" (%s) ", stop.Timetable.ArrivalDelay)
		}
		var departureActual, departureDelay string
		if stop.Timetable.ActualDepartureTime != nil {
			departureActual = time.UnixMilli(int64(stop.Timetable.ActualDepartureTime.(float64))).Local().Format("15:04")
			departureDelayRaw := stop.Timetable.DepartureDelay
			if departureDelayRaw != "" {
				departureDelay = fmt.Sprintf(" (%s) ", stop.Timetable.DepartureDelay)
			}
		} else if stop.Timetable.ScheduledDepartureTime != nil {
			departureScheduled := time.UnixMilli(int64(stop.Timetable.ScheduledDepartureTime.(float64))).Local().Format("15:04")
			departureDelayRaw := stop.Timetable.DepartureDelay
			if departureDelayRaw != "" {
				departureDelay = fmt.Sprintf(" (%s) ", stop.Timetable.DepartureDelay)
			}
			departureActual = departureScheduled
		}
		track := stop.Track.Actual
		out := fmt.Sprintf("%s (%s) at %s%s - %s%s", stopStationName, track, arrivalActual, arrivalDelay, departureActual, departureDelay)
		stopsSlice = append(stopsSlice, out)
		switch stop.DelayReasons.(type) {
		case string:
			delayResons = append(delayResons, stop.DelayReasons.(string))
		}
	}
	return stopsSlice, delayResons
}

func (c iceportalClient) detectWiFi() bool {
	if c.local {
		return true
	}
	wifiStatusCmd := exec.Command("bash", "-c", "ipconfig getsummary $(networksetup -listallhardwareports | awk '/Hardware Port: Wi-Fi/{getline; print $2}') | awk -F ' SSID : ' '/ SSID : / {print $2}'")
	var out strings.Builder
	wifiStatusCmd.Stdout = &out
	err := wifiStatusCmd.Run()
	if err != nil {
		panic(err.Error())
	}
	for _, wifi := range dbWiFis {
		if wifi == strings.TrimSpace(out.String()) {
			return true
		} else if strings.TrimSpace(out.String()) == "<redacted>" {
			fmt.Println(`Please run "sudo ipconfig setverbose 1".`)
			return false
		}
	}
	return false
}
