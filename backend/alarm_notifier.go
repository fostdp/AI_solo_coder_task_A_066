package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type AlarmNotifier struct {
	cfg             *Config
	telemetryCh     chan []DeviceTelemetry
	pueCh           chan *PUERecord
	outCh           chan []Alarm
	stopCh          chan struct{}
	pendingDingTalk []Alarm
	mu              sync.Mutex
}

func NewAlarmNotifier(cfg *Config) *AlarmNotifier {
	return &AlarmNotifier{
		cfg:         cfg,
		telemetryCh: make(chan []DeviceTelemetry, 16),
		pueCh:       make(chan *PUERecord, 16),
		outCh:       make(chan []Alarm, 16),
		stopCh:      make(chan struct{}),
	}
}

func (n *AlarmNotifier) TelemetryCh() chan<- []DeviceTelemetry {
	return n.telemetryCh
}

func (n *AlarmNotifier) PUECh() chan<- *PUERecord {
	return n.pueCh
}

func (n *AlarmNotifier) Output() <-chan []Alarm {
	return n.outCh
}

func (n *AlarmNotifier) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(n.cfg.Alarm.CheckIntervalSeconds) * time.Second)
	defer ticker.Stop()

	go n.retryLoop(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			alarms, err := n.checkAlarms()
			if err != nil {
				log.Printf("alarm check error: %v", err)
				continue
			}
			if len(alarms) > 0 {
				select {
				case n.outCh <- alarms:
				default:
					log.Printf("alarm output channel full, dropping alarms")
				}
			}
		case <-n.telemetryCh:
		case <-n.pueCh:
		}
	}
}

func (n *AlarmNotifier) checkAlarms() ([]Alarm, error) {
	db := GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var newAlarms []Alarm

	thresholds := n.cfg.Alarm.Level1Thresholds
	level1Threshold := n.cfg.Alarm.Level1DurationMinutes * 2

	rows, err := db.Query(fmt.Sprintf(`
		SELECT dt.device_id, COUNT(*) as cnt,
			MAX(dt.supply_temp), MAX(dt.return_temp),
			MAX(dt.pressure), MIN(dt.cop),
			MAX(dt.power), d.rated_power
		FROM device_telemetry dt
		JOIN devices d ON dt.device_id = d.id
		WHERE dt.time > NOW() - INTERVAL '15 minutes'
			AND (dt.supply_temp > $1 OR dt.return_temp > $2 OR dt.pressure > $3 OR dt.cop < $4 OR dt.power > d.rated_power * $5)
		GROUP BY dt.device_id, d.rated_power
		HAVING COUNT(*) >= $6`),
		thresholds.SupplyTemp, thresholds.ReturnTemp, thresholds.Pressure,
		thresholds.COP, thresholds.PowerRatio, level1Threshold)
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
		if maxSupplyTemp > thresholds.SupplyTemp {
			checks = append(checks, thresholdCheck{"supply_temp", maxSupplyTemp, thresholds.SupplyTemp, fmt.Sprintf("供水温度持续超过%.1f°C，当前最大值: %.1f°C", thresholds.SupplyTemp, maxSupplyTemp)})
		}
		if maxReturnTemp > thresholds.ReturnTemp {
			checks = append(checks, thresholdCheck{"return_temp", maxReturnTemp, thresholds.ReturnTemp, fmt.Sprintf("回水温度持续超过%.1f°C，当前最大值: %.1f°C", thresholds.ReturnTemp, maxReturnTemp)})
		}
		if maxPressure > thresholds.Pressure {
			checks = append(checks, thresholdCheck{"pressure", maxPressure, thresholds.Pressure, fmt.Sprintf("压力持续超过%.2fMPa，当前最大值: %.2fMPa", thresholds.Pressure, maxPressure)})
		}
		if minCOP < thresholds.COP {
			checks = append(checks, thresholdCheck{"cop", minCOP, thresholds.COP, fmt.Sprintf("COP持续低于%.1f，当前最小值: %.2f", thresholds.COP, minCOP)})
		}
		if ratedPower > 0 && maxPower > ratedPower*thresholds.PowerRatio {
			checks = append(checks, thresholdCheck{"power", maxPower, ratedPower * thresholds.PowerRatio, fmt.Sprintf("功率持续超过额定功率%.1f倍(%.1fkW)，当前最大值: %.1fkW", thresholds.PowerRatio, ratedPower*thresholds.PowerRatio, maxPower)})
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
				DurationMinutes: float64(n.cfg.Alarm.Level1DurationMinutes),
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

	level2Window := n.cfg.Alarm.Level2DurationMinutes + 5
	level2Threshold := n.cfg.Alarm.Level2DurationMinutes / 5

	var pueCount int
	err = db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM pue_records WHERE time > NOW() - INTERVAL '%d minutes' AND pue_value > $1`, level2Window), n.cfg.PUE.PUEThreshold2).Scan(&pueCount)
	if err != nil {
		return nil, fmt.Errorf("query level 2 alarms: %w", err)
	}

	if pueCount >= level2Threshold {
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
					Message:         fmt.Sprintf("PUE持续超过%.1f超过%d分钟", n.cfg.PUE.PUEThreshold2, n.cfg.Alarm.Level2DurationMinutes),
					MetricName:      "pue",
					MetricValue:     latestPUE,
					Threshold:       n.cfg.PUE.PUEThreshold2,
					DurationMinutes: float64(n.cfg.Alarm.Level2DurationMinutes),
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
		if n.cfg.DingTalkWebhook != "" {
			if err := n.sendDingTalk(newAlarms[i]); err != nil {
				log.Printf("send dingtalk for alarm %d: %v", newAlarms[i].ID, err)
			}
		}
	}

	return newAlarms, nil
}

func (n *AlarmNotifier) sendDingTalk(alarm Alarm) error {
	if n.cfg.DingTalkWebhook == "" {
		return fmt.Errorf("dingtalk webhook not configured")
	}

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

	client := &http.Client{Timeout: time.Duration(n.cfg.Alarm.DingTalkTimeoutSeconds) * time.Second}
	resp, err := client.Post(n.cfg.DingTalkWebhook, "application/json", bytes.NewReader(body))
	if err != nil {
		n.mu.Lock()
		n.pendingDingTalk = append(n.pendingDingTalk, alarm)
		n.mu.Unlock()
		return fmt.Errorf("send dingtalk request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		n.mu.Lock()
		n.pendingDingTalk = append(n.pendingDingTalk, alarm)
		n.mu.Unlock()
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

func (n *AlarmNotifier) retryLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(n.cfg.Alarm.DingTalkRetryIntervalSec) * time.Second)
	defer ticker.Stop()

	retryCounts := make(map[int]int)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.mu.Lock()
			pending := n.pendingDingTalk
			n.pendingDingTalk = nil
			n.mu.Unlock()

			for _, alarm := range pending {
				retryCounts[alarm.ID]++
				if retryCounts[alarm.ID] > n.cfg.Alarm.DingTalkMaxRetries {
					delete(retryCounts, alarm.ID)
					continue
				}
				if err := n.sendDingTalk(alarm); err != nil {
					log.Printf("retry dingtalk for alarm %d (attempt %d/%d): %v", alarm.ID, retryCounts[alarm.ID], n.cfg.Alarm.DingTalkMaxRetries, err)
				} else {
					delete(retryCounts, alarm.ID)
				}
			}
		}
	}
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
