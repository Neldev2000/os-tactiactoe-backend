package client

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"

	"nvivas/backend/tictactoe-go-server/internal/errors"
	"nvivas/backend/tictactoe-go-server/internal/interfaces"
	"nvivas/backend/tictactoe-go-server/internal/logger"
	"nvivas/backend/tictactoe-go-server/internal/room"
	"nvivas/backend/tictactoe-go-server/pkg/models"
)

const (
	// Tiempo máximo para esperar un mensaje del cliente
	readWait = 60 * time.Second

	// Tiempo entre pings
	pingPeriod = (readWait * 9) / 10

	// Límite máximo para mensajes entrantes
	maxMessageSize = 1024 * 16 // 16KB - límite razonable para mensajes de juego
)

// Client representa una conexión de cliente WebSocket
type Client struct {
	ID   string
	Hub  interfaces.Hub
	Room interface{} // Se reemplazará con *room.Room cuando se use
	Conn *websocket.Conn
	Send chan []byte

	// Context para control de cancelación
	ctx    context.Context
	cancel context.CancelFunc
}

// NewClient crea un nuevo cliente
func NewClient(id string, hub interfaces.Hub, conn *websocket.Conn, parentCtx context.Context) *Client {
	// Crear un contexto derivado que se pueda cancelar independientemente
	ctx, cancel := context.WithCancel(parentCtx)

	return &Client{
		ID:     id,
		Hub:    hub,
		Room:   nil,
		Conn:   conn,
		Send:   make(chan []byte, 256), // Buffer para mensajes pendientes
		ctx:    ctx,
		cancel: cancel,
	}
}

// Close cancela el contexto y libera recursos
func (c *Client) Close() {
	c.cancel()
	c.Conn.Close()
	// No cerramos el canal Send aquí para evitar data races
	// La cancelación del contexto debería ser suficiente para que las goroutines terminen
	logger.Info("Cliente cerrado", logger.Fields{"clientID": c.ID})
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
		logger.Info("ReadPump terminando, desregistrando cliente", logger.Fields{
			"clientID": c.ID,
		})

		if c.Hub != nil {
			c.Hub.UnregisterClient(c)
		}

		// Cerrar la conexión y el canal
		c.Conn.Close()

		// Cerrar el canal si no está cerrado
		select {
		case _, ok := <-c.Send:
			if ok {
				close(c.Send)
			}
		default:
			close(c.Send)
		}
	}()

	// Configurar límites y timeouts para prevenir ataques DoS
	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(readWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(readWait))
		return nil
	})

	// Bucle infinito para leer mensajes
	for {
		select {
		case <-c.ctx.Done():
			// Contexto cancelado, terminar
			logger.Info("Contexto cancelado, terminando ReadPump", logger.Fields{
				"clientID": c.ID,
			})
			return

		default:
			// Intentar leer un mensaje
			_, message, err := c.Conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err,
					websocket.CloseGoingAway,
					websocket.CloseAbnormalClosure) {
					logger.Error("Error en conexión WebSocket", logger.Fields{
						"error":    err.Error(),
						"clientID": c.ID,
					})
				}
				return // Salir del bucle si hay error
			}

			// Verificar tamaño del mensaje
			if len(message) > maxMessageSize {
				logger.Warn("Mensaje excede el tamaño máximo permitido", logger.Fields{
					"clientID":    c.ID,
					"messageSize": len(message),
					"maxAllowed":  maxMessageSize,
				})
				errors.MessageTooLarge(c.Send, c.ID)
				continue
			}

			// Deserializar el mensaje recibido
			var envelope models.Envelope
			if err := json.Unmarshal(message, &envelope); err != nil {
				logger.Error("Error deserializando mensaje", logger.Fields{
					"error":    err.Error(),
					"clientID": c.ID,
				})

				// Enviar mensaje de error al cliente
				errors.InvalidMessage(c.Send, c.ID)
				continue
			}

			// Manejar el mensaje según su tipo
			switch envelope.Type {
			case "CREATE_ROOM":
				// Si el cliente solicita crear una sala, enviar al hub
				logger.Info("Cliente solicita crear sala", logger.Fields{
					"clientID": c.ID,
				})

				if c.Hub != nil {
					// Ya no desregistramos al cliente aquí
					// c.Hub.UnregisterClient(c)
					// c.SetRoom(nil)

					hub, ok := c.Hub.(interface {
						CreateRoom(client interfaces.Client)
					})
					if ok {
						hub.CreateRoom(c)
					} else {
						logger.Error("Hub no tiene método CreateRoom", logger.Fields{
							"clientID": c.ID,
						})

						// Enviar mensaje de error al cliente
						errors.Internal(c.Send, c.ID)
					}
				}

			case "JOIN_ROOM":
				// Deserializar el payload para obtener el RoomID
				var joinPayload models.JoinRoomPayload
				if err := json.Unmarshal(envelope.Payload, &joinPayload); err != nil {
					logger.Error("Error deserializando payload JOIN_ROOM", logger.Fields{
						"error":    err.Error(),
						"clientID": c.ID,
					})

					// Enviar mensaje de error al cliente
					errors.InvalidPayload(c.Send, "join room", c.ID)
					continue
				}

				logger.Info("Cliente solicita unirse a sala", logger.Fields{
					"clientID": c.ID,
					"roomID":   joinPayload.RoomID,
				})

				if c.Hub != nil {
					// Ya no desregistramos al cliente aquí
					// c.Hub.UnregisterClient(c)
					// c.SetRoom(nil)

					// Enviar solicitud para unirse a la sala
					hub, ok := c.Hub.(interface {
						JoinRoom(roomID string, client interfaces.Client)
					})
					if ok {
						hub.JoinRoom(joinPayload.RoomID, c)
					} else {
						logger.Error("Hub no tiene método JoinRoom", logger.Fields{
							"clientID": c.ID,
						})

						// Enviar mensaje de error al cliente
						errors.Internal(c.Send, c.ID)
					}
				}

			case "MAKE_MOVE":
				// Verificar que el cliente está en una sala
				if c.Room == nil {
					logger.Warn("Cliente intentó hacer un movimiento sin estar en una sala", logger.Fields{
						"clientID": c.ID,
					})

					errors.NotInRoom(c.Send, c.ID)
					continue
				}

				// Deserializar el payload para obtener las coordenadas del movimiento
				var movePayload models.MakeMovePayload
				if err := json.Unmarshal(envelope.Payload, &movePayload); err != nil {
					logger.Error("Error deserializando payload MAKE_MOVE", logger.Fields{
						"error":    err.Error(),
						"clientID": c.ID,
					})

					// Enviar mensaje de error al cliente
					errors.InvalidPayload(c.Send, "make move", c.ID)
					continue
				}

				// Enviar el movimiento a la sala
				if roomObj, ok := c.Room.(*room.Room); ok && roomObj != nil {
					playerMove := &models.PlayerMove{
						Client:   c,
						MoveData: movePayload.Move,
					}
					roomObj.ReceiveMove <- playerMove

					logger.Info("Movimiento enviado a sala", logger.Fields{
						"clientID": c.ID,
						"roomID":   roomObj.ID,
						"row":      movePayload.Move.Row,
						"col":      movePayload.Move.Col,
					})
				} else {
					logger.Error("Room no es del tipo esperado", logger.Fields{
						"clientID": c.ID,
					})

					// Enviar mensaje de error al cliente
					errors.Internal(c.Send, c.ID)
				}

			case "LIST_ROOMS":
				// Cliente solicita listar las salas disponibles
				logger.Info("Cliente solicita listar salas", logger.Fields{
					"clientID": c.ID,
				})

				if c.Hub != nil {
					// Solicitar al hub que envíe la lista de salas al cliente
					hub, ok := c.Hub.(interface {
						ListRooms(client interfaces.Client)
					})
					if ok {
						hub.ListRooms(c)
					} else {
						logger.Error("Hub no tiene método ListRooms", logger.Fields{
							"clientID": c.ID,
						})

						// Enviar mensaje de error al cliente
						errors.Internal(c.Send, c.ID)
					}
				}

			default:
				logger.Warn("Tipo de mensaje desconocido", logger.Fields{
					"messageType": envelope.Type,
					"clientID":    c.ID,
				})

				// Enviar mensaje de error al cliente
				errors.UnknownMessageType(c.Send, envelope.Type, c.ID)
			}
		}
	}
}

