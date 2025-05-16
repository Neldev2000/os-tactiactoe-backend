package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"

	"nvivas/backend/tictactoe-go-server/internal/client"
	"nvivas/backend/tictactoe-go-server/internal/hub"
	"nvivas/backend/tictactoe-go-server/internal/logger"
)

const (
	defaultPort     = "8080"
	shutdownTimeout = 5 * time.Second

	// Configuración de seguridad para WebSockets
	wsReadBufferSize  = 1024 * 4         // 4KB
	wsWriteBufferSize = 1024 * 4         // 4KB
	wsMaxMessageSize  = 1024 * 16        // 16KB - límite razonable para mensajes de juego
	wsPongWait        = 60 * time.Second // Tiempo máximo para recibir pong

	// Valores por defecto para límites de recursos
	defaultMaxTotalClients = 1000 // Valor predeterminado para el máximo de clientes
	defaultMaxRooms        = 500  // Valor predeterminado para el máximo de salas
)

// Instancia global del Hub
var mainHub *hub.Hub

// Contexto global
var ctx context.Context
var cancel context.CancelFunc

// Límites de recursos
var maxTotalClients int
var maxRooms int

var upgrader = websocket.Upgrader{
	ReadBufferSize:  wsReadBufferSize,
	WriteBufferSize: wsWriteBufferSize,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Implementar una validación de origen adecuada
		return true
	},
}

// handleConnections maneja las conexiones WebSocket entrantes
func handleConnections(w http.ResponseWriter, r *http.Request) {
	// Verificar si ya hemos alcanzado el límite de clientes
	if len(mainHub.Clients) >= maxTotalClients {
		logger.Warn("Límite de clientes alcanzado, rechazando nueva conexión", logger.Fields{
			"currentClients": len(mainHub.Clients),
			"maxClients":     maxTotalClients,
		})
		http.Error(w, "Server is at capacity. Please try again later.", http.StatusServiceUnavailable)
		return
	}

	// Actualizar la conexión HTTP a WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("Error al actualizar la conexión WebSocket", logger.Fields{
			"error": err.Error(),
			"path":  r.URL.Path,
		})
		return
	}

	// Configurar límites y timeouts en la conexión para prevenir DoS
	conn.SetReadLimit(wsMaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		// Renovar el deadline cuando recibimos un pong
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	// Crear una instancia de Client con el contexto global
	c := client.NewClient(uuid.NewString(), mainHub, conn, ctx)

	// Registrar al cliente en el Hub
	mainHub.Register <- c

	// Iniciar goroutines para manejar la comunicación
	go c.ReadPump()
	go c.WritePump()

	logger.Info("Nueva conexión establecida", logger.Fields{
		"clientID": c.GetID(),
		"remote":   conn.RemoteAddr().String(),
	})
}

// loadEnv carga variables de entorno desde .env si existe
func loadEnv() {
	// Intentar cargar .env, pero no fallar si no existe
	_ = godotenv.Load()

	// Cargar límites de recursos desde variables de entorno o usar valores predeterminados
	maxTotalClients = getEnvInt("TICTACTOE_MAX_CLIENTS", defaultMaxTotalClients)
	maxRooms = getEnvInt("TICTACTOE_MAX_ROOMS", defaultMaxRooms)

	logger.Info("Límites de recursos configurados", logger.Fields{
		"maxTotalClients": maxTotalClients,
		"maxRooms":        maxRooms,
	})
}

// getEnvInt obtiene un valor entero de una variable de entorno o devuelve el valor predeterminado
func getEnvInt(name string, defaultValue int) int {
	valueStr := os.Getenv(name)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		logger.Warn("Valor inválido para variable de entorno, usando valor predeterminado", logger.Fields{
			"variable":            name,
			"valor":               valueStr,
			"valorPredeterminado": defaultValue,
			"error":               err.Error(),
		})
		return defaultValue
	}

	return value
}

// getPort determina el puerto del servidor basado en flags, variables de entorno
// o el valor por defecto
func getPort() string {
	// 1. Verificar flag de línea de comandos
	var portFlag string
	flag.StringVar(&portFlag, "port", "", "Puerto del servidor (ej: 8080)")
	flag.StringVar(&portFlag, "p", "", "Puerto del servidor (ej: 8080) (alias corto)")
	flag.Parse()

	if portFlag != "" {
		return portFlag
	}

	// 2. Verificar variable de entorno
	if envPort := os.Getenv("TICTACTOE_PORT"); envPort != "" {
		return envPort
	}

	// 3. Usar el puerto por defecto
	return defaultPort
}

func main() {
	// Inicializar el logger
	logger.Initialize()

	// Cargar variables de entorno
	loadEnv()

	// Crear contexto cancelable
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	// Obtener el puerto de configuración
	port := getPort()

	// Crear e iniciar el Hub con el contexto global
	mainHub = hub.NewHub()
	mainHub.SetLimits(maxRooms) // Configurar límite de salas
	go mainHub.Run()

	logger.Info("Hub iniciado", nil)

	// Configurar rutas
	http.HandleFunc("/ws", handleConnections)

	// Configurar servidor con opciones de cierre controlado
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      http.DefaultServeMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Canal para señales del sistema
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Iniciar el servidor en una goroutine separada
	go func() {
		logger.Info("Iniciando servidor", logger.Fields{
			"port": port,
			"security": logger.Fields{
				"wsMaxMessageSize":  wsMaxMessageSize,
				"wsReadBufferSize":  wsReadBufferSize,
				"wsWriteBufferSize": wsWriteBufferSize,
			},
		})
		fmt.Printf("Servidor iniciado en http://localhost:%s\n", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Error al iniciar el servidor", logger.Fields{"error": err.Error()})
		}
	}()

	// Esperar señal de interrupción
	<-done
	logger.Info("Recibida señal de apagado, iniciando shutdown", nil)

	// Cancelar contexto para que todas las goroutines terminen
	cancel()

	// Crear contexto con timeout para el shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	// Cerrar el hub
	mainHub.Close()

	// Cerrar servidor HTTP con timeout
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error durante el shutdown del servidor", logger.Fields{"error": err.Error()})
	}

	logger.Info("Servidor detenido correctamente", nil)
}
