package backend

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/lib/pq"
)

type ModbusGateway struct {
	cfg         *Config
	mu          sync.Mutex
	conn        net.Conn
	lastUsed    time.Time
	idleLimit   time.Duration
	maxRetries  int
	dialTimeout time.Duration
	readTimeout time.Duration
	outCh       chan []DeviceTelemetry
	stopCh      chan struct{}
}

func NewModbusGateway(cfg *Config) *ModbusGateway {
	return &ModbusGateway{
		cfg:         cfg,
		idleLimit:   time.Duration(cfg.Modbus.IdleTimeoutSeconds) * time.Second,
		maxRetries:  cfg.Modbus.MaxRetries,
		dialTimeout: time.Duration(cfg.Modbus.DialTimeoutSeconds) * time.Second,
		readTimeout: time.Duration(cfg.Modbus.ReadTimeoutSeconds) * time.Second,
		outCh:       make(chan []DeviceTelemetry, 8),
		stopCh:      make(chan struct{}),
	}
}

func (g *ModbusGateway) Output() <-chan []DeviceTelemetry {
	return g.outCh
}

func (g *ModbusGateway) Run(ctx context.Context) {
	interval := time.Duration(g.cfg.Modbus.CollectIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	go g.cleanupLoop(ctx)

	for {
		select {
		case <-ctx.Done():
			g.invalidateConnection()
			return
		case <-ticker.C:
			data, err := g.collect()
			if err != nil {
				log.Println("modbus collect error:", err)
				continue
			}
			if data != nil {
				select {
				case g.outCh <- data:
				default:
					log.Println("modbus output channel full, dropping batch")
				}
			}
		}
	}
}

func (g *ModbusGateway) collect() ([]DeviceTelemetry, error) {
	type deviceRange struct {
		startSlave int
		count      int
	}

	deviceRanges := []deviceRange{
		{1, 8},
		{9, 12},
		{21, 80},
		{101, 20},
	}

	for attempt := 0; attempt < g.maxRetries; attempt++ {
		conn, err := g.getConnection()
		if err != nil {
			g.invalidateConnection()
			if attempt == g.maxRetries-1 {
				return nil, fmt.Errorf("get connection after %d retries: %w", g.maxRetries, err)
			}
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			log.Printf("modbus connection attempt %d failed, retrying in %v: %v", attempt+1, backoff, err)
			time.Sleep(backoff)
			continue
		}

		var results []DeviceTelemetry
		var transactionID uint16
		var readErr error

		for _, dr := range deviceRanges {
			for i := 0; i < dr.count; i++ {
				slaveID := byte(dr.startSlave + i)
				transactionID++

				req := make([]byte, 12)
				binary.BigEndian.PutUint16(req[0:2], transactionID)
				binary.BigEndian.PutUint16(req[2:4], 0x0000)
				binary.BigEndian.PutUint16(req[4:6], 6)
				req[6] = slaveID
				req[7] = 0x03
				binary.BigEndian.PutUint16(req[8:10], 0x0000)
				binary.BigEndian.PutUint16(req[10:12], 0x000A)

				if _, err := conn.Write(req); err != nil {
					readErr = fmt.Errorf("write modbus request slave %d: %w", slaveID, err)
					break
				}

				header := make([]byte, 9)
				if _, err := conn.Read(header); err != nil {
					readErr = fmt.Errorf("read modbus header slave %d: %w", slaveID, err)
					break
				}

				byteCount := int(header[8])
				data := make([]byte, byteCount)
				if _, err := conn.Read(data); err != nil {
					readErr = fmt.Errorf("read modbus data slave %d: %w", slaveID, err)
					break
				}

				readRegister := func(offset int) uint16 {
					return binary.BigEndian.Uint16(data[offset*2 : offset*2+2])
				}

				t := DeviceTelemetry{
					Time:            time.Now().UTC(),
					DeviceID:        int(slaveID),
					SupplyTemp:      float64(readRegister(0)) / 10.0,
					ReturnTemp:      float64(readRegister(1)) / 10.0,
					FlowRate:        float64(readRegister(2)) / 10.0,
					Power:           float64(readRegister(3)) / 10.0,
					Pressure:        float64(readRegister(4)) / 10.0,
					COP:             float64(readRegister(5)) / 100.0,
					CoolingCapacity: float64(readRegister(6)) / 10.0,
					SetpointTemp:    float64(readRegister(7)) / 10.0,
					Status:          int(readRegister(8)),
				}
				results = append(results, t)
			}
			if readErr != nil {
				break
			}
		}

		if readErr != nil {
			g.invalidateConnection()
			if attempt == g.maxRetries-1 {
				return nil, readErr
			}
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			log.Printf("modbus read attempt %d failed, retrying in %v: %v", attempt+1, backoff, readErr)
			time.Sleep(backoff)
			continue
		}

		return results, nil
	}

	return nil, fmt.Errorf("modbus collect failed after %d retries", g.maxRetries)
}

func (g *ModbusGateway) getConnection() (net.Conn, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.conn != nil {
		if time.Since(g.lastUsed) > g.idleLimit {
			g.conn.Close()
			g.conn = nil
		} else {
			oneByte := make([]byte, 1)
			g.conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
			_, err := g.conn.Read(oneByte)
			g.conn.SetReadDeadline(time.Time{})
			if err == nil || isTimeoutError2(err) {
				g.conn.SetDeadline(time.Now().Add(g.readTimeout))
				g.lastUsed = time.Now()
				return g.conn, nil
			}
			g.conn.Close()
			g.conn = nil
		}
	}

	addr := fmt.Sprintf("%s:%s", g.cfg.ModbusHost, g.cfg.ModbusPort)
	conn, err := net.DialTimeout("tcp", addr, g.dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("dial modbus %s: %w", addr, err)
	}
	conn.SetDeadline(time.Now().Add(g.readTimeout))
	g.conn = conn
	g.lastUsed = time.Now()
	return conn, nil
}

func (g *ModbusGateway) invalidateConnection() {
	g.mu.Lock()
	if g.conn != nil {
		g.conn.Close()
		g.conn = nil
	}
	g.mu.Unlock()
}

func (g *ModbusGateway) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.mu.Lock()
			if g.conn != nil && time.Since(g.lastUsed) > g.idleLimit {
				g.conn.Close()
				g.conn = nil
			}
			g.mu.Unlock()
		}
	}
}

