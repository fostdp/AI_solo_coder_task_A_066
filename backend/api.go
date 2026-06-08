package backend

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

func HandleGetDevices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	db := GetDB()
	if db == nil {
		http.Error(w, "database not initialized", http.StatusInternalServerError)
		return
	}

	rows, err := db.Query(`SELECT id, device_code, device_name, device_type, zone, rated_power, rated_cooling_capacity FROM devices ORDER BY id`)
	if err != nil {
		http.Error(w, fmt.Sprintf("query devices: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Code, &d.Name, &d.Type, &d.Zone, &d.RatedPower, &d.RatedCoolingCapacity); err != nil {
			http.Error(w, fmt.Sprintf("scan device: %v", err), http.StatusInternalServerError)
			return
		}
		devices = append(devices, d)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, fmt.Sprintf("rows error: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(devices)
}

func HandleGetDeviceTelemetry(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	deviceIDStr := r.URL.Query().Get("device_id")
	if deviceIDStr == "" {
		http.Error(w, "device_id is required", http.StatusBadRequest)
		return
	}
	deviceID, err := strconv.Atoi(deviceIDStr)
	if err != nil {
		http.Error(w, "invalid device_id", http.StatusBadRequest)
		return
	}

	hours := 24
	if hoursStr := r.URL.Query().Get("hours"); hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil {
			hours = h
		}
	}

	result, err := GetDeviceTelemetryHistory(deviceID, hours)
	if err != nil {
		http.Error(w, fmt.Sprintf("get telemetry: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

func HandleGetPUETrend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	hours := 24
	if hoursStr := r.URL.Query().Get("hours"); hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil {
			hours = h
		}
	}

	result, err := GetPUETrend(hours)
	if err != nil {
		http.Error(w, fmt.Sprintf("get pue trend: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

func HandleGetCurrentPUE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	record, err := GetCurrentPUE()
	if err != nil {
		http.Error(w, fmt.Sprintf("get current pue: %v", err), http.StatusInternalServerError)
		return
	}

	if record == nil {
		http.Error(w, "no pue data found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(record)
}

func HandleGetZones(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	result, err := GetZoneCoolingDemands()
	if err != nil {
		http.Error(w, fmt.Sprintf("get zones: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

func HandleGetSankey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	result, err := GetSankeyData()
	if err != nil {
		http.Error(w, fmt.Sprintf("get sankey: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

func HandleGetRanking(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	result, err := GetEfficiencyRanking()
	if err != nil {
		http.Error(w, fmt.Sprintf("get ranking: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

func HandleGetSuggestions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	db := GetDB()
	if db == nil {
		http.Error(w, "database not initialized", http.StatusInternalServerError)
		return
	}

	rows, err := db.Query(`SELECT id, time, suggestion_type, device_id, zone, current_value, suggested_value, expected_saving, reason, status FROM optimization_suggestions WHERE time > NOW() - INTERVAL '7 days' ORDER BY time DESC LIMIT 100`)
	if err != nil {
		http.Error(w, fmt.Sprintf("query suggestions: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var suggestions []OptimizationSuggestion
	for rows.Next() {
		var s OptimizationSuggestion
		var deviceID sql.NullInt64
		if err := rows.Scan(&s.ID, &s.Time, &s.SuggestionType, &deviceID, &s.Zone, &s.CurrentValue, &s.SuggestedValue, &s.ExpectedSaving, &s.Reason, &s.Status); err != nil {
			http.Error(w, fmt.Sprintf("scan suggestion: %v", err), http.StatusInternalServerError)
			return
		}
		if deviceID.Valid {
			s.DeviceID = int(deviceID.Int64)
		}
		suggestions = append(suggestions, s)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, fmt.Sprintf("rows error: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(suggestions)
}

func HandleGetAlarms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	level := 0
	if levelStr := r.URL.Query().Get("level"); levelStr != "" {
		if l, err := strconv.Atoi(levelStr); err == nil {
			level = l
		}
	}

	var acknowledged *bool
	if ackStr := r.URL.Query().Get("acknowledged"); ackStr != "" {
		if b, err := strconv.ParseBool(ackStr); err == nil {
			acknowledged = &b
		}
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	result, err := GetAlarms(level, acknowledged, limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("get alarms: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

func HandleAcknowledgeAlarm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "alarm id is required", http.StatusBadRequest)
		return
	}
	alarmID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid alarm id", http.StatusBadRequest)
		return
	}

	if err := AcknowledgeAlarm(alarmID); err != nil {
		http.Error(w, fmt.Sprintf("acknowledge alarm: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func HandleGetDeviceStates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	result, err := GetDeviceLatestStates()
	if err != nil {
		http.Error(w, fmt.Sprintf("get device states: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

func HandleGetAlarmCounts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	counts, err := GetUnacknowledgedAlarmCount()
	if err != nil {
		http.Error(w, fmt.Sprintf("get alarm counts: %v", err), http.StatusInternalServerError)
		return
	}

	result := make(map[string]int)
	for level, count := range counts {
		key := fmt.Sprintf("level%d", level)
		result[key] = count
	}

	json.NewEncoder(w).Encode(result)
}
