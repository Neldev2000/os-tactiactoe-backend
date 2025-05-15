package errors

import (
	"encoding/json"
	"nvivas/backend/tictactoe-go-server/internal/logger"
	"nvivas/backend/tictactoe-go-server/pkg/models"
)

// Error types
const (
	ErrorRoomFull           = "ERROR_ROOM_FULL"
	ErrorRoomNotFound       = "ERROR_ROOM_NOT_FOUND"
	ErrorNotInRoom          = "ERROR_NOT_IN_ROOM"
	ErrorNotInGame          = "ERROR_NOT_IN_GAME"
	ErrorNotYourTurn        = "ERROR_NOT_YOUR_TURN"
	ErrorInvalidMove        = "ERROR_INVALID_MOVE"
	ErrorInvalidMessage     = "ERROR_INVALID_MESSAGE"
	ErrorInvalidPayload     = "ERROR_INVALID_PAYLOAD"
	ErrorInternal           = "ERROR_INTERNAL"
	ErrorUnknownMessageType = "ERROR_UNKNOWN_MESSAGE_TYPE"
)

// SendError sends a structured error message to the client
func SendError(channel chan []byte, errorType, message string, clientID string) {
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
		return
	}

	// Log the error
	logger.Error(message, logger.Fields{
		"errorType": errorType,
		"clientID":  clientID,
	})

	// Send to client
	channel <- msgBytes
}

// RoomFull creates a room full error
func RoomFull(channel chan []byte, clientID string) {
	SendError(channel, ErrorRoomFull, "La sala ya está llena", clientID)
}

// RoomNotFound creates a room not found error
func RoomNotFound(channel chan []byte, clientID string) {
	SendError(channel, ErrorRoomNotFound, "La sala solicitada no existe", clientID)
}

// NotInRoom creates a not in room error
func NotInRoom(channel chan []byte, clientID string) {
	SendError(channel, ErrorNotInRoom, "No estás en ninguna sala", clientID)
}

// NotInGame creates a not in game error
func NotInGame(channel chan []byte, clientID string) {
	SendError(channel, ErrorNotInGame, "No eres parte de este juego", clientID)
}

// NotYourTurn creates a not your turn error
func NotYourTurn(channel chan []byte, clientID string) {
	SendError(channel, ErrorNotYourTurn, "No es tu turno", clientID)
}

// InvalidMove creates an invalid move error
func InvalidMove(channel chan []byte, message string, clientID string) {
	SendError(channel, ErrorInvalidMove, message, clientID)
}

// InvalidMessage creates an invalid message error
func InvalidMessage(channel chan []byte, clientID string) {
	SendError(channel, ErrorInvalidMessage, "Formato de mensaje inválido", clientID)
}

// InvalidPayload creates an invalid payload error
func InvalidPayload(channel chan []byte, context string, clientID string) {
	SendError(channel, ErrorInvalidPayload, "Datos inválidos: "+context, clientID)
}

// Internal creates an internal error
func Internal(channel chan []byte, clientID string) {
	SendError(channel, ErrorInternal, "Error interno del servidor", clientID)
}

// UnknownMessageType creates an unknown message type error
func UnknownMessageType(channel chan []byte, msgType string, clientID string) {
	SendError(channel, ErrorUnknownMessageType, "Tipo de mensaje desconocido: "+msgType, clientID)
}
