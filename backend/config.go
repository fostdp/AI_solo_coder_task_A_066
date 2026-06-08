package backend

import (
	"os"
	"strconv"
)

type Config struct {
	DBHost              string
	DBPort              string
	DBUser              string
	DBPassword          string
	DBName              string
	HTTPPort            string
	ModbusHost          string
	ModbusPort          string
	DingTalkWebhook     string
	ITDefaultPower      float64
	PUEThreshold1       float64
	PUEThreshold2       float64
	AlarmLevel1Duration int
	AlarmLevel2Duration int
}

func LoadConfig() *Config {
	cfg := &Config{
		DBHost:              getEnv("DB_HOST", "localhost"),
		DBPort:              getEnv("DB_PORT", "5432"),
		DBUser:              getEnv("DB_USER", "postgres"),
		DBPassword:          getEnv("DB_PASSWORD", "postgres"),
		DBName:              getEnv("DB_NAME", "dc_cooling"),
		HTTPPort:            getEnv("HTTP_PORT", "8080"),
		ModbusHost:          getEnv("MODBUS_HOST", "localhost"),
		ModbusPort:          getEnv("MODBUS_PORT", "502"),
		DingTalkWebhook:     getEnv("DINGTALK_WEBHOOK", ""),
		ITDefaultPower:      getEnvFloat("IT_DEFAULT_POWER", 1000.0),
		PUEThreshold1:       getEnvFloat("PUE_THRESHOLD_1", 1.4),
		PUEThreshold2:       getEnvFloat("PUE_THRESHOLD_2", 1.5),
		AlarmLevel1Duration: getEnvInt("ALARM_LEVEL1_DURATION", 10),
		AlarmLevel2Duration: getEnvInt("ALARM_LEVEL2_DURATION", 30),
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
