package hub

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"nvivas/backend/tictactoe-go-server/internal/errors"
	"nvivas/backend/tictactoe-go-server/internal/interfaces"
	"nvivas/backend/tictactoe-go-server/internal/logger"
	"nvivas/backend/tictactoe-go-server/internal/room"
	"nvivas/backend/tictactoe-go-server/pkg/models"
)

// Hub gestiona clientes conectados y salas de juego
type Hub struct {
	// Context para control de cancelación
	ctx    context.Context
	cancel context.CancelFunc

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

	// Canal para eliminar una sala
	DeleteRoomChan chan string

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
	ctx, cancel := context.WithCancel(context.Background())

	return &Hub{
		ctx:            ctx,
		cancel:         cancel,
		Clients:        make(map[interfaces.Client]bool),
		Rooms:          make(map[string]*room.Room),
		Register:       make(chan interfaces.Client),
		Unregister:     make(chan interfaces.Client),
		CreateRoomChan: make(chan interfaces.Client),
		JoinRoomChan:   make(chan *JoinRequest),
		DeleteRoomChan: make(chan string),
		broadcast:      make(chan []byte),
	}
}

// Close cancela el contexto y libera recursos
func (h *Hub) Close() {
	h.cancel()
	// No cerramos los canales aquí, porque podría haber goroutines escribiendo en ellos
	// La cancelación del contexto debería ser suficiente para que salgan de sus bucles
	logger.Info("Hub cerrado", nil)
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

// DeleteRoom implements interfaces.Hub
func (h *Hub) DeleteRoom(roomID string) {
	h.DeleteRoomChan <- roomID
}

// ListRooms implements interfaces.Hub
func (h *Hub) ListRooms(client interfaces.Client) {
	// Create a list of room information
	roomsList := make([]models.RoomInfo, 0, len(h.Rooms))

	for roomID, room := range h.Rooms {
		// Get player IDs
		playerIDs := room.GetPlayerIDs()

		// Determine if room is full
		isFull := len(playerIDs) >= 2

		// Add room info to the list
		roomInfo := models.RoomInfo{
			RoomID:  roomID,
			Players: playerIDs,
			IsFull:  isFull,
		}
		roomsList = append(roomsList, roomInfo)
	}

	// Create the response
	response := models.RoomListPayload{
		Type:  "ROOM_LIST",
		Rooms: roomsList,
	}

	// Serialize the response
	responseBytes, err := json.Marshal(response)
	if err != nil {
		logger.Error("Error serializando lista de salas", logger.Fields{
			"error":    err.Error(),
			"clientID": client.GetID(),
		})
		errors.Internal(client.GetSendChannel(), client.GetID())
		return
	}

	// Send the response to the client
	client.GetSendChannel() <- responseBytes

	logger.Info("Lista de salas enviada", logger.Fields{
		"clientID":  client.GetID(),
		"roomCount": len(roomsList),
	})
}

// createErrorMessage crea un mensaje de error serializado en JSON
func createErrorMessage(errorType, message string, clientID string) []byte {
	errorMsg := models.ErrorResponse{
		Type:    errorType,
		Message: message,
	}

	msgBytes, err := json.Marshal(errorMsg)
	if err != nil {
		logger.Error("Failed to marshal error message", logger.Fields{
			"error":     err.Error(),
			"errorType": errorType,
			"clientID":  clientID,
		})
		return []byte{}
	}

	return msgBytes
}

// Run inicia el bucle principal del Hub
func (h *Hub) Run() {
	defer func() {
		// Cleanup cuando Run termina
		logger.Info("Finalizando Hub.Run, liberando recursos", nil)

		// Cerrar todas las salas
		for id, r := range h.Rooms {
			logger.Info("Cerrando sala", logger.Fields{"roomID": id})
			r.Close()
		}
	}()

	for {
		select {
		case <-h.ctx.Done():
			// Contexto cancelado, terminar
			logger.Info("Contexto cancelado, terminando Hub.Run", nil)
			return

		case client := <-h.Register:
			// Registrar un nuevo cliente
			h.Clients[client] = true
			logger.Info("Cliente registrado", logger.Fields{
				"clientID": client.GetID(),
			})

		case client := <-h.Unregister:
			// Verificar si el cliente está registrado
			if _, ok := h.Clients[client]; ok {
				// Eliminar el cliente
				delete(h.Clients, client)
				logger.Info("Cliente desregistrado", logger.Fields{
					"clientID": client.GetID(),
				})

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
			newRoom := room.NewRoom(roomID, h, h.ctx)

			// Almacenar la sala en el mapa de salas
			h.Rooms[roomID] = newRoom

			// Iniciar la sala como goroutine
			go newRoom.Run()

			// Si el cliente ya estaba en una sala, limpiamos la referencia
			oldRoom := client.GetRoom()
			if oldRoom != nil {
				client.SetRoom(nil)
			}

			// Actualizar la referencia a la sala en el cliente
			client.SetRoom(newRoom)

			// Registrar al cliente creador en la sala
			newRoom.Register <- client

			// Task 28: Enviar mensaje ROOM_CREATED { roomID, playerSymbol, playerID } al creador
			msg := models.RoomCreatedResponse{
				Type:     "ROOM_CREATED",
				RoomID:   roomID,
				PlayerID: client.GetID(),
				Symbol:   "X", // El creador siempre es X
			}
			msgBytes, _ := json.Marshal(msg)

			// Usar select para enviar de forma segura
			select {
			case client.GetSendChannel() <- msgBytes:
				// Mensaje enviado con éxito
			default:
				logger.Warn("No se pudo enviar mensaje ROOM_CREATED, canal posiblemente cerrado", logger.Fields{
					"clientID": client.GetID(),
					"roomID":   roomID,
				})
			}

			logger.Info("Sala creada", logger.Fields{
				"roomID":   roomID,
				"clientID": client.GetID(),
				"symbol":   "X",
			})

		case joinReq := <-h.JoinRoomChan:
			// Task 29: Mejorar la lógica de unirse a salas
			// Buscar la sala por su ID
			if room, exists := h.Rooms[joinReq.RoomID]; exists {
				// Verificar si la sala está llena antes de unirse
				if len(room.Clients) >= 2 {
					// Sala llena, enviar mensaje de error
					select {
					case joinReq.Client.GetSendChannel() <- createErrorMessage(errors.ErrorRoomFull, "La sala ya está llena", joinReq.Client.GetID()):
						// Mensaje enviado con éxito
					default:
						logger.Warn("No se pudo enviar mensaje de error, canal posiblemente cerrado", logger.Fields{
							"clientID": joinReq.Client.GetID(),
							"roomID":   joinReq.RoomID,
						})
					}

					logger.Warn("Intento de unirse a sala llena", logger.Fields{
						"roomID":   joinReq.RoomID,
						"clientID": joinReq.Client.GetID(),
					})
					continue
				}

				// Si el cliente ya estaba en una sala, primero limpiamos la referencia
				oldRoom := joinReq.Client.GetRoom()
				if oldRoom != nil {
					// Ya no estamos usando el canal Unregister directamente
					// Simplemente limpiamos la referencia
					joinReq.Client.SetRoom(nil)
				}

				// La sala existe y tiene espacio
				// Actualizar la referencia a la sala en el cliente
				joinReq.Client.SetRoom(room)

				// Registrar al cliente en la sala
				// La sala se encargará de enviar ROOM_JOINED y PLAYER_JOINED
				room.Register <- joinReq.Client

				logger.Info("Cliente unido a sala", logger.Fields{
					"roomID":   joinReq.RoomID,
					"clientID": joinReq.Client.GetID(),
				})
			} else {
				// Task 29: Si la sala no existe, enviar un mensaje de error claro
				select {
				case joinReq.Client.GetSendChannel() <- createErrorMessage(errors.ErrorRoomNotFound, "La sala solicitada no existe", joinReq.Client.GetID()):
					// Mensaje enviado con éxito
				default:
					logger.Warn("No se pudo enviar mensaje de error, canal posiblemente cerrado", logger.Fields{
						"clientID": joinReq.Client.GetID(),
						"roomID":   joinReq.RoomID,
					})
				}

				logger.Warn("Intento de unirse a sala inexistente", logger.Fields{
					"roomID":   joinReq.RoomID,
					"clientID": joinReq.Client.GetID(),
				})
			}

		case roomID := <-h.DeleteRoomChan:
			// Eliminar una sala cuando ya no es necesaria
			if room, exists := h.Rooms[roomID]; exists {
				logger.Info("Eliminando sala", logger.Fields{"roomID": roomID})

				// Cancelar el contexto de la sala (ya que Room ahora usará contexto)
				room.Close()

				// Eliminar la sala del mapa
				delete(h.Rooms, roomID)

				logger.Info("Sala eliminada exitosamente", logger.Fields{"roomID": roomID})
			}
		}
	}
}