func isTimeoutError2(err error) bool {
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout()
	}
	return false
}

func InsertTelemetry(batch []DeviceTelemetry) error {
	db := GetDB()
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(pq.CopyIn("device_telemetry",
		"time", "device_id", "supply_temp", "return_temp", "flow_rate",
		"power", "pressure", "cop", "cooling_capacity", "setpoint_temp", "status"))
	if err != nil {
		return fmt.Errorf("prepare copyin: %w", err)
	}
	defer stmt.Close()

	for _, t := range batch {
		_, err := stmt.Exec(t.Time, t.DeviceID, t.SupplyTemp, t.ReturnTemp, t.FlowRate,
			t.Power, t.Pressure, t.COP, t.CoolingCapacity, t.SetpointTemp, t.Status)
		if err != nil {
			return fmt.Errorf("exec copyin: %w", err)
		}
	}

	_, err = stmt.Exec()
	if err != nil {
		return fmt.Errorf("flush copyin: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func GetLatestTelemetry() (map[int]DeviceTelemetry, error) {
	db := GetDB()
	rows, err := db.Query(`SELECT DISTINCT ON (device_id) time, device_id, supply_temp, return_temp, flow_rate, power, pressure, cop, cooling_capacity, setpoint_temp, status FROM device_telemetry ORDER BY device_id, time DESC`)
	if err != nil {
		return nil, fmt.Errorf("query latest telemetry: %w", err)
	}
	defer rows.Close()

	result := make(map[int]DeviceTelemetry)
	for rows.Next() {
		var t DeviceTelemetry
		if err := rows.Scan(&t.Time, &t.DeviceID, &t.SupplyTemp, &t.ReturnTemp, &t.FlowRate,
			&t.Power, &t.Pressure, &t.COP, &t.CoolingCapacity, &t.SetpointTemp, &t.Status); err != nil {
			return nil, fmt.Errorf("scan telemetry: %w", err)
		}
		result[t.DeviceID] = t
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}

func GetDeviceTelemetryHistory(deviceID int, hours int) ([]DeviceTelemetry, error) {
	db := GetDB()
	query := fmt.Sprintf(`SELECT time, device_id, supply_temp, return_temp, flow_rate, power, pressure, cop, cooling_capacity, setpoint_temp, status FROM device_telemetry WHERE device_id = $1 AND time > NOW() - INTERVAL '%d hours' ORDER BY time ASC`, hours)
	rows, err := db.Query(query, deviceID)
	if err != nil {
		return nil, fmt.Errorf("query telemetry history: %w", err)
	}
	defer rows.Close()

	var result []DeviceTelemetry
	for rows.Next() {
		var t DeviceTelemetry
		if err := rows.Scan(&t.Time, &t.DeviceID, &t.SupplyTemp, &t.ReturnTemp, &t.FlowRate,
			&t.Power, &t.Pressure, &t.COP, &t.CoolingCapacity, &t.SetpointTemp, &t.Status); err != nil {
			return nil, fmt.Errorf("scan telemetry: %w", err)
		}
		result = append(result, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}

func COPColor(cop float64) string {
	if cop > 6 {
		return "green"
	}
	if cop >= 4 {
		return "yellow"
	}
	return "red"
}

func GetDeviceLatestStates() ([]DeviceLatestState, error) {
	db := GetDB()
	rows, err := db.Query(`SELECT d.id, d.device_code, d.device_name, d.device_type, d.zone, d.rated_power, d.rated_cooling_capacity, t.time, t.device_id, t.supply_temp, t.return_temp, t.flow_rate, t.power, t.pressure, t.cop, t.cooling_capacity, t.setpoint_temp, t.status FROM devices d LEFT JOIN (SELECT DISTINCT ON (device_id) time, device_id, supply_temp, return_temp, flow_rate, power, pressure, cop, cooling_capacity, setpoint_temp, status FROM device_telemetry ORDER BY device_id, time DESC) t ON d.id = t.device_id`)
	if err != nil {
		return nil, fmt.Errorf("query device latest states: %w", err)
	}
	defer rows.Close()

	var result []DeviceLatestState
	for rows.Next() {
		var d Device
		var tTime sql.NullTime
		var tDeviceID sql.NullInt64
		var tSupplyTemp sql.NullFloat64
		var tReturnTemp sql.NullFloat64
		var tFlowRate sql.NullFloat64
		var tPower sql.NullFloat64
		var tPressure sql.NullFloat64
		var tCOP sql.NullFloat64
		var tCoolingCapacity sql.NullFloat64
		var tSetpointTemp sql.NullFloat64
		var tStatus sql.NullInt64

		if err := rows.Scan(&d.ID, &d.Code, &d.Name, &d.Type, &d.Zone, &d.RatedPower, &d.RatedCoolingCapacity,
			&tTime, &tDeviceID, &tSupplyTemp, &tReturnTemp, &tFlowRate,
			&tPower, &tPressure, &tCOP, &tCoolingCapacity, &tSetpointTemp, &tStatus); err != nil {
			return nil, fmt.Errorf("scan device latest state: %w", err)
		}

		s := DeviceLatestState{Device: d}
		if tTime.Valid {
			s.Telemetry = DeviceTelemetry{
				Time:            tTime.Time,
				DeviceID:        int(tDeviceID.Int64),
				SupplyTemp:      tSupplyTemp.Float64,
				ReturnTemp:      tReturnTemp.Float64,
				FlowRate:        tFlowRate.Float64,
				Power:           tPower.Float64,
				Pressure:        tPressure.Float64,
				COP:             tCOP.Float64,
				CoolingCapacity: tCoolingCapacity.Float64,
				SetpointTemp:    tSetpointTemp.Float64,
				Status:          int(tStatus.Int64),
			}
			s.COPColor = COPColor(s.Telemetry.COP)
		}

		result = append(result, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}
