package backend

import (
	"database/sql"
	"fmt"
)

func CalculatePUE(cfg *Config) ([]PUERecord, error) {
	db := GetDB()

	var coolingPower float64
	err := db.QueryRow(`SELECT COALESCE(SUM(dt.power), 0) FROM device_telemetry dt JOIN devices d ON dt.device_id = d.id WHERE dt.time > NOW() - INTERVAL '5 minutes' AND d.device_type IN ('chiller', 'cooling_tower', 'precision_ac', 'cdu')`).Scan(&coolingPower)
	if err != nil {
		return nil, fmt.Errorf("query cooling power: %w", err)
	}

	var itPower float64
	err = db.QueryRow(`SELECT COALESCE(AVG(total_it_power), $1) FROM it_power_readings WHERE time > NOW() - INTERVAL '5 minutes'`, cfg.ITDefaultPower).Scan(&itPower)
	if err != nil {
		return nil, fmt.Errorf("query it power: %w", err)
	}

	if itPower == 0 {
		itPower = cfg.ITDefaultPower
	}

	totalPower := itPower + coolingPower
	pueValue := totalPower / itPower

	_, err = db.Exec(`INSERT INTO pue_records (time, it_power, cooling_power, total_power, pue_value) VALUES (NOW(), $1, $2, $3, $4)`, itPower, coolingPower, totalPower, pueValue)
	if err != nil {
		return nil, fmt.Errorf("insert pue record: %w", err)
	}

	if pueValue > cfg.PUEThreshold1 {
		TriggerOptimization()
	}

	var record PUERecord
	err = db.QueryRow(`SELECT time, it_power, cooling_power, total_power, pue_value FROM pue_records ORDER BY time DESC LIMIT 1`).Scan(&record.Time, &record.ITPower, &record.CoolingPower, &record.TotalPower, &record.PUEValue)
	if err != nil {
		return nil, fmt.Errorf("query new pue record: %w", err)
	}

	return []PUERecord{record}, nil
}

func GetPUETrend(hours int) ([]PUERecord, error) {
	db := GetDB()
	query := fmt.Sprintf(`SELECT time, it_power, cooling_power, total_power, pue_value FROM pue_records WHERE time > NOW() - INTERVAL '%d hours' ORDER BY time ASC`, hours)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query pue trend: %w", err)
	}
	defer rows.Close()

	var result []PUERecord
	for rows.Next() {
		var r PUERecord
		if err := rows.Scan(&r.Time, &r.ITPower, &r.CoolingPower, &r.TotalPower, &r.PUEValue); err != nil {
			return nil, fmt.Errorf("scan pue record: %w", err)
		}
		result = append(result, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}

func GetCurrentPUE() (*PUERecord, error) {
	db := GetDB()
	var record PUERecord
	err := db.QueryRow(`SELECT time, it_power, cooling_power, total_power, pue_value FROM pue_records ORDER BY time DESC LIMIT 1`).Scan(&record.Time, &record.ITPower, &record.CoolingPower, &record.TotalPower, &record.PUEValue)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query current pue: %w", err)
	}

	return &record, nil
}

func InsertITPower(power float64) error {
	db := GetDB()
	_, err := db.Exec(`INSERT INTO it_power_readings (time, total_it_power) VALUES (NOW(), $1)`, power)
	if err != nil {
		return fmt.Errorf("insert it power: %w", err)
	}
	return nil
}

func GetCoolingTotalPower() (float64, error) {
	db := GetDB()
	var total float64
	err := db.QueryRow(`SELECT COALESCE(SUM(power), 0) FROM (SELECT DISTINCT ON (dt.device_id) dt.power FROM device_telemetry dt JOIN devices d ON dt.device_id = d.id WHERE d.device_type IN ('chiller', 'cooling_tower', 'precision_ac', 'cdu') ORDER BY dt.device_id, dt.time DESC) sub`).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("query cooling total power: %w", err)
	}
	return total, nil
}
