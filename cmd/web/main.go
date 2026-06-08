package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

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

func eventRouter(ctx context.Context, hub *WebSocketHub, gateway *backend.ModbusGateway, calculator *backend.PUECalculator, optimizer *backend.CoolingOptimizer, notifier *backend.AlarmNotifier) {
	telemetryOut := gateway.Output()
	pueOut := calculator.Output()
	suggestionOut := optimizer.Output()
	alarmOut := notifier.Output()
	telemetryIn := notifier.TelemetryCh()
	pueIn := notifier.PUECh()
	triggerCh := optimizer.TriggerCh()

	for {
		select {
		case <-ctx.Done():
			return

		case batch := <-telemetryOut:
			if err := backend.InsertTelemetry(batch); err != nil {
				log.Println("Telemetry insert error:", err)
			}
			msg, _ := json.Marshal(backend.WSMessage{Type: "telemetry", Data: batch})
			hub.broadcast <- msg
			select {
			case telemetryIn <- batch:
			default:
			}

		case record := <-pueOut:
			msg, _ := json.Marshal(backend.WSMessage{Type: "pue_update", Data: record})
			hub.broadcast <- msg
			select {
			case pueIn <- record:
			default:
			}
			if record.PUEValue > optimizer.PUETriggerThreshold() {
				select {
				case triggerCh <- record:
				default:
				}
			}

		case suggestions := <-suggestionOut:
			msg, _ := json.Marshal(backend.WSMessage{Type: "optimization", Data: suggestions})
			hub.broadcast <- msg

		case alarms := <-alarmOut:
			msg, _ := json.Marshal(backend.WSMessage{Type: "alarm", Data: alarms})
			hub.broadcast <- msg
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

	gateway := backend.NewModbusGateway(cfg)
	calculator := backend.NewPUECalculator(cfg)
	optimizer := backend.NewCoolingOptimizer(cfg)
	notifier := backend.NewAlarmNotifier(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go gateway.Run(ctx)
	go calculator.Run(ctx)
	go optimizer.Run(ctx)
	go notifier.Run(ctx)
	go eventRouter(ctx, hub, gateway, calculator, optimizer, notifier)

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
