package room

import (
	"encoding/json"
	"log"
	"nvivas/backend/tictactoe-go-server/internal/game"
	"nvivas/backend/tictactoe-go-server/internal/interfaces"
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
}

// NewRoom crea una nueva sala de juego
func NewRoom(id string, hub interfaces.Hub) *Room {
	return &Room{
		ID:          id,
		Hub:         hub,
		Clients:     make(map[interfaces.Client]bool),
		GameState:   game.NewGameState(),
		Register:    make(chan interfaces.Client),
		Unregister:  make(chan interfaces.Client),
		Broadcast:   make(chan []byte),
		ReceiveMove: make(chan *models.PlayerMove),
	}
}

// Run inicia el bucle principal de la sala
func (r *Room) Run() {
	for {
		select {
		case client := <-r.Register:
			// Añadir cliente a r.Clients
			r.Clients[client] = true

			// Determinar cuántos jugadores hay en la sala
			playerCount := len(r.Clients)

			// Si hay más de 2 jugadores, rechazar
			if playerCount > 2 {
				errorMsg := models.ErrorResponse{
					Type:    "ERROR_ROOM_FULL",
					Message: "La sala ya está llena",
				}
				msgBytes, _ := json.Marshal(errorMsg)
				client.GetSendChannel() <- msgBytes

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

				log.Printf("Jugador %s (símbolo %s) está esperando un oponente en sala %s",
					client.GetID(), symbol, r.ID)
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
				boardJSON := [][]string{
					{r.GameState.Board[0][0], r.GameState.Board[0][1], r.GameState.Board[0][2]},
					{r.GameState.Board[1][0], r.GameState.Board[1][1], r.GameState.Board[1][2]},
					{r.GameState.Board[2][0], r.GameState.Board[2][1], r.GameState.Board[2][2]},
				}

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

				log.Printf("¡Juego iniciado en sala %s! Jugador %s (símbolo %s) vs Jugador %s (símbolo %s)",
					r.ID, firstPlayer.GetID(), r.GameState.PlayerSymbols[firstPlayer.GetID()],
					client.GetID(), symbol)
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

					log.Printf("Jugador %s (símbolo %s) abandonó la sala %s",
						client.GetID(), symbol, r.ID)
				}

				// Si la sala queda vacía, auto-destruirse
				if len(r.Clients) == 0 {
					log.Printf("Sala %s vacía, debería ser eliminada", r.ID)
					// En una implementación más completa, tendríamos:
					// r.Hub.DeleteRoom(r.ID)
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
				log.Printf("Error: Cliente en ReceiveMove no es del tipo correcto")
				continue
			}

			moveData := moveReq.MoveData

			// Obtener el símbolo del cliente
			playerSymbol, ok := r.GameState.PlayerSymbols[moveClient.GetID()]
			if !ok {
				// Cliente no registrado en el juego
				errorMsg := models.ErrorResponse{
					Type:    "ERROR_NOT_IN_GAME",
					Message: "No eres parte de este juego",
				}
				msgBytes, _ := json.Marshal(errorMsg)
				moveClient.GetSendChannel() <- msgBytes
				continue
			}

			// Validar si es el turno del cliente
			if r.GameState.CurrentTurnSymbol != playerSymbol {
				// No es el turno de este jugador
				errorMsg := models.ErrorResponse{
					Type:    "ERROR_NOT_YOUR_TURN",
					Message: "No es tu turno",
				}
				msgBytes, _ := json.Marshal(errorMsg)
				moveClient.GetSendChannel() <- msgBytes
				continue
			}

			// Aplicar el movimiento
			err := game.ApplyMove(r.GameState, playerSymbol, moveData.Row, moveData.Col)
			if err != nil {
				// Movimiento inválido
				errorMsg := models.ErrorResponse{
					Type:    "ERROR_INVALID_MOVE",
					Message: err.Error(),
				}
				msgBytes, _ := json.Marshal(errorMsg)
				moveClient.GetSendChannel() <- msgBytes
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

			log.Printf("Jugador %s (símbolo %s) hizo un movimiento en (%d,%d)",
				moveClient.GetID(), playerSymbol, moveData.Row, moveData.Col)

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
					log.Printf("¡Juego terminado en sala %s! Ganador: Jugador %s (símbolo %s)",
						r.ID, winner, r.GameState.Winner)
				} else {
					isDraw = true
					log.Printf("¡Juego terminado en sala %s! Resultado: Empate", r.ID)
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
