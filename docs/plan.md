### Fase 0: Configuración del Proyecto
1.  **[CORE]** Inicializar el módulo Go: `go mod init nvivas/backend/tictactoe-go-server`
2.  **[CORE]** Añadir dependencia de WebSocket: `go get github.com/gorilla/websocket`
3.  **[UTIL]** Crear estructura básica de directorios: `/cmd/server/main.go`, `/internal/game`, `/internal/hub`, `/internal/client`, `/internal/room`, `/pkg/models` (o similar).
4.  **[CONFIG]** Definir configuración básica (puerto del servidor) - puede ser hardcodeado inicialmente o usar flags/env vars.

### Fase 1: Conexión WebSocket Básica y Estructuras de Cliente
5.  **[CLIENT]** Definir la estructura `Client` en `internal/client/client.go`:
    * `ID string`
    * `Hub *hub.Hub` (puntero, se definirá `Hub` después)
    * `Room *room.Room` (puntero, se definirá `Room` después)
    * `Conn *websocket.Conn`
    * `Send chan []byte` (canal para mensajes salientes)
6.  **[HTTP]** En `main.go`, crear un servidor HTTP básico (`http.ListenAndServe`).
7.  **[HTTP]** Configurar el `websocket.Upgrader` (con `CheckOrigin` permisivo para desarrollo).
8.  **[HTTP]** Crear un manejador HTTP para `/ws` (`handleConnections`):
    * Actualizar la conexión HTTP a WebSocket.
    * Crear una instancia de `Client` (inicialmente `Hub` y `Room` pueden ser `nil`).
    * Asignar un ID único al cliente (e.g., `uuid.NewString()`).
    * Registrar al cliente en el `Hub` (esto se conectará más tarde).
    * Lanzar `client.readPump()` y `client.writePump()` como goroutines.
9.  **[CLIENT]** Implementar `Client.readPump()`:
    * Bucle infinito para `c.Conn.ReadMessage()`.
    * Por ahora, solo imprimir los mensajes recibidos.
    * Manejar errores de lectura y desconexión:
        * Llamar a `c.Hub.Unregister <- c` (se implementará `Hub`).
        * Cerrar `c.Conn`.
        * Cerrar `c.Send` (o usar un `context` para señalizar a `writePump`).
10. **[CLIENT]** Implementar `Client.writePump()`:
    * Bucle infinito con `select`:
        * Caso para leer de `c.Send`: escribir mensaje en `c.Conn`.
        * Caso para un `ticker` (e.g., cada 50 segundos) para enviar pings (`c.Conn.WriteMessage(websocket.PingMessage, nil)`).
    * Manejar errores de escritura (cerrar conexión).
    * Asegurar que la goroutine termine si `c.Send` se cierra o la conexión falla.
11. **[MODELOS]** Definir estructuras básicas para mensajes JSON en `/pkg/models/message.go` (o similar):
    * `BaseMessage { Type string }`
    * `Envelope { Type string; Payload json.RawMessage }` para deserialización inicial.

### Fase 2: Implementación del Hub (Gestor de Clientes y Salas)
12. **[HUB]** Definir la estructura `Hub` en `internal/hub/hub.go`:
    * `Clients map[*client.Client]bool` (para rastrear todos los clientes conectados al servidor)
    * `Rooms map[string]*room.Room` (para rastrear salas activas)
    * `Register chan *client.Client`
    * `Unregister chan *client.Client`
    * `CreateRoom chan *client.Client`
    * `JoinRoom chan *JoinRequest` (definir `JoinRequest struct { Client *client.Client; RoomID string }`)
    * `broadcast chan []byte` (opcional, para mensajes a todos los clientes del hub)
