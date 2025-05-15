package models

import (
	"encoding/json"
)

// BaseMessage is the most basic message structure
type BaseMessage struct {
	Type string `json:"type"`
}

// Envelope is used for initial message deserialization
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// MovePayload contains the data for a move in the game
type MovePayload struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// PlayerMove combines a client with move data
type PlayerMove struct {
	Client   interface{} // Will be a Client implementation
	MoveData MovePayload
}

// CreateRoomPayload contains data for creating a room
type CreateRoomPayload struct {
	// Empty for now, could contain preferences later
}

// JoinRoomPayload contains data for joining a room
type JoinRoomPayload struct {
	RoomID string `json:"roomId"`
}

// MakeMovePayload contains data for making a move
type MakeMovePayload struct {
	Move MovePayload `json:"move"`
}

// RoomCreatedResponse is sent after a room is created
type RoomCreatedResponse struct {
	Type     string `json:"type"`
	RoomID   string `json:"roomId"`
	PlayerID string `json:"playerId"`
	Symbol   string `json:"symbol"`
}

// RoomJoinedResponse is sent after successfully joining a room
type RoomJoinedResponse struct {
	Type     string `json:"type"`
	RoomID   string `json:"roomId"`
	PlayerID string `json:"playerId"`
	Symbol   string `json:"symbol"`
}

// PlayerJoinedResponse is sent to the first player when a second player joins
type PlayerJoinedResponse struct {
	Type     string `json:"type"`
	PlayerID string `json:"playerId"`
}

// GameStartResponse is sent to both players when the game starts
type GameStartResponse struct {
	Type        string            `json:"type"`
	Board       [][]string        `json:"board"`
	CurrentTurn string            `json:"currentTurn"`
	Players     map[string]string `json:"players"` // map[playerID]symbol
}

// GameUpdateResponse is sent after a valid move
type GameUpdateResponse struct {
	Type        string      `json:"type"`
	Board       [][]string  `json:"board"`
	CurrentTurn string      `json:"currentTurn"`
	LastMove    MovePayload `json:"lastMove"`
}

// GameOverResponse is sent when the game ends
type GameOverResponse struct {
	Type   string     `json:"type"`
	Board  [][]string `json:"board"`
	Winner string     `json:"winner"` // PlayerID or empty for draw
	IsDraw bool       `json:"isDraw"`
}

// ErrorResponse is sent when an error occurs
type ErrorResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// PlayerLeftResponse is sent when a player leaves the game
type PlayerLeftResponse struct {
	Type     string `json:"type"`
	PlayerID string `json:"playerId"`
}

// ListRoomsPayload is empty as it doesn't need any parameters
type ListRoomsPayload struct {
	// Empty for now, could contain filters later
}

// RoomInfo contains information about a room
type RoomInfo struct {
	RoomID  string   `json:"roomId"`
	Players []string `json:"players"`
	IsFull  bool     `json:"isFull"`
}

// RoomListPayload contains the list of available rooms
type RoomListPayload struct {
	Type  string     `json:"type"`
	Rooms []RoomInfo `json:"rooms"`
}
