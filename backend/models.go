package backend

import "time"

type Device struct {
	ID                   int     `json:"id"`
	Code                 string  `json:"code"`
	Name                 string  `json:"name"`
	Type                 string  `json:"type"`
	Zone                 string  `json:"zone"`
	RatedPower           float64 `json:"rated_power"`
	RatedCoolingCapacity float64 `json:"rated_cooling_capacity"`
}

type DeviceTelemetry struct {
	Time            time.Time `json:"time"`
	DeviceID        int       `json:"device_id"`
	SupplyTemp      float64   `json:"supply_temp"`
	ReturnTemp      float64   `json:"return_temp"`
	FlowRate        float64   `json:"flow_rate"`
	Power           float64   `json:"power"`
	Pressure        float64   `json:"pressure"`
	COP             float64   `json:"cop"`
	CoolingCapacity float64   `json:"cooling_capacity"`
	SetpointTemp    float64   `json:"setpoint_temp"`
	Status          int       `json:"status"`
}

type PUERecord struct {
	Time         time.Time `json:"time"`
	ITPower      float64   `json:"it_power"`
	CoolingPower float64   `json:"cooling_power"`
	TotalPower   float64   `json:"total_power"`
	PUEValue     float64   `json:"pue_value"`
}

type ZoneCoolingDemand struct {
	Time             time.Time `json:"time"`
	Zone             string    `json:"zone"`
	SetpointTemp     float64   `json:"setpoint_temp"`
	CurrentTemp      float64   `json:"current_temp"`
	HeatLoad         float64   `json:"heat_load"`
	AllocatedCooling float64   `json:"allocated_cooling"`
	OptimalCooling   float64   `json:"optimal_cooling"`
}

type OptimizationSuggestion struct {
	ID             int       `json:"id"`
	Time           time.Time `json:"time"`
	SuggestionType string    `json:"suggestion_type"`
	DeviceID       int       `json:"device_id"`
	Zone           string    `json:"zone"`
	CurrentValue   float64   `json:"current_value"`
	SuggestedValue float64   `json:"suggested_value"`
	ExpectedSaving float64   `json:"expected_saving"`
	Reason         string    `json:"reason"`
	Status         string    `json:"status"`
}

type Alarm struct {
	ID              int       `json:"id"`
	Time            time.Time `json:"time"`
	AlarmLevel      int       `json:"alarm_level"`
	DeviceID        int       `json:"device_id"`
	AlarmType       string    `json:"alarm_type"`
	Message         string    `json:"message"`
	MetricName      string    `json:"metric_name"`
	MetricValue     float64   `json:"metric_value"`
	Threshold       float64   `json:"threshold"`
	DurationMinutes float64   `json:"duration_minutes"`
	Acknowledged    bool      `json:"acknowledged"`
	DingTalkSent    bool      `json:"dingtalk_sent"`
}

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type DeviceLatestState struct {
	Device     Device           `json:"device"`
	Telemetry  DeviceTelemetry  `json:"telemetry"`
	COPColor   string           `json:"cop_color"`
}

type SankeyNode struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Color string  `json:"color"`
}

type SankeyLink struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	Value  float64 `json:"value"`
}

type SankeyData struct {
	Nodes []SankeyNode `json:"nodes"`
	Links []SankeyLink `json:"links"`
}

type EfficiencyRanking struct {
	DeviceID   int     `json:"device_id"`
	DeviceCode string  `json:"device_code"`
	DeviceName string  `json:"device_name"`
	DeviceType string  `json:"device_type"`
	AvgCOP     float64 `json:"avg_cop"`
	AvgPower   float64 `json:"avg_power"`
	COPColor   string  `json:"cop_color"`
}