13. **[HUB]** Implementar `NewHub()` para inicializar un `Hub`.
14. **[HUB]** Implementar `Hub.Run()`:
    * Bucle infinito con `select` para escuchar en los canales `Register`, `Unregister`, `CreateRoom`, `JoinRoom`.
    * Lógica para `Register`: añadir cliente a `h.Clients`.
    * Lógica para `Unregister`:
        * Si el cliente está en `h.Clients`, eliminarlo.
        * Cerrar `client.Send` (si no se cerró ya).
        * Si el cliente estaba en una sala, notificar a la sala (se detallará).
    * Lógica para `CreateRoom` (inicial, se expandirá):
        * Crear un `RoomID` único.
        * Crear una instancia de `Room` (se definirá `Room` después).
        * Almacenar `room` en `h.Rooms[RoomID]`.
        * Lanzar `room.Run()` como goroutine.
        * Enviar el cliente creador al canal `Register` de la nueva `Room`.
        * (Opcional) Enviar mensaje `ROOM_CREATED` al cliente.
    * Lógica para `JoinRoom` (inicial, se expandirá):
        * Buscar `Room` por `RoomID` en `h.Rooms`.
        * Si existe y hay espacio: enviar cliente al canal `Register` de la `Room`.
        * Si no: enviar mensaje de error al cliente.
15. **[MAIN]** En `main.go`, crear una instancia global del `Hub` y lanzar `hub.Run()` como goroutine.
16. **[HTTP]** En `handleConnections`, después de crear `Client`, enviarlo a `hub.Register <- newClient`.
17. **[CLIENT]** En `Client.readPump()`, al desconectar, enviar `c.Hub.Unregister <- c`.

### Fase 3: Implementación de la Sala de Juego (Room)
18. **[GAME]** Definir la lógica base del juego en `internal/game/logic.go`:
    * `Board [3][3]string`
    * `GameState struct { Board Board; CurrentTurnSymbol string; PlayerSymbols map[string]string; Winner string; IsGameOver bool; IsDraw bool }` (usar `string` para `PlayerSymbols` key si es `clientID`)
    * `NewGameState() (*GameState)`
    * `ApplyMove(gs *GameState, playerSymbol string, row, col int) error`
    * `CheckWin(gs *GameState) (winnerSymbol string, isDraw bool)`
19. **[ROOM]** Definir la estructura `Room` en `internal/room/room.go`:
    * `ID string`
    * `Hub *hub.Hub`
    * `Clients map[*client.Client]bool` (o `[2]*client.Client` y manejar `nil`)
    * `GameState *game.GameState`
    * `Register chan *client.Client`
    * `Unregister chan *client.Client`
    * `Broadcast chan []byte` (o `chan models.GameMessage`)
    * `ReceiveMove chan *models.PlayerMove` (definir `PlayerMove struct { Client *client.Client; MoveData models.MovePayload }`)
20. **[ROOM]** Implementar `NewRoom(id string, hub *hub.Hub)` para inicializar una `Room`.
21. **[ROOM]** Implementar `Room.Run()`:
    * Bucle infinito con `select`:
        * Caso `Register`:
            * Añadir cliente a `r.Clients`.
            * Asignar símbolo (X o O). Almacenar en `r.GameState.PlayerSymbols[client.ID] = symbol`.
            * Actualizar `client.Room = r`.
            * Si es el primer jugador: enviar `ROOM_CREATED` (o un mensaje similar de espera).
            * Si es el segundo jugador:
                * Iniciar el juego (`r.GameState.CurrentTurnSymbol` = 'X').
                * Enviar `PLAYER_JOINED` al primer jugador.
                * Enviar `ROOM_JOINED` (o `GAME_START`) con el estado inicial a ambos.
        * Caso `Unregister`:
            * Eliminar cliente de `r.Clients`.
            * Actualizar `client.Room = nil`.
            * Notificar al otro jugador (si existe) con `PLAYER_LEFT`. El juego podría terminar.
            * Si la sala queda vacía, auto-destruirse (enviar señal al `Hub` para eliminarla de `Hub.Rooms`).
        * Caso `Broadcast`:
            * Iterar sobre `r.Clients` y enviar el mensaje a `client.Send`.
        * Caso `ReceiveMove`:
            * Obtener `client` y `moveData` del `PlayerMove`.
            * Validar si es el turno de `client` (`r.GameState.PlayerSymbols[client.ID] == r.GameState.CurrentTurnSymbol`).
            * Llamar a `game.ApplyMove()`.
            * Si es válido:
                * Llamar a `game.CheckWin()`.
                * Actualizar `r.GameState` (tablero, ganador, fin de juego, siguiente turno).
                * Crear mensaje `GAME_UPDATE`.
                * Enviar `GAME_UPDATE` a `r.Broadcast`.
            * Si no es válido: enviar mensaje `ERROR` solo a `client.Send`.
