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
	var body []byte
	if c.local {
		var err error
		body, err = os.ReadFile("testdata/status.json")
		if err != nil {
			return err
		}
	} else {
		req, err := http.NewRequest(http.MethodGet, iceportalStatusURL, nil)
		if err != nil {
			return err
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
	}

	if err := json.Unmarshal(body, &c.status); err != nil {
		return err
	}

	return nil
}

func (c *iceportalClient) getTrip() error {
	var body []byte
	if c.local {
		var err error
		body, err = os.ReadFile("testdata/trip.json")
		if err != nil {
			return err
		}
	} else {
		req, err := http.NewRequest(http.MethodGet, iceportalTripURL, nil)
		if err != nil {
			return err
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
	}
	if err := json.Unmarshal(body, &c.trip); err != nil {
		return err
	}
	return nil
}

func (c iceportalClient) calculateArrival() string {
	currentNextStop := c.trip.Trip.StopInfo.ActualNext
	var arivelTime time.Time
	for _, stop := range c.trip.Trip.Stops {
		if stop.Station.EvaNr == currentNextStop {
			arivelTime = time.UnixMilli(int64(stop.Timetable.ActualArrivalTime.(float64))).Local()
		}
	}
	return time.Until(arivelTime).Round(time.Minute).String()
}

func (c iceportalClient) outputBuilder() string {
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
	nextStopString := stops[0]
	re := regexp.MustCompile(`^(.*)(at [0-9]{2}:[0-9]{2})(\ -\ [0-9]{2}:[0-9]{2})$`)
	sub := fmt.Sprintf("${1} in %s ${2}", c.calculateArrival())
	nextStop := re.ReplaceAllString(nextStopString, sub)
	wifi := c.getWifiStatus()
	delayReason := strings.Join(delayReasons, "\n")
	if len(delayReason) == 0 {
		return fmt.Sprintf(":train.side.front.car: %s\n---\n***%s***|md=true\n%s → %s\n%s\n%s\n%s\n---\n**Next Stop:**|md=true\n%s\n---\n**Wifi:**|md=true\n%s", nextStop, trainName, startingStation, destinationStation, series, class, speed, strings.Join(stops, "\n"), wifi)
	} else {
		return fmt.Sprintf(":train.side.front.car: %s\n---\n***%s***|md=true\n%s → %s\n%s\n%s\n%s\n---\n**Next Stop:**|md=true\n%s\n---\n**Wifi:**|md=true\n%s\n**Delay Reasons:**|md=true\n%s", nextStop, trainName, startingStation, destinationStation, series, class, speed, strings.Join(stops, "\n"), wifi, delayReason)
	}
}

func (c iceportalClient) getWifiStatus() string {
	wifiCurrentStatus := strings.ToLower(c.status.Connectivity.CurrentState)
	wifiNextStatus := strings.ToLower(c.status.Connectivity.NextState)
	wifiStatusRemainingSecondsRawStr := c.status.Connectivity.RemainingTimeSeconds
	wifiStatusRemainingSecondsRaw := strconv.Itoa(wifiStatusRemainingSecondsRawStr)
	wifiRemainingString, err := time.ParseDuration(wifiStatusRemainingSecondsRaw + "s")
	if err != nil {
		return fmt.Sprintf("Quality: %s\nChanges to %s", wifiCurrentStatus, wifiNextStatus)
	} else {
		return fmt.Sprintf("Quality: %s\nChanges to %s in %s", wifiCurrentStatus, wifiNextStatus, wifiRemainingString)
	}
}

func (c iceportalClient) getStops() ([]string, []string) {
	var stopsSlice []string
	var delayResons []string

	currentNextStop := c.trip.Trip.StopInfo.ActualNext
	var stopStationName, arrivalActual, ArrivalDelay, ArrivalDelayRaw, departureActual, departureDelay, departureDelayRaw, departureScheduled, track string
	for _, stop := range c.trip.Trip.Stops {
		if stop.Info.Passed && stop.Info.PositionStatus != "arrived" {
			continue
		}
		isCurrentStop := stop.Station.EvaNr == currentNextStop
		stopStationName = stop.Station.Name
		if stop.Timetable.ActualArrivalTime != nil {
			arrivalActual = time.UnixMilli(int64(stop.Timetable.ActualArrivalTime.(float64))).Local().Format("15:04")
		} else if stop.Timetable.ScheduledArrivalTime != nil {
			arrivalActual = time.UnixMilli(int64(stop.Timetable.ScheduledArrivalTime.(float64))).Local().Format("15:04")
		} else {
			arrivalActual = "Unkmown"
			if isCurrentStop {
				if stop.Timetable.ActualDepartureTime != nil {
					arrivalActual = time.UnixMilli(int64(stop.Timetable.ScheduledArrivalTime.(float64))).Local().Format("15:04")
				} else if stop.Timetable.ScheduledDepartureTime != nil {
					arrivalActual = time.UnixMilli(int64(stop.Timetable.ScheduledDepartureTime.(float64))).Local().Format("15:04")
				}
			}
		}

		ArrivalDelayRaw = stop.Timetable.ArrivalDelay
		if ArrivalDelayRaw != "" {
			ArrivalDelay = fmt.Sprintf(" (%s) ", stop.Timetable.ArrivalDelay)
		}
		if stop.Timetable.ActualDepartureTime != nil {
			departureActual = time.UnixMilli(int64(stop.Timetable.ActualDepartureTime.(float64))).Local().Format("15:04")
			departureDelayRaw = stop.Timetable.DepartureDelay
			if departureDelayRaw != "" {
				departureDelay = fmt.Sprintf(" (%s) ", stop.Timetable.DepartureDelay)
			}
		} else if stop.Timetable.ScheduledDepartureTime != nil {
			departureScheduled = time.UnixMilli(int64(stop.Timetable.ScheduledDepartureTime.(float64))).Local().Format("15:04")
			departureDelayRaw = stop.Timetable.DepartureDelay
			if departureDelayRaw != "" {
				departureDelay = fmt.Sprintf(" (%s) ", stop.Timetable.DepartureDelay)
			}
			departureActual = departureScheduled
		}
		track = stop.Track.Actual
		out := fmt.Sprintf("%s (%s) at %s%s - %s%s", stopStationName, track, arrivalActual, ArrivalDelay, departureActual, departureDelay)
		stopsSlice = append(stopsSlice, out)
		ArrivalDelayRaw = ""
		departureDelayRaw = ""
		ArrivalDelay = ""
		departureDelay = ""
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
		}
	}
	return false
}
