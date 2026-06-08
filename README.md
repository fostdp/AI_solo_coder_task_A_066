# 大型数据中心制冷系统能效优化平台

## 系统架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Docker Compose                               │
│                                                                     │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────────┐   │
│  │  modbus-     │    │  go-server   │    │    timescaledb       │   │
│  │  simulator   │───▶│  :8080       │───▶│    :5432             │   │
│  │  :5020       │    │              │    │                      │   │
│  │  :8081(ctrl) │    │  /api/*      │    │  7 tables            │   │
│  │              │    │  /ws         │    │  5 hypertables       │   │
│  │  8 chillers  │    │  /metrics    │    │  2 continuous aggs   │   │
│  │  12 towers   │    │  /debug/pprof│    │  4 compression pols  │   │
│  │  80 PACs     │    │  Gzip        │    │  6 retention pols    │   │
│  │  20 CDUs     │    │              │    │                      │   │
│  └──────────────┘    └──────┬───────┘    └──────────────────────┘   │
│                             │                                      │
│                    ┌────────▼────────┐                              │
│                    │   Browser       │                              │
│                    │   :8080         │                              │
│                    │                 │                              │
│                    │  Three.js 3D    │                              │
│                    │  Canvas Charts  │                              │
│                    │  WebSocket RT   │                              │
│                    └─────────────────┘                              │
└─────────────────────────────────────────────────────────────────────┘
```

### Go 后端模块架构（Channel 通信）

```
ModbusGateway ──outCh──▶ eventRouter ──▶ InsertTelemetry + WS broadcast
                                    └──▶ AlarmNotifier.telemetryCh
                                          │
PUECalculator ──outCh──▶ eventRouter ──▶ WS broadcast
                                    └──▶ AlarmNotifier.pueCh
                                    └──▶ CoolingOptimizer.triggerCh (PUE>1.4)
                                          │
CoolingOptimizer ──outCh──▶ eventRouter ──▶ WS broadcast
                                          │
AlarmNotifier ──outCh──▶ eventRouter ──▶ WS broadcast + DingTalk
```

## 快速部署

### 前置条件

- Docker 20.10+
- Docker Compose V2

### 一键启动

```bash
docker compose up -d --build
```

启动后访问：
- **前端界面**: http://localhost:8080
- **Prometheus 指标**: http://localhost:8080/metrics
- **pprof 性能分析**: http://localhost:8080/debug/pprof/
- **Modbus 控制端口**: localhost:8081

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DB_HOST` | timescaledb | 数据库主机 |
| `DB_PORT` | 5432 | 数据库端口 |
| `DB_USER` | postgres | 数据库用户 |
| `DB_PASSWORD` | postgres | 数据库密码 |
| `DB_NAME` | dc_cooling | 数据库名 |
| `HTTP_PORT` | 8080 | HTTP 服务端口 |
| `MODBUS_HOST` | modbus-simulator | Modbus 设备地址 |
| `MODBUS_PORT` | 5020 | Modbus 端口 |
| `CONFIG_PATH` | /etc/dc-cooling/config.json | 模型参数配置文件 |
| `DINGTALK_WEBHOOK` | (空) | 钉钉告警推送 Webhook |

### 配置文件

模型参数通过 `config/config.json` 管理，支持运行时调整：

| 配置段 | 关键参数 | 默认值 |
|--------|----------|--------|
| `modbus` | idle_timeout_seconds | 60 |
| `modbus` | collect_interval_seconds | 30 |
| `pue` | calculate_interval_seconds | 300 |
| `pue` | distribution_loss_ratio | 0.03 |
| `pue` | pue_threshold_1 / pue_threshold_2 | 1.4 / 1.5 |
| `optimization` | diff_threshold / saving_ratio | 0.05 / 0.3 |
| `alarm` | level1_duration_minutes | 10 |
| `alarm` | level2_duration_minutes | 30 |
| `alarm` | dingtalk_max_retries | 3 |

## Modbus 模拟器

### 命令行参数

```bash
python simulator/modbus_simulator.py \
  --host 0.0.0.0 \
  --port 5020 \
  --chillers 8 \
  --towers 12 \
  --pacs 80 \
  --cdus 20 \
  --interval 30 \
  --anomaly-prob 0.03 \
  --low-cop 2.5 \
  --control-port 8081
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--chillers` | 8 | 离心式冷水机组数量 |
| `--towers` | 12 | 冷却塔数量 |
| `--pacs` | 80 | 精密空调数量 |
| `--cdus` | 20 | 液冷 CDU 数量 |
| `--interval` | 30 | 数据刷新间隔（秒） |
| `--anomaly-prob` | 0.03 | 每周期随机 COP 异常概率 |
| `--low-cop` | 2.5 | 异常时 COP 目标值 |
| `--control-port` | 8081 | 控制服务端口 |

### 异常注入

模拟器运行时，通过控制端口动态注入能效异常：

```bash
# 向所有设备注入 5 分钟异常
echo '{"action":"inject_anomaly","duration":300}' | nc localhost 8081

# 仅向冷水机组注入异常
echo '{"action":"inject_anomaly","device_type":"chiller","duration":600}' | nc localhost 8081

# 查询模拟器状态
echo '{"action":"status"}' | nc localhost 8081
```

`device_type` 可选值: `chiller`, `cooling_tower`, `precision_ac`, `cdu`

### Modbus 寄存器映射

每个设备暴露 10 个 Holding Register（功能码 0x03）：

| 地址 | 参数 | 缩放 |
|------|------|------|
| 0 | 供水温度 | ×10 |
| 1 | 回水温度 | ×10 |
| 2 | 流量 | ×10 |
| 3 | 功率 | ×10 |
| 4 | 压力 | ×10 |
| 5 | COP | ×100 |
| 6 | 制冷量 | ×10 |
| 7 | 设定温度 | ×10 |
| 8 | 运行状态 | 1=运行 0=停机 |
| 9 | 保留 | - |

## API 接口

| 路径 | 方法 | 说明 |
|------|------|------|
| `/api/devices` | GET | 设备列表 |
| `/api/devices/states` | GET | 设备实时状态（含 COP 颜色） |
| `/api/telemetry?device_id=1&hours=24` | GET | 遥测历史 |
| `/api/pue/trend?hours=24` | GET | PUE 趋势 |
| `/api/pue/current` | GET | 当前 PUE |
| `/api/zones` | GET | 冷区需求 |
| `/api/sankey` | GET | 桑基图数据 |
| `/api/ranking` | GET | 能效排行 |
| `/api/suggestions` | GET | 优化建议 |
| `/api/alarms?level=1&limit=50` | GET | 告警列表 |
| `/api/alarms/{id}/ack` | POST | 确认告警 |
| `/api/alarms/counts` | GET | 未确认告警数 |
| `/ws` | WebSocket | 实时推送 |
| `/metrics` | GET | Prometheus 指标 |
| `/debug/pprof/` | GET | pprof 性能分析 |

## Prometheus 指标

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `dc_cooling_http_requests_total` | Counter | HTTP 请求计数（method/path/status） |
| `dc_cooling_http_request_duration_seconds` | Histogram | HTTP 请求延迟 |
| `dc_cooling_modbus_collect_total` | Counter | Modbus 采集周期计数 |
| `dc_cooling_modbus_collect_errors_total` | Counter | Modbus 采集错误计数 |
| `dc_cooling_modbus_collect_duration_seconds` | Histogram | Modbus 采集耗时 |
| `dc_cooling_pue_value` | Gauge | 当前 PUE |
| `dc_cooling_cooling_power_watts` | Gauge | 总制冷功率 |
| `dc_cooling_it_power_watts` | Gauge | IT 负载功率 |
| `dc_cooling_alarms_total` | Counter | 告警计数（按级别） |
| `dc_cooling_websocket_connections` | Gauge | WebSocket 连接数 |
| `dc_cooling_db_query_duration_seconds` | Histogram | 数据库查询耗时 |

## TimescaleDB 数据治理

| 表 | 压缩策略 | 保留策略 | 压缩 SegmentBy |
|----|----------|----------|----------------|
| device_telemetry | 7 天后压缩 | 90 天 | device_id |
| pue_records | 14 天后压缩 | 365 天 | - |
| zone_cooling_demand | 7 天后压缩 | 90 天 | zone |
| it_power_readings | 14 天后压缩 | 365 天 | - |
| optimization_suggestions | - | 180 天 | - |
| alarms | - | 180 天 | - |

连续聚合：
- `device_telemetry_5min` — 5 分钟粒度，3 小时窗口，5 分钟刷新
- `device_telemetry_1hour` — 1 小时粒度，7 天窗口，1 小时刷新

## pprof 使用示例

```bash
# CPU profile（30秒采样）
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

# 内存分配
go tool pprof http://localhost:8080/debug/pprof/heap

# Goroutine 分析
go tool pprof http://localhost:8080/debug/pprof/goroutine

# 在线可视化（需要 graphviz）
go tool pprof -http=:6060 http://localhost:8080/debug/pprof/profile?seconds=30
```

## 停止服务

```bash
docker compose down

# 同时删除数据卷
docker compose down -v
```
