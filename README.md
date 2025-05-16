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
    "roomID": "ad275651-fb84-4f89-92f0-7a77299e8645"
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
    "playerID": "your-player-id",
    "gameState": "[[\"X\",\"\",\"\"],[\"O\",\"\",\"\"],[\"X\",\"\",\"\"]]"
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

### Player Reconnected
Sent to players when another player reconnects to the game:
```json
{
  "type": "PLAYER_RECONNECTED",
  "payload": {
    "playerID": "player-id-who-reconnected"
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

## Player Reconnection

The server supports automatic player reconnection:

- If a player disconnects (browser refresh, network issue, etc.), they can reconnect by joining the same room again
- The server will recognize the player based on their ID and allow them to rejoin even if the room appears full
- Upon reconnection, the player will receive a complete sequence of messages to restore their game state:
  1. A `ROOM_JOINED` message with their room and player information, including the current game state
  2. A `GAME_START` message with the complete board state and player mapping
  3. A `GAME_UPDATE` message with the current game state
- The opponent will receive a `PLAYER_RECONNECTED` notification
- Player symbols and game progress are preserved during reconnection

To reconnect:
1. Establish a new WebSocket connection
2. Send a `JOIN_ROOM` message with the same room ID as before
3. Process the sequence of state restoration messages
4. Continue play from the current game state

This ensures games can continue even if temporary connection issues occur.

## Frontend Implementation Guide for Reconnection

To properly implement reconnection handling in your frontend application:

1. **Persist connection data locally**:
   - Store the room ID and player ID in localStorage or sessionStorage
   - Example: `localStorage.setItem('ttt_roomId', roomId)`

2. **Initialize WebSocket connection**:
   - Create a function to establish and configure the WebSocket connection
   - Example:
     ```javascript
     let ws = null;
     
     function connectWebSocket() {
       return new Promise((resolve, reject) => {
         // Close existing connection if any
         if (ws) {
           ws.close();
         }
         
         // Create new connection
         ws = new WebSocket('ws://your-server.com:8080/ws');
         
         // Set up event handlers
         ws.onopen = () => {
           console.log('WebSocket connection established');
           resolve(ws);
         };
         
         ws.onerror = (error) => {
           console.error('WebSocket error:', error);
           reject(error);
         };
         
         ws.onmessage = (event) => {
           const message = JSON.parse(event.data);
           handleMessage(message);
         };
         
         ws.onclose = (event) => {
           console.log('WebSocket connection closed', event);
           if (!event.wasClean) {
             // Connection lost unexpectedly
             startReconnectionProcess();
           }
         };
       });
     }
     
     function sendMessage(type, payload = {}) {
       if (ws && ws.readyState === WebSocket.OPEN) {
         ws.send(JSON.stringify({ type, payload }));
       } else {
         console.error('WebSocket not connected');
       }
     }
     
     function sendJoinRoomMessage(roomId) {
       sendMessage('JOIN_ROOM', { roomId });
     }
     ```

3. **Implement reconnection logic**:
   - Set up automatic reconnection with exponential backoff
   - Example:
     ```javascript
     function reconnect(attempt = 0) {
       const maxAttempts = 5;
       const delay = Math.min(1000 * Math.pow(2, attempt), 30000);
       
       if (attempt < maxAttempts) {
         // Set reconnecting flag to true
         isReconnecting = true;
         reconnectionStep = 0;
         
         showReconnectionStatus(`Reconnecting (attempt ${attempt + 1}/${maxAttempts})...`);
         
         setTimeout(() => {
           connectWebSocket().then(() => {
             // If connection successful
             const roomId = localStorage.getItem('ttt_roomId');
             if (roomId) {
               // Rejoin the same room
               sendJoinRoomMessage(roomId);
             } else {
               // Not in a game, reset reconnection state
               isReconnecting = false;
             }
           }).catch(() => {
             reconnect(attempt + 1);
           });
         }, delay);
       } else {
         // Max attempts reached
         isReconnecting = false;
         showReconnectionStatus("Reconnection failed. Please try manually reconnecting.");
       }
     }
     ```

4. **Handle reconnection responses**:
   - Add a handler for the new `PLAYER_RECONNECTED` message type
   - Update the UI to indicate when the opponent has reconnected
   - Example:
     ```javascript
     function handleMessage(message) {
       switch (message.type) {
         case "PLAYER_RECONNECTED":
           showNotification(`Player ${message.payload.playerID} has reconnected`);
           break;
         // other message handlers
       }
     }
     ```

5. **Maintain game state**:
   - Process the reconnection message sequence in order:
     1. `ROOM_JOINED`: Update local room and player information
     2. `GAME_START`: Initialize the game board and player mappings
     3. `GAME_UPDATE`: Apply current game state and turn information
   - Example:
     ```javascript
     // Store references to reconnection state
     let isReconnecting = false;
     let reconnectionStep = 0;
     
     function handleMessage(message) {
       // During reconnection, messages arrive in a specific sequence
       if (isReconnecting) {
         switch (message.type) {
           case "ROOM_JOINED":
             reconnectionStep = 1;
             updateRoomInfo(message.payload);
             break;
             
           case "GAME_START":
             reconnectionStep = 2;
             initializeGame(message.payload);
             break;
             
           case "GAME_UPDATE":
             reconnectionStep = 3;
             updateGameState(message.payload);
             if (reconnectionStep === 3) {
               // Reconnection complete
               isReconnecting = false;
               showNotification("Reconnection successful!");
             }
             break;
         }
         return;
       }
       
       // Regular message handling...
     }
     ```

6. **User feedback**:
   - Show reconnection status to users (attempting reconnection, reconnected, etc.)
   - Allow manual reconnection attempts if automatic reconnection fails

This approach ensures that players can seamlessly continue their games after connection issues, improving the overall user experience.