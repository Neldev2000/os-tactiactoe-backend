package interfaces

import "github.com/gorilla/websocket"

// Hub defines the interface for hub operations needed by clients
type Hub interface {
	// UnregisterClient removes a client from the hub
	UnregisterClient(client Client)

	// CreateRoom creates a new room with the client as the first player
	CreateRoom(client Client)

	// JoinRoom adds a client to an existing room
	JoinRoom(roomID string, client Client)
}

// Client defines the interface for client operations needed by the hub
type Client interface {
	// GetID returns the client's unique identifier
	GetID() string

	// GetSendChannel returns the client's message sending channel
	GetSendChannel() chan []byte

	// GetConnection returns the client's websocket connection
	GetConnection() *websocket.Conn

	// SetRoom sets the client's current room
	SetRoom(room interface{})

	// GetRoom gets the client's current room
	GetRoom() interface{}
}