22. **[CLIENT]** En `Client.readPump()`, deserializar mensajes.
    * Si `type == "CREATE_ROOM"`: `c.Hub.CreateRoom <- c`.
    * Si `type == "JOIN_ROOM"`: `c.Hub.JoinRoom <- &hub.JoinRequest{Client: c, RoomID: payload.RoomID}`.
    * Si `type == "MAKE_MOVE"`:
        * Verificar que `c.Room != nil`.
        * `c.Room.ReceiveMove <- &models.PlayerMove{Client: c, MoveData: payload.Move}`.

### Fase 4: Mensajería Detallada y Flujo de Juego
23. **[MODELOS]** Definir todas las estructuras de mensajes JSON (cliente-servidor y servidor-cliente) de forma completa en `/pkg/models`.
    * Ej: `CreateRoomPayload`, `JoinRoomPayload`, `MakeMovePayload`.
    * Ej: `RoomCreatedResponse`, `GameUpdateResponse`, `ErrorResponse`.
24. **[SERIALIZACIÓN]** Asegurar que todos los mensajes se serializan (`json.Marshal`) antes de enviar por `client.Send` y `room.Broadcast`, y se deserializan (`json.Unmarshal`) en `readPump`.
25. **[ROOM]** Refinar la lógica de asignación de símbolos (X/O) en `Room.Run()` al registrar jugadores.
26. **[ROOM]** Enviar mensajes `GAME_START` cuando el segundo jugador se une.
27. **[ROOM]** Enviar mensajes `GAME_OVER` (con `winner` o `draw`) cuando el juego termina.
28. **[HUB]** En `Hub.Run()` / `CreateRoom`:
    * Enviar mensaje `ROOM_CREATED { roomID, playerSymbol, playerID }` al creador.
29. **[HUB]** En `Hub.Run()` / `JoinRoom`:
    * Si se une con éxito, la `Room` se encargará de notificar con `ROOM_JOINED` y `PLAYER_JOINED`.
    * Si falla (sala llena/no existe), el `Hub` envía `ERROR { message: "Sala no encontrada o llena" }`.

### Fase 5: Robustez y Mejoras
30. **[ERROR]** Implementar manejo de errores consistente. Usar mensajes `ERROR` para el cliente.
31. **[LOGGING]** Añadir logging estructurado (e.g., `log/slog` o `logrus`) en puntos clave (conexiones, creación de salas, movimientos, errores).
32. **[CLEANUP]** Asegurar que todas las goroutines terminen limpiamente y los canales se cierren cuando sea apropiado para evitar fugas de goroutines.
    * Considerar el uso de `context.Context` para la cancelación en cascada.
33. **[HUB/ROOM]** Implementar la lógica para que una `Room` se elimine del `Hub` cuando quede vacía o el juego termine y no haya más actividad esperada.
    * La `Room` puede enviar un mensaje a un canal `deleteRoomChan` en el `Hub`.
34. **[TESTING]** Escribir pruebas unitarias para:
    * Lógica del juego (`internal/game`).
    * Manejo de estado básico en `Room` (sin WebSockets).
35. **[CONFIG]** Permitir configuración del puerto a través de variables de entorno o flags.
36. **[SECURITY]** Considerar límites de tamaño de mensaje en WebSocket para prevenir DoS. `Upgrader.ReadBufferSize`, `Upgrader.WriteBufferSize`. `Conn.SetReadLimit()`.
37. **[DOCS]** Documentar la API de mensajes WebSocket (qué enviar, qué esperar).

Este plan es bastante granular y debería guiarte a través del proceso de construcción. Cada paso se basa en el anterior, permitiendo un desarrollo incremental y pruebas a medida que avanzas. ¡Mucha suerte con la implementación!
