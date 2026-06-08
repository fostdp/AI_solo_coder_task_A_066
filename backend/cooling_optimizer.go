package backend

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"time"
)

type CoolingOptimizer struct {
	cfg      *Config
	triggerCh chan *PUERecord
	outCh    chan []OptimizationSuggestion
	stopCh   chan struct{}
}

func NewCoolingOptimizer(cfg *Config) *CoolingOptimizer {
	return &CoolingOptimizer{
		cfg:      cfg,
		triggerCh: make(chan *PUERecord, 8),
		outCh:    make(chan []OptimizationSuggestion, 8),
		stopCh:   make(chan struct{}),
	}
}

func (o *CoolingOptimizer) TriggerCh() chan<- *PUERecord {
	return o.triggerCh
}

func (o *CoolingOptimizer) Output() <-chan []OptimizationSuggestion {
	return o.outCh
}

func (o *CoolingOptimizer) PUETriggerThreshold() float64 {
	return o.cfg.PUE.PUEThreshold1
}

func (o *CoolingOptimizer) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(o.cfg.Optimization.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			suggestions, err := o.optimize()
			if err != nil {
				log.Printf("optimization error: %v", err)
				continue
			}
			if len(suggestions) > 0 {
				select {
				case o.outCh <- suggestions:
				default:
					log.Printf("optimization output channel full, dropping suggestions")
				}
			}
		case record := <-o.triggerCh:
			if record.PUEValue > o.cfg.PUE.PUEThreshold1 {
				suggestions, err := o.optimize()
				if err != nil {
					log.Printf("optimization error: %v", err)
					continue
				}
				if len(suggestions) > 0 {
					select {
					case o.outCh <- suggestions:
					default:
						log.Printf("optimization output channel full, dropping suggestions")
					}
				}
			}
		}
	}
}

