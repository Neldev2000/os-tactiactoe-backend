package room

import (
	"context"
	"encoding/json"

	"nvivas/backend/tictactoe-go-server/internal/errors"
	"nvivas/backend/tictactoe-go-server/internal/game"
	"nvivas/backend/tictactoe-go-server/internal/interfaces"
	"nvivas/backend/tictactoe-go-server/internal/logger"
	"nvivas/backend/tictactoe-go-server/pkg/models"
)

// Room representa una sala de juego
type Room struct {
	ID          string                     // Identificador único de la sala
	Hub         interfaces.Hub             // Referencia al Hub principal
	Clients     map[interfaces.Client]bool // Clientes en la sala (máximo 2)
	GameState   *game.GameState            // Estado actual del juego
	Register    chan interfaces.Client     // Canal para registrar clientes
	Unregister  chan interfaces.Client     // Canal para desregistrar clientes
	Broadcast   chan []byte                // Canal para mensajes a todos los clientes
	ReceiveMove chan *models.PlayerMove    // Canal para recibir movimientos

	// Context para control de cancelación
	ctx    context.Context
	cancel context.CancelFunc
}

// NewRoom crea una nueva sala de juego
func NewRoom(id string, hub interfaces.Hub, parentCtx context.Context) *Room {
	// Crear un contexto derivado que se pueda cancelar independientemente
	ctx, cancel := context.WithCancel(parentCtx)

	return &Room{
		ID:          id,
		Hub:         hub,
		Clients:     make(map[interfaces.Client]bool),
		GameState:   game.NewGameState(),
		Register:    make(chan interfaces.Client),
		Unregister:  make(chan interfaces.Client),
		Broadcast:   make(chan []byte),
		ReceiveMove: make(chan *models.PlayerMove),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Close cancela el contexto y libera recursos
func (r *Room) Close() {
	r.cancel()
	// No cerramos los canales aquí, porque podría haber goroutines escribiendo en ellos
	// La cancelación del contexto debería ser suficiente para que salgan de sus bucles
	logger.Info("Sala cerrada", logger.Fields{"roomID": r.ID})
}

// Run inicia el bucle principal de la sala
func (r *Room) Run() {
	defer func() {
		// Cleanup cuando Run termina
		logger.Info("Finalizando Room.Run, liberando recursos", logger.Fields{
			"roomID": r.ID,
		})

		// Informar a los clientes que la sala se ha cerrado
		for client := range r.Clients {
			// Desasociar el cliente de la sala
			client.SetRoom(nil)

			// Enviar mensaje de sala cerrada
			closeMsg := models.BaseMessage{Type: "ROOM_CLOSED"}
			msgBytes, _ := json.Marshal(closeMsg)

			select {
			case client.GetSendChannel() <- msgBytes:
				// Mensaje enviado con éxito
			default:
				// Error al enviar, el canal posiblemente esté cerrado
			}
		}

		// Limpiar el mapa de clientes
		r.Clients = make(map[interfaces.Client]bool)
	}()

	for {
		select {
		case <-r.ctx.Done():
			// Contexto cancelado, terminar
			logger.Info("Contexto cancelado, terminando Room.Run", logger.Fields{
				"roomID": r.ID,
			})
			return

		case client := <-r.Register:
			// Añadir cliente a r.Clients
			r.Clients[client] = true

			// Determinar cuántos jugadores hay en la sala
			playerCount := len(r.Clients)

			// Si hay más de 2 jugadores, rechazar
			if playerCount > 2 {
				errors.RoomFull(client.GetSendChannel(), client.GetID())

				// Eliminar el cliente
				delete(r.Clients, client)
				client.SetRoom(nil)
				continue
			}

			// Mejorada lógica de asignación de símbolos
			// Verificar si ya hay símbolos asignados (por si acaso)
			var symbol string

			// Si es el primer jugador o no hay símbolos asignados todavía
			if playerCount == 1 || len(r.GameState.PlayerSymbols) == 0 {
				symbol = "X" // Primer jugador siempre es X

				// Reiniciar símbolos por si hay una reconexión
				r.GameState.PlayerSymbols = make(map[string]string)
				r.GameState.PlayerSymbols[client.GetID()] = symbol

				// Enviar mensaje de espera con información de la sala
				roomInfo := models.RoomCreatedResponse{
					Type:     "WAITING_FOR_OPPONENT",
					RoomID:   r.ID,
					PlayerID: client.GetID(),
					Symbol:   symbol,
				}
				msgBytes, _ := json.Marshal(roomInfo)
				client.GetSendChannel() <- msgBytes

				logger.Info("Jugador esperando oponente", logger.Fields{
					"roomID":   r.ID,
					"clientID": client.GetID(),
					"symbol":   symbol,
				})
			} else if playerCount == 2 {
				// Para el segundo jugador, asignar el símbolo contrario al del primer jugador
				var firstPlayerSymbol string
				var firstPlayer interfaces.Client

				// Obtener el símbolo del primer jugador
				for c := range r.Clients {
					if c.GetID() != client.GetID() {
						firstPlayer = c
						firstPlayerSymbol = r.GameState.PlayerSymbols[c.GetID()]
						break
					}
				}

				// Asignar símbolo opuesto al segundo jugador
				if firstPlayerSymbol == "X" {
					symbol = "O"
				} else {
					symbol = "X"
				}

				// Guardar símbolo del segundo jugador
				r.GameState.PlayerSymbols[client.GetID()] = symbol

				// Establecer turno actual (siempre empieza X)
				r.GameState.CurrentTurnSymbol = "X"

				// Notificar al primer jugador que se unió un oponente
				playerJoinedMsg := models.PlayerJoinedResponse{
					Type:     "PLAYER_JOINED",
					PlayerID: client.GetID(),
				}
				joinedBytes, _ := json.Marshal(playerJoinedMsg)
				firstPlayer.GetSendChannel() <- joinedBytes

				// Informar al segundo jugador que se unió a la sala
				roomJoinedMsg := models.RoomJoinedResponse{
					Type:     "ROOM_JOINED",
					RoomID:   r.ID,
					PlayerID: client.GetID(),
					Symbol:   symbol,
				}
				joinedMsgBytes, _ := json.Marshal(roomJoinedMsg)
				client.GetSendChannel() <- joinedMsgBytes

				// Convertir el tablero a formato JSON para el mensaje
				boardJSON := getBoardJSON(r.GameState.Board)

				// Mensaje mejorado de inicio de juego con estado completo
				gameStartMsg := models.GameStartResponse{
					Type:        "GAME_START",
					Board:       boardJSON,
					CurrentTurn: r.GameState.CurrentTurnSymbol,
					Players:     r.GameState.PlayerSymbols,
				}
				startBytes, _ := json.Marshal(gameStartMsg)

				// Enviar mensaje GAME_START a ambos jugadores
				for c := range r.Clients {
					c.GetSendChannel() <- startBytes
				}

				logger.Info("Juego iniciado", logger.Fields{
					"roomID":        r.ID,
					"player1ID":     firstPlayer.GetID(),
					"player1Symbol": r.GameState.PlayerSymbols[firstPlayer.GetID()],
					"player2ID":     client.GetID(),
					"player2Symbol": symbol,
				})
			}

		case client := <-r.Unregister:
			if _, ok := r.Clients[client]; ok {
				// Obtener el símbolo del jugador que se va
				symbol, exists := r.GameState.PlayerSymbols[client.GetID()]

				// Eliminar cliente de r.Clients
				delete(r.Clients, client)

				// Eliminar símbolo del jugador
				if exists {
					delete(r.GameState.PlayerSymbols, client.GetID())
				}

				// Actualizar client.Room = nil
				client.SetRoom(nil)

				// Notificar al otro jugador (si existe) con PLAYER_LEFT
				if len(r.Clients) > 0 {
					playerLeftMsg := models.PlayerLeftResponse{
						Type:     "PLAYER_LEFT",
						PlayerID: client.GetID(),
					}
					msgBytes, _ := json.Marshal(playerLeftMsg)

					for c := range r.Clients {
						c.GetSendChannel() <- msgBytes

						// También enviar un mensaje GAME_OVER ya que no se puede continuar
						// si un jugador abandona
						gameOverMsg := models.GameOverResponse{
							Type:   "GAME_OVER",
							Board:  getBoardJSON(r.GameState.Board),
							Winner: c.GetID(), // El jugador que queda gana por abandono
							IsDraw: false,
						}
						overBytes, _ := json.Marshal(gameOverMsg)
						c.GetSendChannel() <- overBytes
					}

					logger.Info("Jugador abandonó la sala", logger.Fields{
						"roomID":   r.ID,
						"clientID": client.GetID(),
						"symbol":   symbol,
					})
				}

				// Si la sala queda vacía, auto-destruirse
				if len(r.Clients) == 0 {
					logger.Info("Sala vacía, eliminando", logger.Fields{"roomID": r.ID})

					// Verificar si el Hub tiene método para eliminar salas
					hubWithDelete, ok := r.Hub.(interface {
						DeleteRoom(roomID string)
					})

					if ok {
						// Informar al Hub que elimine esta sala
						hubWithDelete.DeleteRoom(r.ID)
					}
				}
			}

		case message := <-r.Broadcast:
			// Iterar sobre r.Clients y enviar el mensaje a client.Send
			for client := range r.Clients {
				select {
				case client.GetSendChannel() <- message:
					// Mensaje enviado con éxito
				default:
					// Error al enviar, cliente probablemente desconectado
					close(client.GetSendChannel())
					delete(r.Clients, client)
				}
			}

		case moveReq := <-r.ReceiveMove:
			// Obtener client y moveData del PlayerMove
			moveClient, ok := moveReq.Client.(interfaces.Client)
			if !ok {
				logger.Error("Cliente en ReceiveMove no es del tipo correcto", nil)
				continue
			}

			moveData := moveReq.MoveData

			// Obtener el símbolo del cliente
			playerSymbol, ok := r.GameState.PlayerSymbols[moveClient.GetID()]
			if !ok {
				// Cliente no registrado en el juego
				errors.NotInGame(moveClient.GetSendChannel(), moveClient.GetID())
				continue
			}

			// Validar si es el turno del cliente
			if r.GameState.CurrentTurnSymbol != playerSymbol {
				// No es el turno de este jugador
				errors.NotYourTurn(moveClient.GetSendChannel(), moveClient.GetID())
				continue
			}

			// Aplicar el movimiento
			err := game.ApplyMove(r.GameState, playerSymbol, moveData.Row, moveData.Col)
			if err != nil {
				// Movimiento inválido
				errors.InvalidMove(moveClient.GetSendChannel(), err.Error(), moveClient.GetID())
				continue
			}

			// Obtener el tablero en formato JSON
			boardJSON := getBoardJSON(r.GameState.Board)

			// Movimiento válido, informar a todos los clientes
			updateMsg := models.GameUpdateResponse{
				Type:        "GAME_UPDATE",
				Board:       boardJSON,
				CurrentTurn: r.GameState.CurrentTurnSymbol,
				LastMove:    moveData,
			}
			updateBytes, _ := json.Marshal(updateMsg)

			// Enviar actualización a todos los jugadores
			for client := range r.Clients {
				client.GetSendChannel() <- updateBytes
			}

			logger.Info("Movimiento realizado", logger.Fields{
				"roomID":   r.ID,
				"clientID": moveClient.GetID(),
				"symbol":   playerSymbol,
				"row":      moveData.Row,
				"col":      moveData.Col,
			})

			// Si el juego ha terminado, enviar mensaje adicional
			if r.GameState.IsGameOver {
				var winner string
				isDraw := false

				if r.GameState.Winner != "" {
					// Encontrar el ID del jugador ganador basado en su símbolo
					for clientID, symbol := range r.GameState.PlayerSymbols {
						if symbol == r.GameState.Winner {
							winner = clientID
							break
						}
					}
					logger.Info("Juego terminado con ganador", logger.Fields{
						"roomID":    r.ID,
						"winnerID":  winner,
						"winSymbol": r.GameState.Winner,
					})
				} else {
					isDraw = true
					logger.Info("Juego terminado en empate", logger.Fields{"roomID": r.ID})
				}

				// Enviar mensaje GAME_OVER con información detallada
				endMsg := models.GameOverResponse{
					Type:   "GAME_OVER",
					Board:  boardJSON,
					Winner: winner,
					IsDraw: isDraw,
				}
				endBytes, _ := json.Marshal(endMsg)

				for client := range r.Clients {
					client.GetSendChannel() <- endBytes
				}

				// Task 33: Programar la eliminación de la sala después de que el juego termina
				// ya que no se espera más actividad en ella
				logger.Info("Juego terminado, programando eliminación de sala", logger.Fields{"roomID": r.ID})

				// Verificar si el Hub tiene método para eliminar salas
				hubWithDelete, ok := r.Hub.(interface {
					DeleteRoom(roomID string)
				})

				if ok {
					// Informar al Hub que elimine esta sala
					hubWithDelete.DeleteRoom(r.ID)
				}
			}
		}
	}
}

// getBoardJSON convierte el tablero del juego a formato JSON
func getBoardJSON(board [3][3]string) [][]string {
	return [][]string{
		{board[0][0], board[0][1], board[0][2]},
		{board[1][0], board[1][1], board[1][2]},
		{board[2][0], board[2][1], board[2][2]},
	}
}

// Este paquete será implementado en la Fase 3
