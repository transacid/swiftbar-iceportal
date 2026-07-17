package main

import "net/http"

type iceportalClient struct {
	client *http.Client
	status APIStatus
	trip   APITrip
	local  bool
}

type APIStatus struct {
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

type APITrip struct {
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
				EvaNr          string `json:"evaNr"`
				Name           string `json:"name"`
				Code           any    `json:"code"`
				Geocoordinates struct {
					Latitude  float64 `json:"latitude"`
					Longitude float64 `json:"longitude"`
				} `json:"geocoordinates"`
			} `json:"station"`
			Timetable struct {
				ScheduledArrivalTime    any    `json:"scheduledArrivalTime"`
				ActualArrivalTime       any    `json:"actualArrivalTime"`
				ShowActualArrivalTime   any    `json:"showActualArrivalTime"`
				ArrivalDelay            string `json:"arrivalDelay"`
				ScheduledDepartureTime  any    `json:"scheduledDepartureTime"`
				ActualDepartureTime     any    `json:"actualDepartureTime"`
				ShowActualDepartureTime bool   `json:"showActualDepartureTime"`
				DepartureDelay          string `json:"departureDelay"`
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
			DelayReasons any `json:"delayReasons"`
		} `json:"stops"`
	} `json:"trip"`
	Connection    any `json:"connection"`
	SelectedRoute struct {
		ConflictInfo struct {
			Status string `json:"status"`
			Text   any    `json:"text"`
		} `json:"conflictInfo"`
		Mobility any `json:"mobility"`
	} `json:"selectedRoute"`
	Active any `json:"active"`
}
