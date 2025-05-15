package main

import (
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"nvivas/backend/tictactoe-go-server/internal/client"
	"nvivas/backend/tictactoe-go-server/internal/hub"
)

const (
	defaultPort = "8080"
)

// Instancia global del Hub
var mainHub *hub.Hub

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Permisivo para desarrollo
	},
}

// handleConnections maneja las conexiones WebSocket entrantes
func handleConnections(w http.ResponseWriter, r *http.Request) {
	// Actualizar la conexión HTTP a WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error al actualizar la conexión: %v", err)
		return
	}

	// Crear una instancia de Client
	c := &client.Client{
		ID:   uuid.NewString(),
		Hub:  mainHub,
		Room: nil, // Se conectará más tarde
		Conn: conn,
		Send: make(chan []byte, 256),
	}

	// Registrar al cliente en el Hub
	mainHub.Register <- c

	// Iniciar goroutines para manejar la comunicación
	go c.ReadPump()
	go c.WritePump()
}

func main() {
	port := defaultPort

	// Crear e iniciar el Hub
	mainHub = hub.NewHub()
	go mainHub.Run()

	log.Printf("Hub iniciado")

	// Configurar rutas
	http.HandleFunc("/ws", handleConnections)

	log.Printf("Iniciando servidor en el puerto %s...", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}
