package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"dc-cooling-platform/backend"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type WebSocketClient struct {
	Conn *websocket.Conn
	Send chan []byte
}

type WebSocketHub struct {
	clients    map[*WebSocketClient]bool
	broadcast  chan []byte
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	mu         sync.RWMutex
}

func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*WebSocketClient]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
	}
}

func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					h.mu.RUnlock()
					h.mu.Lock()
					close(client.Send)
					delete(h.clients, client)
					h.mu.Unlock()
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func handleWebSocket(hub *WebSocketHub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	client := &WebSocketClient{Conn: conn, Send: make(chan []byte, 256)}
	hub.register <- client

	go func() {
		defer func() {
			hub.unregister <- client
			conn.Close()
		}()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()

	go func() {
		for msg := range client.Send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				break
			}
		}
		conn.Close()
	}()
}

func modbusDataIngestion(ctx context.Context, hub *WebSocketHub, cfg *backend.Config) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			data, err := backend.ReceiveModbusData(cfg.ModbusHost, cfg.ModbusPort)
			if err != nil {
				log.Println("Modbus read error:", err)
				continue
			}
			if len(data) > 0 {
				if err := backend.InsertTelemetry(data); err != nil {
					log.Println("Telemetry insert error:", err)
					continue
				}
				msg, _ := json.Marshal(backend.WSMessage{Type: "telemetry", Data: data})
				hub.broadcast <- msg
			}
		}
	}
}

func pueCalculation(ctx context.Context, hub *WebSocketHub, cfg *backend.Config) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			records, err := backend.CalculatePUE(cfg)
			if err != nil {
				log.Println("PUE calculation error:", err)
				continue
			}
			if len(records) > 0 {
				msg, _ := json.Marshal(backend.WSMessage{Type: "pue_update", Data: records})
				hub.broadcast <- msg
			}
		}
	}
}

func alarmChecker(ctx context.Context, hub *WebSocketHub, cfg *backend.Config) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			alarms, err := backend.CheckAlarms(cfg)
			if err != nil {
				log.Println("Alarm check error:", err)
				continue
			}
			for _, alarm := range alarms {
				msg, _ := json.Marshal(backend.WSMessage{Type: "alarm", Data: alarm})
				hub.broadcast <- msg
			}
		}
	}
}

func coolingOptimization(ctx context.Context, hub *WebSocketHub, cfg *backend.Config) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			suggestions, err := backend.OptimizeCoolingDistribution(cfg)
			if err != nil {
				log.Println("Optimization error:", err)
				continue
			}
			if len(suggestions) > 0 {
				msg, _ := json.Marshal(backend.WSMessage{Type: "optimization", Data: suggestions})
				hub.broadcast <- msg
			}
		}
	}
}

func main() {
	cfg := backend.LoadConfig()

	if err := backend.InitDB(cfg); err != nil {
		log.Fatal("Database initialization failed:", err)
	}
	defer backend.CloseDB()

	hub := NewWebSocketHub()
	go hub.Run()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go modbusDataIngestion(ctx, hub, cfg)
	go pueCalculation(ctx, hub, cfg)
	go alarmChecker(ctx, hub, cfg)
	go coolingOptimization(ctx, hub, cfg)

	r := mux.NewRouter()

	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(hub, w, r)
	})

	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/devices", backend.HandleGetDevices).Methods("GET")
	api.HandleFunc("/devices/states", backend.HandleGetDeviceStates).Methods("GET")
	api.HandleFunc("/telemetry", backend.HandleGetDeviceTelemetry).Methods("GET")
	api.HandleFunc("/pue/trend", backend.HandleGetPUETrend).Methods("GET")
	api.HandleFunc("/pue/current", backend.HandleGetCurrentPUE).Methods("GET")
	api.HandleFunc("/zones", backend.HandleGetZones).Methods("GET")
	api.HandleFunc("/sankey", backend.HandleGetSankey).Methods("GET")
	api.HandleFunc("/ranking", backend.HandleGetRanking).Methods("GET")
	api.HandleFunc("/suggestions", backend.HandleGetSuggestions).Methods("GET")
	api.HandleFunc("/alarms", backend.HandleGetAlarms).Methods("GET")
	api.HandleFunc("/alarms/{id}/ack", backend.HandleAcknowledgeAlarm).Methods("POST")
	api.HandleFunc("/alarms/counts", backend.HandleGetAlarmCounts).Methods("GET")

	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./frontend/")))

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("Server starting on port", cfg.HTTPPort)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal("Server error:", err)
	}
}
