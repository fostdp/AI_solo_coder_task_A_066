CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS devices (
    id SERIAL PRIMARY KEY,
    device_code VARCHAR(32) NOT NULL UNIQUE,
    device_name VARCHAR(128) NOT NULL,
    device_type VARCHAR(32) NOT NULL,
    zone VARCHAR(64),
    rated_power FLOAT,
    rated_cooling_capacity FLOAT,
    metadata JSONB DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS device_telemetry (
    time TIMESTAMPTZ NOT NULL,
    device_id INT NOT NULL REFERENCES devices(id),
    supply_temp FLOAT,
    return_temp FLOAT,
    flow_rate FLOAT,
    power FLOAT,
    pressure FLOAT,
    cop FLOAT,
    cooling_capacity FLOAT,
    setpoint_temp FLOAT,
    status INT DEFAULT 1
);
SELECT create_hypertable('device_telemetry', 'time', chunk_time_interval => INTERVAL '1 day', migrate_data => TRUE);

CREATE INDEX idx_telemetry_device_time ON device_telemetry (device_id, time DESC);

CREATE TABLE IF NOT EXISTS pue_records (
    time TIMESTAMPTZ NOT NULL,
    it_power FLOAT NOT NULL,
    cooling_power FLOAT NOT NULL,
    total_power FLOAT NOT NULL,
    pue_value FLOAT NOT NULL
);
SELECT create_hypertable('pue_records', 'time', chunk_time_interval => INTERVAL '1 day', migrate_data => TRUE);

CREATE TABLE IF NOT EXISTS zone_cooling_demand (
    time TIMESTAMPTZ NOT NULL,
    zone VARCHAR(64) NOT NULL,
    setpoint_temp FLOAT,
    current_temp FLOAT,
    heat_load FLOAT,
    allocated_cooling FLOAT,
    optimal_cooling FLOAT
);
SELECT create_hypertable('zone_cooling_demand', 'time', chunk_time_interval => INTERVAL '1 day', migrate_data => TRUE);

CREATE TABLE IF NOT EXISTS optimization_suggestions (
    id SERIAL,
    time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    suggestion_type VARCHAR(64) NOT NULL,
    device_id INT REFERENCES devices(id),
    zone VARCHAR(64),
    current_value FLOAT,
    suggested_value FLOAT,
    expected_saving FLOAT,
    reason TEXT,
    status VARCHAR(16) DEFAULT 'pending'
);
SELECT create_hypertable('optimization_suggestions', 'time', chunk_time_interval => INTERVAL '7 days', migrate_data => TRUE);

CREATE TABLE IF NOT EXISTS alarms (
    id SERIAL PRIMARY KEY,
    time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    alarm_level INT NOT NULL,
    device_id INT REFERENCES devices(id),
    alarm_type VARCHAR(64) NOT NULL,
    message TEXT NOT NULL,
    metric_name VARCHAR(64),
    metric_value FLOAT,
    threshold FLOAT,
    duration_minutes FLOAT,
    acknowledged BOOLEAN DEFAULT FALSE,
    dingtalk_sent BOOLEAN DEFAULT FALSE
);
CREATE INDEX idx_alarms_time ON alarms (time DESC);
CREATE INDEX idx_alarms_device ON alarms (device_id, time DESC);
CREATE INDEX idx_alarms_level ON alarms (alarm_level, acknowledged, time DESC);

CREATE TABLE IF NOT EXISTS it_power_readings (
    time TIMESTAMPTZ NOT NULL,
    total_it_power FLOAT NOT NULL
);
SELECT create_hypertable('it_power_readings', 'time', chunk_time_interval => INTERVAL '1 day', migrate_data => TRUE);

INSERT INTO devices (device_code, device_name, device_type, zone, rated_power, rated_cooling_capacity) VALUES
('CHU-001', 'Centrifugal Chiller #1', 'chiller', 'chiller_plant', 1200, 4200),
('CHU-002', 'Centrifugal Chiller #2', 'chiller', 'chiller_plant', 1200, 4200),
('CHU-003', 'Centrifugal Chiller #3', 'chiller', 'chiller_plant', 1200, 4200),
('CHU-004', 'Centrifugal Chiller #4', 'chiller', 'chiller_plant', 1200, 4200),
('CHU-005', 'Centrifugal Chiller #5', 'chiller', 'chiller_plant', 1200, 4200),
('CHU-006', 'Centrifugal Chiller #6', 'chiller', 'chiller_plant', 1200, 4200),
('CHU-007', 'Centrifugal Chiller #7', 'chiller', 'chiller_plant', 1200, 4200),
('CHU-008', 'Centrifugal Chiller #8', 'chiller', 'chiller_plant', 1200, 4200),
('CT-001', 'Cooling Tower #1', 'cooling_tower', 'outdoor', 75, 3500),
('CT-002', 'Cooling Tower #2', 'cooling_tower', 'outdoor', 75, 3500),
('CT-003', 'Cooling Tower #3', 'cooling_tower', 'outdoor', 75, 3500),
('CT-004', 'Cooling Tower #4', 'cooling_tower', 'outdoor', 75, 3500),
('CT-005', 'Cooling Tower #5', 'cooling_tower', 'outdoor', 75, 3500),
('CT-006', 'Cooling Tower #6', 'cooling_tower', 'outdoor', 75, 3500),
('CT-007', 'Cooling Tower #7', 'cooling_tower', 'outdoor', 75, 3500),
('CT-008', 'Cooling Tower #8', 'cooling_tower', 'outdoor', 75, 3500),
('CT-009', 'Cooling Tower #9', 'cooling_tower', 'outdoor', 75, 3500),
('CT-010', 'Cooling Tower #10', 'cooling_tower', 'outdoor', 75, 3500),
('CT-011', 'Cooling Tower #11', 'cooling_tower', 'outdoor', 75, 3500),
('CT-012', 'Cooling Tower #12', 'cooling_tower', 'outdoor', 75, 3500),
('PAC-001', 'Precision AC #1', 'precision_ac', 'zone_A', 45, 150),
('PAC-002', 'Precision AC #2', 'precision_ac', 'zone_A', 45, 150),
('PAC-003', 'Precision AC #3', 'precision_ac', 'zone_A', 45, 150),
('PAC-004', 'Precision AC #4', 'precision_ac', 'zone_A', 45, 150),
('PAC-005', 'Precision AC #5', 'precision_ac', 'zone_A', 45, 150),
('PAC-006', 'Precision AC #6', 'precision_ac', 'zone_A', 45, 150),
('PAC-007', 'Precision AC #7', 'precision_ac', 'zone_A', 45, 150),
('PAC-008', 'Precision AC #8', 'precision_ac', 'zone_A', 45, 150),
('PAC-009', 'Precision AC #9', 'precision_ac', 'zone_A', 45, 150),
('PAC-010', 'Precision AC #10', 'precision_ac', 'zone_A', 45, 150),
('PAC-011', 'Precision AC #11', 'precision_ac', 'zone_B', 45, 150),
('PAC-012', 'Precision AC #12', 'precision_ac', 'zone_B', 45, 150),
('PAC-013', 'Precision AC #13', 'precision_ac', 'zone_B', 45, 150),
('PAC-014', 'Precision AC #14', 'precision_ac', 'zone_B', 45, 150),
('PAC-015', 'Precision AC #15', 'precision_ac', 'zone_B', 45, 150),
('PAC-016', 'Precision AC #16', 'precision_ac', 'zone_B', 45, 150),
('PAC-017', 'Precision AC #17', 'precision_ac', 'zone_B', 45, 150),
('PAC-018', 'Precision AC #18', 'precision_ac', 'zone_B', 45, 150),
('PAC-019', 'Precision AC #19', 'precision_ac', 'zone_B', 45, 150),
('PAC-020', 'Precision AC #20', 'precision_ac', 'zone_B', 45, 150),
('PAC-021', 'Precision AC #21', 'precision_ac', 'zone_C', 45, 150),
('PAC-022', 'Precision AC #22', 'precision_ac', 'zone_C', 45, 150),
('PAC-023', 'Precision AC #23', 'precision_ac', 'zone_C', 45, 150),
('PAC-024', 'Precision AC #24', 'precision_ac', 'zone_C', 45, 150),
('PAC-025', 'Precision AC #25', 'precision_ac', 'zone_C', 45, 150),
('PAC-026', 'Precision AC #26', 'precision_ac', 'zone_C', 45, 150),
('PAC-027', 'Precision AC #27', 'precision_ac', 'zone_C', 45, 150),
('PAC-028', 'Precision AC #28', 'precision_ac', 'zone_C', 45, 150),
('PAC-029', 'Precision AC #29', 'precision_ac', 'zone_C', 45, 150),
('PAC-030', 'Precision AC #30', 'precision_ac', 'zone_C', 45, 150),
('PAC-031', 'Precision AC #31', 'precision_ac', 'zone_D', 45, 150),
('PAC-032', 'Precision AC #32', 'precision_ac', 'zone_D', 45, 150),
('PAC-033', 'Precision AC #33', 'precision_ac', 'zone_D', 45, 150),
('PAC-034', 'Precision AC #34', 'precision_ac', 'zone_D', 45, 150),
('PAC-035', 'Precision AC #35', 'precision_ac', 'zone_D', 45, 150),
('PAC-036', 'Precision AC #36', 'precision_ac', 'zone_D', 45, 150),
('PAC-037', 'Precision AC #37', 'precision_ac', 'zone_D', 45, 150),
('PAC-038', 'Precision AC #38', 'precision_ac', 'zone_D', 45, 150),
('PAC-039', 'Precision AC #39', 'precision_ac', 'zone_D', 45, 150),
('PAC-040', 'Precision AC #40', 'precision_ac', 'zone_D', 45, 150),
('PAC-041', 'Precision AC #41', 'precision_ac', 'zone_E', 45, 150),
('PAC-042', 'Precision AC #42', 'precision_ac', 'zone_E', 45, 150),
('PAC-043', 'Precision AC #43', 'precision_ac', 'zone_E', 45, 150),
('PAC-044', 'Precision AC #44', 'precision_ac', 'zone_E', 45, 150),
('PAC-045', 'Precision AC #45', 'precision_ac', 'zone_E', 45, 150),
('PAC-046', 'Precision AC #46', 'precision_ac', 'zone_E', 45, 150),
('PAC-047', 'Precision AC #47', 'precision_ac', 'zone_E', 45, 150),
('PAC-048', 'Precision AC #48', 'precision_ac', 'zone_E', 45, 150),
('PAC-049', 'Precision AC #49', 'precision_ac', 'zone_E', 45, 150),
('PAC-050', 'Precision AC #50', 'precision_ac', 'zone_E', 45, 150),
('PAC-051', 'Precision AC #51', 'precision_ac', 'zone_F', 45, 150),
('PAC-052', 'Precision AC #52', 'precision_ac', 'zone_F', 45, 150),
('PAC-053', 'Precision AC #53', 'precision_ac', 'zone_F', 45, 150),
('PAC-054', 'Precision AC #54', 'precision_ac', 'zone_F', 45, 150),
('PAC-055', 'Precision AC #55', 'precision_ac', 'zone_F', 45, 150),
('PAC-056', 'Precision AC #56', 'precision_ac', 'zone_F', 45, 150),
('PAC-057', 'Precision AC #57', 'precision_ac', 'zone_F', 45, 150),
('PAC-058', 'Precision AC #58', 'precision_ac', 'zone_F', 45, 150),
('PAC-059', 'Precision AC #59', 'precision_ac', 'zone_F', 45, 150),
('PAC-060', 'Precision AC #60', 'precision_ac', 'zone_F', 45, 150),
('PAC-061', 'Precision AC #61', 'precision_ac', 'zone_G', 45, 150),
('PAC-062', 'Precision AC #62', 'precision_ac', 'zone_G', 45, 150),
('PAC-063', 'Precision AC #63', 'precision_ac', 'zone_G', 45, 150),
('PAC-064', 'Precision AC #64', 'precision_ac', 'zone_G', 45, 150),
('PAC-065', 'Precision AC #65', 'precision_ac', 'zone_G', 45, 150),
('PAC-066', 'Precision AC #66', 'precision_ac', 'zone_G', 45, 150),
('PAC-067', 'Precision AC #67', 'precision_ac', 'zone_G', 45, 150),
('PAC-068', 'Precision AC #68', 'precision_ac', 'zone_G', 45, 150),
('PAC-069', 'Precision AC #69', 'precision_ac', 'zone_G', 45, 150),
('PAC-070', 'Precision AC #70', 'precision_ac', 'zone_G', 45, 150),
('PAC-071', 'Precision AC #71', 'precision_ac', 'zone_H', 45, 150),
('PAC-072', 'Precision AC #72', 'precision_ac', 'zone_H', 45, 150),
('PAC-073', 'Precision AC #73', 'precision_ac', 'zone_H', 45, 150),
('PAC-074', 'Precision AC #74', 'precision_ac', 'zone_H', 45, 150),
('PAC-075', 'Precision AC #75', 'precision_ac', 'zone_H', 45, 150),
('PAC-076', 'Precision AC #76', 'precision_ac', 'zone_H', 45, 150),
('PAC-077', 'Precision AC #77', 'precision_ac', 'zone_H', 45, 150),
('PAC-078', 'Precision AC #78', 'precision_ac', 'zone_H', 45, 150),
('PAC-079', 'Precision AC #79', 'precision_ac', 'zone_H', 45, 150),
('PAC-080', 'Precision AC #80', 'precision_ac', 'zone_H', 45, 150),
('CDU-001', 'Liquid Cooling CDU #1', 'cdu', 'zone_A', 30, 200),
('CDU-002', 'Liquid Cooling CDU #2', 'cdu', 'zone_A', 30, 200),
('CDU-003', 'Liquid Cooling CDU #3', 'cdu', 'zone_B', 30, 200),
('CDU-004', 'Liquid Cooling CDU #4', 'cdu', 'zone_B', 30, 200),
('CDU-005', 'Liquid Cooling CDU #5', 'cdu', 'zone_C', 30, 200),
('CDU-006', 'Liquid Cooling CDU #6', 'cdu', 'zone_C', 30, 200),
('CDU-007', 'Liquid Cooling CDU #7', 'cdu', 'zone_D', 30, 200),
('CDU-008', 'Liquid Cooling CDU #8', 'cdu', 'zone_D', 30, 200),
('CDU-009', 'Liquid Cooling CDU #9', 'cdu', 'zone_E', 30, 200),
('CDU-010', 'Liquid Cooling CDU #10', 'cdu', 'zone_E', 30, 200),
('CDU-011', 'Liquid Cooling CDU #11', 'cdu', 'zone_F', 30, 200),
('CDU-012', 'Liquid Cooling CDU #12', 'cdu', 'zone_F', 30, 200),
('CDU-013', 'Liquid Cooling CDU #13', 'cdu', 'zone_G', 30, 200),
('CDU-014', 'Liquid Cooling CDU #14', 'cdu', 'zone_G', 30, 200),
('CDU-015', 'Liquid Cooling CDU #15', 'cdu', 'zone_H', 30, 200),
('CDU-016', 'Liquid Cooling CDU #16', 'cdu', 'zone_H', 30, 200),
('CDU-017', 'Liquid Cooling CDU #17', 'cdu', 'liquid_cold_aisle', 30, 200),
('CDU-018', 'Liquid Cooling CDU #18', 'cdu', 'liquid_cold_aisle', 30, 200),
('CDU-019', 'Liquid Cooling CDU #19', 'cdu', 'liquid_cold_aisle', 30, 200),
('CDU-020', 'Liquid Cooling CDU #20', 'cdu', 'liquid_cold_aisle', 30, 200);

CREATE OR REPLACE FUNCTION continuous_aggregates_refresh()
RETURNS VOID AS $$
BEGIN
    PERFORM refresh_continuous_aggregate('device_telemetry_5min', NULL, NULL);
    PERFORM refresh_continuous_aggregate('device_telemetry_1hour', NULL, NULL);
END;
$$ LANGUAGE plpgsql;

CREATE MATERIALIZED VIEW device_telemetry_5min
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('5 minutes', time) AS bucket,
    device_id,
    AVG(supply_temp) AS avg_supply_temp,
    AVG(return_temp) AS avg_return_temp,
    AVG(flow_rate) AS avg_flow_rate,
    AVG(power) AS avg_power,
    AVG(pressure) AS avg_pressure,
    AVG(cop) AS avg_cop,
    AVG(cooling_capacity) AS avg_cooling_capacity
FROM device_telemetry
GROUP BY bucket, device_id;

CREATE MATERIALIZED VIEW device_telemetry_1hour
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
    device_id,
    AVG(supply_temp) AS avg_supply_temp,
    AVG(return_temp) AS avg_return_temp,
    AVG(flow_rate) AS avg_flow_rate,
    AVG(power) AS avg_power,
    AVG(pressure) AS avg_pressure,
    AVG(cop) AS avg_cop,
    AVG(cooling_capacity) AS avg_cooling_capacity,
    MAX(power) AS max_power,
    MIN(cop) AS min_cop
FROM device_telemetry
GROUP BY bucket, device_id;

SELECT add_continuous_aggregate_policy('device_telemetry_5min',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 minute',
    schedule_interval => INTERVAL '5 minutes');

SELECT add_continuous_aggregate_policy('device_telemetry_1hour',
    start_offset => INTERVAL '7 days',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour');

SELECT add_retention_policy('device_telemetry', INTERVAL '90 days');
SELECT add_retention_policy('pue_records', INTERVAL '365 days');
SELECT add_retention_policy('alarms', INTERVAL '180 days');
