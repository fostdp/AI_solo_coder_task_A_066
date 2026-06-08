package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

type ModbusConfig struct {
	IdleTimeoutSeconds    int `json:"idle_timeout_seconds"`
	MaxRetries            int `json:"max_retries"`
	DialTimeoutSeconds    int `json:"dial_timeout_seconds"`
	ReadTimeoutSeconds    int `json:"read_timeout_seconds"`
	CollectIntervalSeconds int `json:"collect_interval_seconds"`
}

type PUEConfig struct {
	CalculateIntervalSeconds int     `json:"calculate_interval_seconds"`
	DistributionLossRatio    float64 `json:"distribution_loss_ratio"`
	ITDefaultPower           float64 `json:"it_default_power"`
	PUEThreshold1            float64 `json:"pue_threshold_1"`
	PUEThreshold2            float64 `json:"pue_threshold_2"`
}

type OptimizationConfig struct {
	IntervalSeconds int     `json:"interval_seconds"`
	DiffThreshold   float64 `json:"diff_threshold"`
	SavingRatio     float64 `json:"saving_ratio"`
	HeatLoadDivisor float64 `json:"heat_load_divisor"`
}

type AlarmThresholds struct {
	SupplyTemp float64 `json:"supply_temp"`
	ReturnTemp float64 `json:"return_temp"`
	Pressure   float64 `json:"pressure"`
	COP        float64 `json:"cop"`
	PowerRatio float64 `json:"power_ratio"`
}

type AlarmConfig struct {
	CheckIntervalSeconds     int             `json:"check_interval_seconds"`
	Level1DurationMinutes    int             `json:"level1_duration_minutes"`
	Level2DurationMinutes    int             `json:"level2_duration_minutes"`
	Level1Thresholds         AlarmThresholds `json:"level1_thresholds"`
	DingTalkTimeoutSeconds   int             `json:"dingtalk_timeout_seconds"`
	DingTalkMaxRetries       int             `json:"dingtalk_max_retries"`
	DingTalkRetryIntervalSec int             `json:"dingtalk_retry_interval_seconds"`
}

type Config struct {
	DBHost         string `json:"-"`
	DBPort         string `json:"-"`
	DBUser         string `json:"-"`
	DBPassword     string `json:"-"`
	DBName         string `json:"-"`
	HTTPPort       string `json:"-"`
	ModbusHost     string `json:"-"`
	ModbusPort     string `json:"-"`
	DingTalkWebhook string `json:"-"`
	Modbus         ModbusConfig      `json:"modbus"`
	PUE            PUEConfig         `json:"pue"`
	Optimization   OptimizationConfig `json:"optimization"`
	Alarm          AlarmConfig       `json:"alarm"`
}

func LoadConfig() *Config {
	cfg := &Config{
		DBHost:          getEnv("DB_HOST", "localhost"),
		DBPort:          getEnv("DB_PORT", "5432"),
		DBUser:          getEnv("DB_USER", "postgres"),
		DBPassword:      getEnv("DB_PASSWORD", "postgres"),
		DBName:          getEnv("DB_NAME", "dc_cooling"),
		HTTPPort:        getEnv("HTTP_PORT", "8080"),
		ModbusHost:      getEnv("MODBUS_HOST", "localhost"),
		ModbusPort:      getEnv("MODBUS_PORT", "502"),
		DingTalkWebhook: getEnv("DINGTALK_WEBHOOK", ""),
		Modbus: ModbusConfig{
			IdleTimeoutSeconds:     60,
			MaxRetries:             3,
			DialTimeoutSeconds:     5,
			ReadTimeoutSeconds:     10,
			CollectIntervalSeconds: 30,
		},
		PUE: PUEConfig{
			CalculateIntervalSeconds: 300,
			DistributionLossRatio:    0.03,
			ITDefaultPower:           1000.0,
			PUEThreshold1:            1.4,
			PUEThreshold2:            1.5,
		},
		Optimization: OptimizationConfig{
			IntervalSeconds: 600,
			DiffThreshold:   0.05,
			SavingRatio:     0.3,
			HeatLoadDivisor: 10.0,
		},
		Alarm: AlarmConfig{
			CheckIntervalSeconds:     60,
			Level1DurationMinutes:    10,
			Level2DurationMinutes:    30,
			Level1Thresholds: AlarmThresholds{
				SupplyTemp: 15.0,
				ReturnTemp: 25.0,
				Pressure:   1.2,
				COP:        3.0,
				PowerRatio: 1.1,
			},
			DingTalkTimeoutSeconds:   10,
			DingTalkMaxRetries:       3,
			DingTalkRetryIntervalSec: 30,
		},
	}

	configPath := getEnv("CONFIG_PATH", "config/config.json")
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "config parse error: %v\n", err)
		}
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value, exists := os.LookupEnv(key); exists {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}