// WritePump maneja el envío de mensajes al WebSocket
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
		logger.Info("WritePump terminado", logger.Fields{"clientID": c.ID})
	}()

	for {
		select {
		case <-c.ctx.Done():
			// Contexto cancelado, terminar
			logger.Info("Contexto cancelado, terminando WritePump", logger.Fields{
				"clientID": c.ID,
			})
			return

		case message, ok := <-c.Send:
			// Establecer tiempo máximo para escribir
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// El canal Send está cerrado
				logger.Info("Canal Send cerrado, enviando mensaje de cierre", logger.Fields{
					"clientID": c.ID,
				})
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				logger.Error("Error obteniendo writer de WebSocket", logger.Fields{
					"error":    err.Error(),
					"clientID": c.ID,
				})
				return
			}

			if _, err := w.Write(message); err != nil {
				logger.Error("Error escribiendo mensaje", logger.Fields{
					"error":    err.Error(),
					"clientID": c.ID,
				})
				return
			}

			// Añadir cualquier mensaje pendiente en el canal
			n := len(c.Send)
			for i := 0; i < n; i++ {
				msg := <-c.Send
				if _, err := w.Write(msg); err != nil {
					logger.Error("Error escribiendo mensaje encolado", logger.Fields{
						"error":    err.Error(),
						"clientID": c.ID,
					})
				}
			}

			if err := w.Close(); err != nil {
				logger.Error("Error cerrando writer de WebSocket", logger.Fields{
					"error":    err.Error(),
					"clientID": c.ID,
				})
				return
			}

		case <-ticker.C:
			// Enviar ping periódico para mantener la conexión activa
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Error("Error enviando ping", logger.Fields{
					"error":    err.Error(),
					"clientID": c.ID,
				})
				return
			}
			logger.Debug("Ping enviado", logger.Fields{"clientID": c.ID})
		}
	}
}
