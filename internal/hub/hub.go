package hub

import (
	"encoding/json"
	"log"

	"github.com/google/uuid"

	"nvivas/backend/tictactoe-go-server/internal/interfaces"
	"nvivas/backend/tictactoe-go-server/internal/room"
	"nvivas/backend/tictactoe-go-server/pkg/models"
)

// Hub gestiona clientes conectados y salas de juego
type Hub struct {
	// Clientes conectados al servidor
	Clients map[interfaces.Client]bool

	// Salas activas
	Rooms map[string]*room.Room

	// Canal para registrar nuevos clientes
	Register chan interfaces.Client

	// Canal para desregistrar clientes
	Unregister chan interfaces.Client

	// Canal para crear una nueva sala
	CreateRoomChan chan interfaces.Client

	// Canal para unirse a una sala existente
	JoinRoomChan chan *JoinRequest

	// Canal para mensajes a todos los clientes (opcional)
	broadcast chan []byte
}

// JoinRequest representa una solicitud para unirse a una sala
type JoinRequest struct {
	Client interfaces.Client
	RoomID string
}

// NewHub crea una nueva instancia de Hub
func NewHub() *Hub {
	return &Hub{
		Clients:        make(map[interfaces.Client]bool),
		Rooms:          make(map[string]*room.Room),
		Register:       make(chan interfaces.Client),
		Unregister:     make(chan interfaces.Client),
		CreateRoomChan: make(chan interfaces.Client),
		JoinRoomChan:   make(chan *JoinRequest),
		broadcast:      make(chan []byte),
	}
}

// UnregisterClient implements interfaces.Hub
func (h *Hub) UnregisterClient(client interfaces.Client) {
	h.Unregister <- client
}

// CreateRoom implements interfaces.Hub
func (h *Hub) CreateRoom(client interfaces.Client) {
	h.CreateRoomChan <- client
}

// JoinRoom implements interfaces.Hub
func (h *Hub) JoinRoom(roomID string, client interfaces.Client) {
	h.JoinRoomChan <- &JoinRequest{
		Client: client,
		RoomID: roomID,
	}
}

// Run inicia el bucle principal del Hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			// Registrar un nuevo cliente
			h.Clients[client] = true
			log.Printf("Cliente registrado: %s", client.GetID())

		case client := <-h.Unregister:
			// Verificar si el cliente está registrado
			if _, ok := h.Clients[client]; ok {
				// Eliminar el cliente
				delete(h.Clients, client)
				log.Printf("Cliente desregistrado: %s", client.GetID())

				// Cerrar el canal Send si no se ha cerrado ya
				sendChan := client.GetSendChannel()
				select {
				case <-sendChan:
					// Canal ya cerrado
				default:
					close(sendChan)
				}

				// Si el cliente estaba en una sala, notificar a la sala
				if clientRoom, ok := client.GetRoom().(*room.Room); ok && clientRoom != nil {
					clientRoom.Unregister <- client
				}
			}

		case client := <-h.CreateRoomChan:
			// Crear un ID único para la sala
			roomID := uuid.NewString()

			// Crear una instancia de Room
			newRoom := room.NewRoom(roomID, h)

			// Almacenar la sala en el mapa de salas
			h.Rooms[roomID] = newRoom

			// Iniciar la sala como goroutine
			go newRoom.Run()

			// Registrar al cliente creador en la sala
			newRoom.Register <- client

			// Actualizar la referencia a la sala en el cliente
			client.SetRoom(newRoom)

			// Task 28: Enviar mensaje ROOM_CREATED { roomID, playerSymbol, playerID } al creador
			msg := models.RoomCreatedResponse{
				Type:     "ROOM_CREATED",
				RoomID:   roomID,
				PlayerID: client.GetID(),
				Symbol:   "X", // El creador siempre es X
			}
			msgBytes, _ := json.Marshal(msg)
			client.GetSendChannel() <- msgBytes

			log.Printf("Sala creada: %s por cliente %s (símbolo X)", roomID, client.GetID())

		case joinReq := <-h.JoinRoomChan:
			// Task 29: Mejorar la lógica de unirse a salas
			// Buscar la sala por su ID
			if room, exists := h.Rooms[joinReq.RoomID]; exists {
				// Verificar si la sala está llena antes de unirse
				if len(room.Clients) >= 2 {
					// Sala llena, enviar mensaje de error
					errorMsg := models.ErrorResponse{
						Type:    "ERROR_ROOM_FULL",
						Message: "La sala ya está llena",
					}
					msgBytes, _ := json.Marshal(errorMsg)
					joinReq.Client.GetSendChannel() <- msgBytes

					log.Printf("Error: Cliente %s intentó unirse a sala llena %s",
						joinReq.Client.GetID(), joinReq.RoomID)
					continue
				}

				// La sala existe y tiene espacio
				// Actualizar la referencia a la sala en el cliente
				joinReq.Client.SetRoom(room)

				// Registrar al cliente en la sala
				// La sala se encargará de enviar ROOM_JOINED y PLAYER_JOINED
				room.Register <- joinReq.Client

				log.Printf("Cliente %s se unió a sala %s", joinReq.Client.GetID(), joinReq.RoomID)
			} else {
				// Task 29: Si la sala no existe, enviar un mensaje de error claro
				errorMsg := models.ErrorResponse{
					Type:    "ERROR_ROOM_NOT_FOUND",
					Message: "La sala solicitada no existe",
				}
				msgBytes, _ := json.Marshal(errorMsg)
				joinReq.Client.GetSendChannel() <- msgBytes

				log.Printf("Error: Cliente %s intentó unirse a sala inexistente %s",
					joinReq.Client.GetID(), joinReq.RoomID)
			}
		}
	}
}
