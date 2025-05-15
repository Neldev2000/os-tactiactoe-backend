# Tic-Tac-Toe Backend WebSocket API Documentation

This document describes how to connect to the Tic-Tac-Toe backend server and interact with it using WebSockets.

## Connection

Connect to the WebSocket server at:
```
ws://[SERVER_HOST]:[PORT]/ws
```

## Message Format

All messages follow this JSON format:
```json
{
  "type": "MESSAGE_TYPE",
  "payload": {
    // Message-specific data
  }
}
```

## Client → Server Messages

### Create a Room
Request to create a new game room:
```json
{
  "type": "CREATE_ROOM",
  "payload": {}
}
```

### Join a Room
Request to join an existing room:
```json
{
  "type": "JOIN_ROOM",
  "payload": {
    "roomID": "room-identifier"
  }
}
```

### Make a Move
Make a move in the game:
```json
{
  "type": "MAKE_MOVE",
  "payload": {
    "move": {
      "row": 0,
      "col": 0
    }
  }
}
```

### List Rooms
Request the list of available rooms:
```json
{
  "type": "LIST_ROOMS",
  "payload": {}
}
```

## Server → Client Messages

### Room Created
Sent after successfully creating a room:
```json
{
  "type": "ROOM_CREATED",
  "payload": {
    "roomID": "room-identifier",
    "playerSymbol": "X",
    "playerID": "your-player-id"
  }
}
```

### Room Joined
Sent after successfully joining a room:
```json
{
  "type": "ROOM_JOINED",
  "payload": {
    "roomID": "room-identifier",
    "playerSymbol": "O",
    "playerID": "your-player-id"
  }
}
```

### Player Joined
Sent to the existing player when another player joins:
```json
{
  "type": "PLAYER_JOINED",
  "payload": {
    "playerID": "opponent-player-id"
  }
}
```

### Game Start
Sent to both players when the game is ready to start:
```json
{
  "type": "GAME_START",
  "payload": {
    "board": [["", "", ""], ["", "", ""], ["", "", ""]],
    "currentTurn": "X"
  }
}
```

### Game Update
Sent after a valid move is made:
```json
{
  "type": "GAME_UPDATE",
  "payload": {
    "board": [["X", "", ""], ["", "", ""], ["", "", ""]],
    "currentTurn": "O"
  }
}
```

### Game Over
Sent when the game ends:
```json
{
  "type": "GAME_OVER",
  "payload": {
    "board": [["X", "X", "X"], ["O", "O", ""], ["", "", ""]],
    "winner": "X",
    "isDraw": false
  }
}
```

### Player Left
Sent when a player disconnects:
```json
{
  "type": "PLAYER_LEFT",
  "payload": {
    "playerID": "player-id-who-left"
  }
}
```

### Room List
Sent in response to a LIST_ROOMS request:
```json
{
  "type": "ROOM_LIST",
  "payload": {
    "rooms": [
      {
        "roomID": "room-identifier-1",
        "players": ["player-id-1", "player-id-2"],
        "isFull": true
      },
      {
        "roomID": "room-identifier-2",
        "players": ["player-id-3"],
        "isFull": false
      }
    ]
  }
}
```

### Error
Sent when an error occurs:
```json
{
  "type": "ERROR",
  "payload": {
    "code": "error_code",
    "message": "Error description"
  }
}
```

Common error codes:
- `invalid_message`: Message format is invalid
- `invalid_payload`: Payload format is invalid
- `unknown_message_type`: Unknown message type
- `not_in_room`: Action requires being in a room
- `invalid_move`: Move is not valid
- `not_your_turn`: Not your turn to make a move
- `room_not_found`: Room does not exist
- `room_full`: Room is already full

## Game Flow Example

1. Connect to the WebSocket server
2. Send `LIST_ROOMS` to see available rooms
3. Either:
   - Send `CREATE_ROOM` to create a new game room, or
   - Send `JOIN_ROOM` with the room ID of an existing room
4. If creating a room, receive `ROOM_CREATED` with your room ID
5. Share the room ID with the second player
6. Second player sends `JOIN_ROOM` with the room ID
7. First player receives `PLAYER_JOINED`
8. Both players receive `GAME_START`
9. Players take turns sending `MAKE_MOVE`
10. Both players receive `GAME_UPDATE` after each move
11. When game ends, both players receive `GAME_OVER`

## Connection Management

- The server sends ping messages every 54 seconds
- If you don't receive anything for 60 seconds, consider the connection lost
- Reconnect if the connection is lost and rejoin the room
