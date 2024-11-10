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

var dbWiFis []string = []string{"WIFI@DB", "WIFIonICE"}
var iceportalStatusUrl = "https://iceportal.de/api1/rs/status"
var iceportalTripUrl = "https://iceportal.de/api1/rs/tripInfo/trip"
var iceportalLoginCheckUrl = "https://login.wifionice.de/cna/wifi/user_info"
var iceportalLoginUrl = "https://login.wifionice.de/cna/logon"

func main() {
	c := newClient(local)
	if !c.detectWiFi() {
		fmt.Println("")
		os.Exit(0)
	}
	err := c.iceportalTryLogin()
	if err != nil {
		panic(err.Error())
	}
	if !c.loggedin {
		c.iceportalLogin()
	}
	c.getStatus()
	c.getTrip()
	fmt.Println(c.outputBuilder())
}

type iceportalClient struct {
	client   *http.Client
	status   ApiStatus
	trip     ApiTrip
	loggedin bool
	local    bool
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

func (c *iceportalClient) iceportalTryLogin() error {
	if c.local {
		return nil
	}
	req, err := http.NewRequest(http.MethodGet, iceportalLoginCheckUrl, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var loginResponse iceportalLoginResponse
	if err = json.Unmarshal(body, &loginResponse); err != nil {
		return err
	}
	if loginResponse.Result.Authenticated == "1" {
		c.loggedin = true
		return nil
	}
	return nil
}

func (c *iceportalClient) iceportalLogin() error {
	if c.local {
		return nil
	}
	req, err := http.NewRequest(http.MethodPost, iceportalLoginUrl, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login unsuccessful")
	}
	c.loggedin = true
	return nil
}

func (c *iceportalClient) getStatus() error {
	var body []byte
	if c.local {
		var err error
		body, err = os.ReadFile("/Users/transacid/testdata/status.json")
		if err != nil {
			return err
		}
	} else {
		req, err := http.NewRequest(http.MethodGet, iceportalStatusUrl, nil)
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
		body, err = os.ReadFile("/Users/transacid/testdata/trip.json")
		if err != nil {
			return err
		}
	} else {
		req, err := http.NewRequest(http.MethodGet, iceportalTripUrl, nil)
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
	stops := c.getStops()
	nextStopString := stops[0]
	re := regexp.MustCompile(`^(.*)(at [0-9]{2}:[0-9]{2})(\ -\ [0-9]{2}:[0-9]{2})$`)
	sub := fmt.Sprintf("${1} in %s ${2}", c.calculateArrival())
	nextStop := re.ReplaceAllString(nextStopString, sub)
	wifi := c.getWifiStatus()

	return fmt.Sprintf(":train.side.front.car: %s\n---\n***%s***|md=true\n%s → %s\n%s\n%s\n%s\n---\n**Next Stop:**|md=true\n%s\n---\n**Wifi:**|md=true\n%s", nextStop, trainName, startingStation, destinationStation, series, class, speed, strings.Join(stops, "\n"), wifi)
}

func (c iceportalClient) getWifiStatus() string {
	wifiCurrentStatus := strings.ToLower(c.status.Connectivity.CurrentState)
	wifiStatusRemainingSeconds := c.status.Connectivity.RemainingTimeSeconds
	wifiRemainingString := fmt.Sprintf("%d:%d", (wifiStatusRemainingSeconds-(wifiStatusRemainingSeconds%60))/60, wifiStatusRemainingSeconds-(wifiStatusRemainingSeconds-(wifiStatusRemainingSeconds%60))/60*60)
	wifiNextStatus := strings.ToLower(c.status.Connectivity.NextState)
	return fmt.Sprintf("Quality: %s\nChanges to %s in %s", wifiCurrentStatus, wifiNextStatus, wifiRemainingString)
}

func (c iceportalClient) getStops() []string {
	var stopsSlice []string

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
	}
	return stopsSlice
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

type ApiStatus struct {
	Connection   bool    `json:"connection"`
	ServiceLevel string  `json:"serviceLevel"`
	GpsStatus    string  `json:"gpsStatus"`
	Internet     string  `json:"internet"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	TileY        int     `json:"tileY"`
	TileX        int     `json:"tileX"`
	Series       string  `json:"series"`
	ServerTime   float64 `json:"serverTime"`
	Speed        float64 `json:"speed"`
	TrainType    string  `json:"trainType"`
	Tzn          string  `json:"tzn"`
	WagonClass   string  `json:"wagonClass"`
	Connectivity struct {
		CurrentState         string `json:"currentState"`
		NextState            string `json:"nextState"`
		RemainingTimeSeconds int    `json:"remainingTimeSeconds"`
	} `json:"connectivity"`
	BapInstalled bool `json:"bapInstalled"`
}

type ApiTrip struct {
	Trip struct {
		TripDate             string `json:"tripDate"`
		TrainType            string `json:"trainType"`
		Vzn                  string `json:"vzn"`
		ActualPosition       int    `json:"actualPosition"`
		DistanceFromLastStop int    `json:"distanceFromLastStop"`
		TotalDistance        int    `json:"totalDistance"`
		StopInfo             struct {
			ScheduledNext     string `json:"scheduledNext"`
			ActualNext        string `json:"actualNext"`
			ActualLast        string `json:"actualLast"`
			ActualLastStarted string `json:"actualLastStarted"`
			FinalStationName  string `json:"finalStationName"`
			FinalStationEvaNr string `json:"finalStationEvaNr"`
		} `json:"stopInfo"`
		Stops []struct {
			Station struct {
				EvaNr          string      `json:"evaNr"`
				Name           string      `json:"name"`
				Code           interface{} `json:"code"`
				Geocoordinates struct {
					Latitude  float64 `json:"latitude"`
					Longitude float64 `json:"longitude"`
				} `json:"geocoordinates"`
			} `json:"station"`
			Timetable struct {
				ScheduledArrivalTime    interface{} `json:"scheduledArrivalTime"`
				ActualArrivalTime       interface{} `json:"actualArrivalTime"`
				ShowActualArrivalTime   interface{} `json:"showActualArrivalTime"`
				ArrivalDelay            string      `json:"arrivalDelay"`
				ScheduledDepartureTime  interface{} `json:"scheduledDepartureTime"`
				ActualDepartureTime     interface{} `json:"actualDepartureTime"`
				ShowActualDepartureTime bool        `json:"showActualDepartureTime"`
				DepartureDelay          string      `json:"departureDelay"`
			} `json:"timetable"`
			Track struct {
				Scheduled string `json:"scheduled"`
				Actual    string `json:"actual"`
			} `json:"track"`
			Info struct {
				Status            int    `json:"status"`
				Passed            bool   `json:"passed"`
				PositionStatus    string `json:"positionStatus"`
				Distance          int    `json:"distance"`
				DistanceFromStart int    `json:"distanceFromStart"`
			} `json:"info"`
			DelayReasons interface{} `json:"delayReasons"`
		} `json:"stops"`
	} `json:"trip"`
	Connection    interface{} `json:"connection"`
	SelectedRoute struct {
		ConflictInfo struct {
			Status string      `json:"status"`
			Text   interface{} `json:"text"`
		} `json:"conflictInfo"`
		Mobility interface{} `json:"mobility"`
	} `json:"selectedRoute"`
	Active interface{} `json:"active"`
}

type iceportalLoginResponse struct {
	Result struct {
		Authenticated string `json:"authenticated"`
	} `json:"result"`
}
