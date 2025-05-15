package client

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"

	"nvivas/backend/tictactoe-go-server/internal/interfaces"
	"nvivas/backend/tictactoe-go-server/internal/room"
	"nvivas/backend/tictactoe-go-server/pkg/models"
)

// Client representa una conexión de cliente WebSocket
type Client struct {
	ID   string
	Hub  interfaces.Hub
	Room interface{} // Se reemplazará con *room.Room cuando se use
	Conn *websocket.Conn
	Send chan []byte
}

// GetID implements interfaces.Client
func (c *Client) GetID() string {
	return c.ID
}

// GetSendChannel implements interfaces.Client
func (c *Client) GetSendChannel() chan []byte {
	return c.Send
}

// GetConnection implements interfaces.Client
func (c *Client) GetConnection() *websocket.Conn {
	return c.Conn
}

// SetRoom implements interfaces.Client
func (c *Client) SetRoom(room interface{}) {
	c.Room = room
}

// GetRoom implements interfaces.Client
func (c *Client) GetRoom() interface{} {
	return c.Room
}

// ReadPump maneja la lectura de mensajes desde el WebSocket
func (c *Client) ReadPump() {
	defer func() {
		// Cuando ReadPump termina, desregistrar cliente y cerrar conexiones
		if c.Hub != nil {
			c.Hub.UnregisterClient(c)
		}
		c.Conn.Close()
		close(c.Send)
	}()

	// Configurar conexión
	c.Conn.SetReadLimit(1024) // Límite de tamaño de mensaje
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Bucle infinito para leer mensajes
	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure) {
				log.Printf("Error: %v", err)
			}
			break // Salir del bucle si hay error
		}

		// Deserializar el mensaje recibido
		var envelope models.Envelope
		if err := json.Unmarshal(message, &envelope); err != nil {
			log.Printf("Error al deserializar mensaje: %v", err)

			// Enviar mensaje de error al cliente
			errorMsg := models.ErrorResponse{
				Type:    "ERROR_INVALID_MESSAGE",
				Message: "Formato de mensaje inválido",
			}
			msgBytes, _ := json.Marshal(errorMsg)
			c.Send <- msgBytes
			continue
		}

		// Manejar el mensaje según su tipo
		switch envelope.Type {
		case "CREATE_ROOM":
			// Si el cliente solicita crear una sala, enviar al hub
			log.Printf("Cliente %s solicita crear sala", c.ID)
			if c.Hub != nil {
				// Enviar el cliente al hub para crear una sala
				c.Hub.UnregisterClient(c)
				c.SetRoom(nil)
				hub, ok := c.Hub.(interface {
					CreateRoom(client interfaces.Client)
				})
				if ok {
					hub.CreateRoom(c)
				} else {
					log.Printf("Error: Hub no tiene método CreateRoom")

					// Enviar mensaje de error al cliente
					errorMsg := models.ErrorResponse{
						Type:    "ERROR_INTERNAL",
						Message: "Error interno del servidor",
					}
					msgBytes, _ := json.Marshal(errorMsg)
					c.Send <- msgBytes
				}
			}

		case "JOIN_ROOM":
			// Deserializar el payload para obtener el RoomID
			var joinPayload models.JoinRoomPayload
			if err := json.Unmarshal(envelope.Payload, &joinPayload); err != nil {
				log.Printf("Error al deserializar JOIN_ROOM payload: %v", err)

				// Enviar mensaje de error al cliente
				errorMsg := models.ErrorResponse{
					Type:    "ERROR_INVALID_PAYLOAD",
					Message: "Datos de unión a sala inválidos",
				}
				msgBytes, _ := json.Marshal(errorMsg)
				c.Send <- msgBytes
				continue
			}

			log.Printf("Cliente %s solicita unirse a sala %s", c.ID, joinPayload.RoomID)
			if c.Hub != nil {
				// Si el cliente ya estaba en una sala, desregistrarlo
				c.Hub.UnregisterClient(c)
				c.SetRoom(nil)

				// Enviar solicitud para unirse a la sala
				hub, ok := c.Hub.(interface {
					JoinRoom(roomID string, client interfaces.Client)
				})
				if ok {
					hub.JoinRoom(joinPayload.RoomID, c)
				} else {
					log.Printf("Error: Hub no tiene método JoinRoom")

					// Enviar mensaje de error al cliente
					errorMsg := models.ErrorResponse{
						Type:    "ERROR_INTERNAL",
						Message: "Error interno del servidor",
					}
					msgBytes, _ := json.Marshal(errorMsg)
					c.Send <- msgBytes
				}
			}

		case "MAKE_MOVE":
			// Verificar que el cliente está en una sala
			if c.Room == nil {
				log.Printf("Error: Cliente %s intentó hacer un movimiento sin estar en una sala", c.ID)
				errorMsg := models.ErrorResponse{
					Type:    "ERROR_NOT_IN_ROOM",
					Message: "No estás en ninguna sala",
				}
				msgBytes, _ := json.Marshal(errorMsg)
				c.Send <- msgBytes
				continue
			}

			// Deserializar el payload para obtener las coordenadas del movimiento
			var movePayload models.MakeMovePayload
			if err := json.Unmarshal(envelope.Payload, &movePayload); err != nil {
				log.Printf("Error al deserializar MAKE_MOVE payload: %v", err)

				// Enviar mensaje de error al cliente
				errorMsg := models.ErrorResponse{
					Type:    "ERROR_INVALID_PAYLOAD",
					Message: "Datos de movimiento inválidos",
				}
				msgBytes, _ := json.Marshal(errorMsg)
				c.Send <- msgBytes
				continue
			}

			// Enviar el movimiento a la sala
			if roomObj, ok := c.Room.(*room.Room); ok && roomObj != nil {
				playerMove := &models.PlayerMove{
					Client:   c,
					MoveData: movePayload.Move,
				}
				roomObj.ReceiveMove <- playerMove
			} else {
				log.Printf("Error: Room no es del tipo esperado")

				// Enviar mensaje de error al cliente
				errorMsg := models.ErrorResponse{
					Type:    "ERROR_INTERNAL",
					Message: "Error interno del servidor",
				}
				msgBytes, _ := json.Marshal(errorMsg)
				c.Send <- msgBytes
			}

		default:
			log.Printf("Tipo de mensaje desconocido: %s", envelope.Type)

			// Enviar mensaje de error al cliente
			errorMsg := models.ErrorResponse{
				Type:    "ERROR_UNKNOWN_MESSAGE_TYPE",
				Message: "Tipo de mensaje desconocido",
			}
			msgBytes, _ := json.Marshal(errorMsg)
			c.Send <- msgBytes
		}
	}
}

// WritePump maneja el envío de mensajes al WebSocket
func (c *Client) WritePump() {
	ticker := time.NewTicker(50 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// El canal Send está cerrado
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Añadir cualquier mensaje pendiente en el canal
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
