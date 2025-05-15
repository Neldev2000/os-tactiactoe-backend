package game

import (
	"errors"
	"fmt"
)

// Board representa un tablero de 3x3 para el juego
type Board [3][3]string

// GameState contiene el estado completo del juego
type GameState struct {
	Board             Board             // Tablero actual
	CurrentTurnSymbol string            // Símbolo del jugador actual ("X" o "O")
	PlayerSymbols     map[string]string // Mapa de ID de cliente a símbolo
	Winner            string            // Símbolo del ganador, vacío si no hay ganador
	IsGameOver        bool              // Indica si el juego ha terminado
	IsDraw            bool              // Indica si el juego terminó en empate
}

// NewGameState crea un nuevo estado de juego inicializado
func NewGameState() *GameState {
	return &GameState{
		Board:             Board{},                 // Tablero vacío
		CurrentTurnSymbol: "X",                     // X siempre comienza
		PlayerSymbols:     make(map[string]string), // Mapa vacío de jugadores
		Winner:            "",                      // Sin ganador inicial
		IsGameOver:        false,                   // Juego no terminado
		IsDraw:            false,                   // No es empate
	}
}

// ApplyMove aplica un movimiento al estado del juego
func ApplyMove(gs *GameState, playerSymbol string, row, col int) error {
	// Verificar si el juego ya terminó
	if gs.IsGameOver {
		return errors.New("el juego ya ha terminado")
	}

	// Verificar si es el turno del jugador
	if gs.CurrentTurnSymbol != playerSymbol {
		return fmt.Errorf("no es el turno de %s, es el turno de %s", playerSymbol, gs.CurrentTurnSymbol)
	}

	// Verificar si la posición está dentro del tablero
	if row < 0 || row > 2 || col < 0 || col > 2 {
		return errors.New("posición fuera del tablero")
	}

	// Verificar si la casilla está vacía
	if gs.Board[row][col] != "" {
		return errors.New("casilla ya ocupada")
	}

	// Aplicar el movimiento
	gs.Board[row][col] = playerSymbol

	// Comprobar si hay un ganador o empate
	winner, isDraw := CheckWin(gs)

	if winner != "" {
		gs.Winner = winner
		gs.IsGameOver = true
	} else if isDraw {
		gs.IsDraw = true
		gs.IsGameOver = true
	} else {
		// Cambiar el turno al otro jugador
		if gs.CurrentTurnSymbol == "X" {
			gs.CurrentTurnSymbol = "O"
		} else {
			gs.CurrentTurnSymbol = "X"
		}
	}

	return nil
}

// CheckWin verifica si hay un ganador o empate
func CheckWin(gs *GameState) (winnerSymbol string, isDraw bool) {
	board := gs.Board

	// Comprobar filas
	for i := 0; i < 3; i++ {
		if board[i][0] != "" && board[i][0] == board[i][1] && board[i][1] == board[i][2] {
			return board[i][0], false
		}
	}

	// Comprobar columnas
	for i := 0; i < 3; i++ {
		if board[0][i] != "" && board[0][i] == board[1][i] && board[1][i] == board[2][i] {
			return board[0][i], false
		}
	}

	// Comprobar diagonal principal
	if board[0][0] != "" && board[0][0] == board[1][1] && board[1][1] == board[2][2] {
		return board[0][0], false
	}

	// Comprobar diagonal secundaria
	if board[0][2] != "" && board[0][2] == board[1][1] && board[1][1] == board[2][0] {
		return board[0][2], false
	}

	// Comprobar empate (si no hay casillas vacías)
	isDraw = true
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if board[i][j] == "" {
				isDraw = false
				break
			}
		}
		if !isDraw {
			break
		}
	}

	return "", isDraw
}
