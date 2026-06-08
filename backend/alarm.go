package backend

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

func CheckAlarms(cfg *Config) ([]Alarm, error) {
	db := GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var newAlarms []Alarm

	rows, err := db.Query(`
		SELECT dt.device_id, COUNT(*) as cnt,
			MAX(dt.supply_temp), MAX(dt.return_temp),
			MAX(dt.pressure), MIN(dt.cop),
			MAX(dt.power), d.rated_power
		FROM device_telemetry dt
		JOIN devices d ON dt.device_id = d.id
		WHERE dt.time > NOW() - INTERVAL '15 minutes'
			AND (dt.supply_temp > 15 OR dt.return_temp > 25 OR dt.pressure > 1.2 OR dt.cop < 3 OR dt.power > d.rated_power * 1.1)
		GROUP BY dt.device_id, d.rated_power
		HAVING COUNT(*) >= 20`)
	if err != nil {
		return nil, fmt.Errorf("query level 1 alarms: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var deviceID int
		var cnt int
		var maxSupplyTemp, maxReturnTemp, maxPressure, minCOP, maxPower, ratedPower float64
		if err := rows.Scan(&deviceID, &cnt, &maxSupplyTemp, &maxReturnTemp, &maxPressure, &minCOP, &maxPower, &ratedPower); err != nil {
			return nil, fmt.Errorf("scan level 1 alarm row: %w", err)
		}

		type thresholdCheck struct {
			metricName  string
			metricValue float64
			threshold   float64
			message     string
		}

		checks := []thresholdCheck{}
		if maxSupplyTemp > 15 {
			checks = append(checks, thresholdCheck{"supply_temp", maxSupplyTemp, 15, fmt.Sprintf("供水温度持续超过15°C，当前最大值: %.1f°C", maxSupplyTemp)})
		}
		if maxReturnTemp > 25 {
			checks = append(checks, thresholdCheck{"return_temp", maxReturnTemp, 25, fmt.Sprintf("回水温度持续超过25°C，当前最大值: %.1f°C", maxReturnTemp)})
		}
		if maxPressure > 1.2 {
			checks = append(checks, thresholdCheck{"pressure", maxPressure, 1.2, fmt.Sprintf("压力持续超过1.2MPa，当前最大值: %.2fMPa", maxPressure)})
		}
		if minCOP < 3 {
			checks = append(checks, thresholdCheck{"cop", minCOP, 3, fmt.Sprintf("COP持续低于3，当前最小值: %.2f", minCOP)})
		}
		if ratedPower > 0 && maxPower > ratedPower*1.1 {
			checks = append(checks, thresholdCheck{"power", maxPower, ratedPower * 1.1, fmt.Sprintf("功率持续超过额定功率1.1倍(%.1fkW)，当前最大值: %.1fkW", ratedPower*1.1, maxPower)})
		}

		for _, check := range checks {
			var exists bool
			err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM alarms WHERE device_id = $1 AND alarm_level = 1 AND alarm_type = 'parameter_exceed' AND metric_name = $2 AND acknowledged = false)`, deviceID, check.metricName).Scan(&exists)
			if err != nil {
				log.Printf("check existing alarm for device %d metric %s: %v", deviceID, check.metricName, err)
				continue
			}
			if exists {
				continue
			}

			alarm := Alarm{
				AlarmLevel:      1,
				DeviceID:        deviceID,
				AlarmType:       "parameter_exceed",
				Message:         check.message,
				MetricName:      check.metricName,
				MetricValue:     check.metricValue,
				Threshold:       check.threshold,
				DurationMinutes: float64(cfg.AlarmLevel1Duration),
				Acknowledged:    false,
				DingTalkSent:    false,
			}

			err = db.QueryRow(`INSERT INTO alarms (alarm_level, device_id, alarm_type, message, metric_name, metric_value, threshold, duration_minutes, acknowledged, dingtalk_sent) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id, time`,
				alarm.AlarmLevel, alarm.DeviceID, alarm.AlarmType, alarm.Message, alarm.MetricName, alarm.MetricValue, alarm.Threshold, alarm.DurationMinutes, alarm.Acknowledged, alarm.DingTalkSent).Scan(&alarm.ID, &alarm.Time)
			if err != nil {
				log.Printf("insert level 1 alarm: %v", err)
				continue
			}

			newAlarms = append(newAlarms, alarm)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("level 1 rows error: %w", err)
	}

	var pueCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM pue_records WHERE time > NOW() - INTERVAL '35 minutes' AND pue_value > $1`, cfg.PUEThreshold2).Scan(&pueCount)
	if err != nil {
		return nil, fmt.Errorf("query level 2 alarms: %w", err)
	}

	if pueCount >= 6 {
		var exists bool
		err = db.QueryRow(`SELECT EXISTS(SELECT 1 FROM alarms WHERE alarm_level = 2 AND alarm_type = 'pue_exceed' AND acknowledged = false)`).Scan(&exists)
		if err != nil {
			log.Printf("check existing level 2 alarm: %v", err)
		} else if !exists {
			var latestPUE float64
			var latestTime time.Time
			err = db.QueryRow(`SELECT pue_value, time FROM pue_records ORDER BY time DESC LIMIT 1`).Scan(&latestPUE, &latestTime)
			if err != nil {
				log.Printf("query latest pue: %v", err)
			} else {
				alarm := Alarm{
					AlarmLevel:      2,
					AlarmType:       "pue_exceed",
					Message:         "PUE持续超过1.5超过30分钟",
					MetricName:      "pue",
					MetricValue:     latestPUE,
					Threshold:       1.5,
					DurationMinutes: float64(cfg.AlarmLevel2Duration),
					Acknowledged:    false,
					DingTalkSent:    false,
				}

				err = db.QueryRow(`INSERT INTO alarms (alarm_level, device_id, alarm_type, message, metric_name, metric_value, threshold, duration_minutes, acknowledged, dingtalk_sent) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id, time`,
					alarm.AlarmLevel, alarm.DeviceID, alarm.AlarmType, alarm.Message, alarm.MetricName, alarm.MetricValue, alarm.Threshold, alarm.DurationMinutes, alarm.Acknowledged, alarm.DingTalkSent).Scan(&alarm.ID, &alarm.Time)
				if err != nil {
					log.Printf("insert level 2 alarm: %v", err)
				} else {
					newAlarms = append(newAlarms, alarm)
				}
			}
		}
	}

	for i := range newAlarms {
		if cfg.DingTalkWebhook != "" {
			if err := SendDingTalkNotification(cfg.DingTalkWebhook, newAlarms[i]); err != nil {
				log.Printf("send dingtalk notification for alarm %d: %v", newAlarms[i].ID, err)
			}
		}
	}

	return newAlarms, nil
}

func SendDingTalkNotification(webhook string, alarm Alarm) error {
	deviceStr := "系统"
	if alarm.DeviceID != 0 {
		deviceStr = fmt.Sprintf("Device %d", alarm.DeviceID)
	}

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": "制冷系统告警",
			"text": fmt.Sprintf("## 告警通知\n**级别**: Level %d\n**设备**: %s\n**类型**: %s\n**详情**: %s\n**时间**: %s",
				alarm.AlarmLevel, deviceStr, alarm.AlarmType, alarm.Message, alarm.Time.Format("2006-01-02 15:04:05")),
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal dingtalk payload: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhook, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("send dingtalk request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("dingtalk returned status %d", resp.StatusCode)
	}

	db := GetDB()
	if db != nil {
		_, err = db.Exec(`UPDATE alarms SET dingtalk_sent = true WHERE id = $1`, alarm.ID)
		if err != nil {
			log.Printf("update dingtalk_sent for alarm %d: %v", alarm.ID, err)
		}
	}

	return nil
}

func GetAlarms(level int, acknowledged *bool, limit int) ([]Alarm, error) {
	db := GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT id, time, alarm_level, device_id, alarm_type, message, metric_name, metric_value, threshold, duration_minutes, acknowledged, dingtalk_sent FROM alarms WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if level > 0 {
		query += fmt.Sprintf(" AND alarm_level = $%d", argIdx)
		args = append(args, level)
		argIdx++
	}
	if acknowledged != nil {
		query += fmt.Sprintf(" AND acknowledged = $%d", argIdx)
		args = append(args, *acknowledged)
		argIdx++
	}

	query += " ORDER BY time DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query alarms: %w", err)
	}
	defer rows.Close()

	var alarms []Alarm
	for rows.Next() {
		var a Alarm
		if err := rows.Scan(&a.ID, &a.Time, &a.AlarmLevel, &a.DeviceID, &a.AlarmType, &a.Message, &a.MetricName, &a.MetricValue, &a.Threshold, &a.DurationMinutes, &a.Acknowledged, &a.DingTalkSent); err != nil {
			return nil, fmt.Errorf("scan alarm: %w", err)
		}
		alarms = append(alarms, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return alarms, nil
}

func AcknowledgeAlarm(alarmID int) error {
	db := GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	result, err := db.Exec(`UPDATE alarms SET acknowledged = true WHERE id = $1`, alarmID)
	if err != nil {
		return fmt.Errorf("acknowledge alarm: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("alarm %d not found", alarmID)
	}

	return nil
}

func GetUnacknowledgedAlarmCount() (map[int]int, error) {
	db := GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := db.Query(`SELECT alarm_level, COUNT(*) FROM alarms WHERE acknowledged = false GROUP BY alarm_level`)
	if err != nil {
		return nil, fmt.Errorf("query unacknowledged alarm count: %w", err)
	}
	defer rows.Close()

	result := make(map[int]int)
	for rows.Next() {
		var level, count int
		if err := rows.Scan(&level, &count); err != nil {
			return nil, fmt.Errorf("scan alarm count: %w", err)
		}
		result[level] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}
