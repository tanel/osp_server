package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/mux"
)

func defineRoutes() {
	r := mux.NewRouter()

	r.HandleFunc("/api/coordinators/{coordinator_id}/sensors", getCoordinatorSensors).Methods("GET")
	r.HandleFunc("/api/coordinators/{coordinator_id}", putCoordinator).Methods("POST", "PUT")
	r.HandleFunc("/api/coordinators/{coordinator_id}/{hash}", getCoordinator).Methods("GET")

	r.HandleFunc("/api/sensors/{sensor_id}", putSensor).Methods("POST", "PUT")
	r.HandleFunc("/api/sensors/{sensor_id}/ticks", getSensorTicks).Methods("GET")
	r.HandleFunc("/api/sensors/{sensor_id}/dots", getSensorDots).Methods("GET")

	// FIXME: deprecated
	r.HandleFunc("/api/log", getLogsV1).Methods("GET")
	r.HandleFunc("/api/logs", getLogsV1).Methods("GET")

	r.HandleFunc("/api/debug_log", getDebugLogs).Methods("GET")
	r.HandleFunc("/api/debug_logs", getDebugLogs).Methods("GET")

	r.HandleFunc("/api/v1/log", getLogsV1).Methods("GET")
	r.HandleFunc("/api/v1/logs", getLogsV1).Methods("GET")
	r.HandleFunc("/api/v2/log", getLogsV2).Methods("GET")
	r.HandleFunc("/api/v2/logs", getLogsV2).Methods("GET")

	http.Handle("/", r)
}

func getCoordinator(w http.ResponseWriter, r *http.Request) {
	log.Println(r)

	coordinatorID, ok := mux.Vars(r)["coordinator_id"]
	if !ok {
		http.Error(w, "Missing coordinator_id", http.StatusBadRequest)
		return
	}
	hashToken, ok := mux.Vars(r)["hash"]
	if !ok {
		http.Error(w, "Missing token hash", http.StatusBadRequest)
		return
	}

	c := &coordinator{ID: coordinatorID, Token: hashToken}

	token, err := coordinatorToken(c.ID)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if token != c.Token {
		http.Error(w, "Incorrect token for this coordinator", http.StatusUnauthorized)
	}

	name, err := coordinatorName(c.ID)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.Name = name

	b, err := json.Marshal(c)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func putCoordinator(w http.ResponseWriter, r *http.Request) {
	log.Println(r)

	coordinatorID, ok := mux.Vars(r)["coordinator_id"]
	if !ok {
		http.Error(w, "Missing coordinator_id", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var c coordinator
	if err := json.Unmarshal(b, &c); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := saveCoordinatorName(coordinatorID, c.Name); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func putSensor(w http.ResponseWriter, r *http.Request) {
	log.Println(r)

	sensorID, exists := mux.Vars(r)["sensor_id"]
	if !exists {
		http.Error(w, "Missing or invalid sensor_id", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var s sensor
	if err := json.Unmarshal(b, &s); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := saveSensorCoordinates(sensorID, s.Lat, s.Lng); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func getCoordinatorSensors(w http.ResponseWriter, r *http.Request) {
	log.Println(r)

	coordinatorID, ok := mux.Vars(r)["coordinator_id"]
	if !ok {
		http.Error(w, "Missing coordinator_id", http.StatusBadRequest)
		return
	}

	redisClient := redisPool.Get()
	defer redisClient.Close()

	ids, err := redis.Strings(redisClient.Do("SMEMBERS", keyOfCoordinatorSensors(coordinatorID)))
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sensors := make([]*sensor, 0)
	for _, sensorID := range ids {
		if len(sensorID) == 0 {
			log.Println(err)
			http.Error(w, "Invalid or missing sensor ID", http.StatusInternalServerError)
			return
		}
		s := &sensor{ID: sensorID, ControllerID: coordinatorID}

		// Get lat, lng of sensor
		bb, err := redisClient.Do("HMGET", keyOfSensor(sensorID), "lat", "lng")
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if bb != nil {
			list := bb.([]interface{})
			if len(list) > 0 {
				if list[0] != nil {
					s.Lat = string(list[0].([]byte))
				}
				if list[1] != nil {
					s.Lng = string(list[1].([]byte))
				}
			}
		}

		// Get last tick of sensor
		ticks, err := findTicksByRange(sensorID, 0, 0)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(ticks) > 0 {
			s.LastTick = &ticks[0].Datetime

		}

		sensors = append(sensors, s)
	}

	b, err := json.Marshal(sensors)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func getSensorDots(w http.ResponseWriter, r *http.Request) {
	log.Println(r)

	sensorID, exists := mux.Vars(r)["sensor_id"]
	if !exists {
		http.Error(w, "Missing sensor_id", http.StatusBadRequest)
		return
	}

	start, err := strconv.Atoi(r.FormValue("start"))
	if err != nil {
		http.Error(w, "Invalid start", http.StatusBadRequest)
		return
	}

	end, err := strconv.Atoi(r.FormValue("end"))
	if err != nil {
		http.Error(w, "Invalid end", http.StatusBadRequest)
		return
	}

	dotsPerDay, err := strconv.Atoi(r.FormValue("dots_per_day"))
	if err != nil {
		http.Error(w, "Invalid dots_per_day", http.StatusBadRequest)
		return
	}
	if dotsPerDay < 0 || dotsPerDay > 24 {
		http.Error(w, "dots_per_day must be in range 0-24", http.StatusBadRequest)
		return
	}

	redisClient := redisPool.Get()
	defer redisClient.Close()

	ticks, err := findTicksByScore(sensorID, start, end)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var dots []*tick
	if dotsPerDay > 0 {
		dots = findAverages(ticks, dotsPerDay, start, end)
	} else {
		dots = ticks
	}

	b, err := json.Marshal(dots)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func getSensorTicks(w http.ResponseWriter, r *http.Request) {
	log.Println(r)

	sensorID, exists := mux.Vars(r)["sensor_id"]
	if !exists {
		http.Error(w, "Missing sensor_id", http.StatusBadRequest)
		return
	}

	start, err := strconv.Atoi(r.FormValue("start"))
	if err != nil {
		http.Error(w, "Missing or invalid start", http.StatusBadRequest)
		return
	}

	end, err := strconv.Atoi(r.FormValue("end"))
	if err != nil {
		http.Error(w, "Missing or invalid end", http.StatusBadRequest)
		return
	}

	result, err := findTicksByScore(sensorID, start, end)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	b, err := json.Marshal(result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func getLogsV1(w http.ResponseWriter, r *http.Request) {
	getLogs(w, r, loggingKeyV1)
}

func getLogsV2(w http.ResponseWriter, r *http.Request) {
	getLogs(w, r, loggingKeyV2)
}

func getLogs(w http.ResponseWriter, r *http.Request, key string) {
	redisClient := redisPool.Get()
	defer redisClient.Close()

	bb, err := redisClient.Do("LRANGE", key, 0, 1000)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	for _, item := range bb.([]interface{}) {
		s := string(item.([]byte))
		s = strconv.Quote(s)
		w.Write([]byte(s))
		w.Write([]byte("\n\r"))
	}
}

func getDebugLogs(w http.ResponseWriter, r *http.Request) {
	getLogs(w, r, debugLogKey)
}