func (o *CoolingOptimizer) optimize() ([]OptimizationSuggestion, error) {
	db := GetDB()

	rows, err := db.Query(`SELECT d.zone, AVG(t.setpoint_temp) as setpoint, AVG(t.return_temp) as current_temp, SUM(t.cooling_capacity) as allocated, SUM(t.power) as power FROM devices d JOIN (SELECT DISTINCT ON (device_id) * FROM device_telemetry ORDER BY device_id, time DESC) t ON d.id = t.device_id WHERE d.device_type IN ('precision_ac','cdu') GROUP BY d.zone`)
	if err != nil {
		return nil, fmt.Errorf("query zone cooling data: %w", err)
	}
	defer rows.Close()

	type zoneData struct {
		zone             string
		setpoint         float64
		currentTemp      float64
		allocatedCooling float64
		power            float64
		heatLoad         float64
		optimalCooling   float64
	}

	var zones []zoneData
	var totalHeatLoad float64
	var totalAllocatedCooling float64

	for rows.Next() {
		var z zoneData
		if err := rows.Scan(&z.zone, &z.setpoint, &z.currentTemp, &z.allocatedCooling, &z.power); err != nil {
			return nil, fmt.Errorf("scan zone data: %w", err)
		}
		z.heatLoad = z.allocatedCooling * (z.currentTemp - z.setpoint) / o.cfg.Optimization.HeatLoadDivisor
		if z.heatLoad < 0 {
			z.heatLoad = 0
		}
		totalHeatLoad += z.heatLoad
		totalAllocatedCooling += z.allocatedCooling
		zones = append(zones, z)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	for i := range zones {
		if totalHeatLoad > 0 {
			zones[i].optimalCooling = totalAllocatedCooling * zones[i].heatLoad / totalHeatLoad
		} else {
			zones[i].optimalCooling = zones[i].allocatedCooling
		}
	}

	var suggestions []OptimizationSuggestion

	for _, z := range zones {
		_, err := db.Exec(`INSERT INTO zone_cooling_demand (time, zone, setpoint_temp, current_temp, heat_load, allocated_cooling, optimal_cooling) VALUES (NOW(), $1, $2, $3, $4, $5, $6)`,
			z.zone, z.setpoint, z.currentTemp, z.heatLoad, z.allocatedCooling, z.optimalCooling)
		if err != nil {
			return nil, fmt.Errorf("insert zone cooling demand: %w", err)
		}

		var diff float64
		if z.allocatedCooling != 0 {
			diff = math.Abs(z.optimalCooling-z.allocatedCooling) / z.allocatedCooling
		}

		if diff > o.cfg.Optimization.DiffThreshold {
			powerDiff := z.power * math.Abs(z.optimalCooling-z.allocatedCooling) / z.allocatedCooling
			expectedSaving := powerDiff * o.cfg.Optimization.SavingRatio
			reason := fmt.Sprintf("Zone %s: adjust cooling from %.1f to %.1f (heat load %.1f)", z.zone, z.allocatedCooling, z.optimalCooling, z.heatLoad)

			var id int
			err := db.QueryRow(`INSERT INTO optimization_suggestions (time, suggestion_type, zone, current_value, suggested_value, expected_saving, reason, status) VALUES (NOW(), $1, $2, $3, $4, $5, $6, $7) RETURNING id`,
				"cooling_redistribution", z.zone, z.allocatedCooling, z.optimalCooling, expectedSaving, reason, "pending").Scan(&id)
			if err != nil {
				return nil, fmt.Errorf("insert optimization suggestion: %w", err)
			}

			suggestions = append(suggestions, OptimizationSuggestion{
				ID:             id,
				SuggestionType: "cooling_redistribution",
				Zone:           z.zone,
				CurrentValue:   z.allocatedCooling,
				SuggestedValue: z.optimalCooling,
				ExpectedSaving: expectedSaving,
				Reason:         reason,
				Status:         "pending",
			})
		}
	}

	return suggestions, nil
}

func GetZoneCoolingDemands() ([]ZoneCoolingDemand, error) {
	db := GetDB()
	rows, err := db.Query(`SELECT time, zone, setpoint_temp, current_temp, heat_load, allocated_cooling, optimal_cooling FROM zone_cooling_demand WHERE time > NOW() - INTERVAL '24 hours' ORDER BY time ASC`)
	if err != nil {
		return nil, fmt.Errorf("query zone cooling demands: %w", err)
	}
	defer rows.Close()

	var result []ZoneCoolingDemand
	for rows.Next() {
		var z ZoneCoolingDemand
		if err := rows.Scan(&z.Time, &z.Zone, &z.SetpointTemp, &z.CurrentTemp, &z.HeatLoad, &z.AllocatedCooling, &z.OptimalCooling); err != nil {
			return nil, fmt.Errorf("scan zone cooling demand: %w", err)
		}
		result = append(result, z)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}

func GetSankeyData() (*SankeyData, error) {
	db := GetDB()
	rows, err := db.Query(`SELECT d.zone, d.device_type, SUM(t.cooling_capacity) as total_cooling, SUM(t.power) as total_power FROM devices d JOIN (SELECT DISTINCT ON (device_id) * FROM device_telemetry ORDER BY device_id, time DESC) t ON d.id = t.device_id WHERE d.device_type IN ('precision_ac','cdu') GROUP BY d.zone, d.device_type`)
	if err != nil {
		return nil, fmt.Errorf("query sankey data: %w", err)
	}
	defer rows.Close()

	type zoneTypeInfo struct {
		zone         string
		deviceType   string
		totalCooling float64
		totalPower   float64
	}

	var entries []zoneTypeInfo
	zoneCooling := make(map[string]float64)
	zonePower := make(map[string]float64)

	for rows.Next() {
		var e zoneTypeInfo
		if err := rows.Scan(&e.zone, &e.deviceType, &e.totalCooling, &e.totalPower); err != nil {
			return nil, fmt.Errorf("scan sankey entry: %w", err)
		}
		entries = append(entries, e)
		zoneCooling[e.zone] += e.totalCooling
		zonePower[e.zone] += e.totalPower
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	var totalCoolingSupply float64
	for _, v := range zoneCooling {
		totalCoolingSupply += v
	}

	data := &SankeyData{}

	data.Nodes = append(data.Nodes, SankeyNode{
		Name:  "总冷量供给",
		Value: totalCoolingSupply,
		Color: "blue",
	})

	for zone, cooling := range zoneCooling {
		color := "red"
		power := zonePower[zone]
		if power > 0 {
			cop := cooling / power
			color = COPColor(cop)
		}
		data.Nodes = append(data.Nodes, SankeyNode{
			Name:  zone,
			Value: cooling,
			Color: color,
		})
		data.Links = append(data.Links, SankeyLink{
			Source: "总冷量供给",
			Target: zone,
			Value:  cooling,
		})
	}

	for _, e := range entries {
		nodeName := e.zone + "_" + e.deviceType
		color := "red"
		if e.totalPower > 0 {
			cop := e.totalCooling / e.totalPower
			color = COPColor(cop)
		}
		data.Nodes = append(data.Nodes, SankeyNode{
			Name:  nodeName,
			Value: e.totalCooling,
			Color: color,
		})
		data.Links = append(data.Links, SankeyLink{
			Source: e.zone,
			Target: nodeName,
			Value:  e.totalCooling,
		})
	}

	return data, nil
}

func GetEfficiencyRanking() ([]EfficiencyRanking, error) {
	db := GetDB()
	rows, err := db.Query(`SELECT d.id, d.device_code, d.device_name, d.device_type, AVG(t.cop) as avg_cop, AVG(t.power) as avg_power FROM devices d JOIN device_telemetry t ON d.id = t.device_id WHERE t.time > NOW() - INTERVAL '1 hour' GROUP BY d.id, d.device_code, d.device_name, d.device_type ORDER BY avg_cop ASC`)
	if err != nil {
		return nil, fmt.Errorf("query efficiency ranking: %w", err)
	}
	defer rows.Close()

	var result []EfficiencyRanking
	for rows.Next() {
		var r EfficiencyRanking
		if err := rows.Scan(&r.DeviceID, &r.DeviceCode, &r.DeviceName, &r.DeviceType, &r.AvgCOP, &r.AvgPower); err != nil {
			return nil, fmt.Errorf("scan efficiency ranking: %w", err)
		}
		r.COPColor = COPColor(r.AvgCOP)
		result = append(result, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}
