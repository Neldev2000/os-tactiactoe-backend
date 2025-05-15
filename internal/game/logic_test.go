package game

import (
	"testing"
)

func TestNewGameState(t *testing.T) {
	gs := NewGameState()

	// Verificar inicialización correcta
	if gs.CurrentTurnSymbol != "X" {
		t.Errorf("Turno inicial esperado 'X', se obtuvo '%s'", gs.CurrentTurnSymbol)
	}
	if gs.Winner != "" {
		t.Errorf("Ganador inicial esperado vacío, se obtuvo '%s'", gs.Winner)
	}
	if gs.IsGameOver {
		t.Error("Estado inicial de juego esperado 'no terminado'")
	}
	if gs.IsDraw {
		t.Error("Estado inicial de empate esperado 'no empate'")
	}

	// Verificar que el tablero está vacío
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if gs.Board[i][j] != "" {
				t.Errorf("Posición inicial [%d][%d] esperada vacía, se obtuvo '%s'", i, j, gs.Board[i][j])
			}
		}
	}
}

func TestApplyMove(t *testing.T) {
	t.Run("Movimiento válido", func(t *testing.T) {
		gs := NewGameState()
		err := ApplyMove(gs, "X", 0, 0)

		if err != nil {
			t.Errorf("Error inesperado al aplicar movimiento: %v", err)
		}
		if gs.Board[0][0] != "X" {
			t.Errorf("Movimiento no aplicado correctamente, se esperaba 'X', se obtuvo '%s'", gs.Board[0][0])
		}
		if gs.CurrentTurnSymbol != "O" {
			t.Errorf("Turno no cambió correctamente, se esperaba 'O', se obtuvo '%s'", gs.CurrentTurnSymbol)
		}
	})

	t.Run("Movimiento fuera del tablero", func(t *testing.T) {
		gs := NewGameState()
		err := ApplyMove(gs, "X", 3, 3)

		if err == nil {
			t.Error("Se esperaba error por movimiento fuera del tablero")
		}
	})

	t.Run("Posición ya ocupada", func(t *testing.T) {
		gs := NewGameState()
		// Realizar primer movimiento
		ApplyMove(gs, "X", 0, 0)

		// Intentar sobrescribir la misma posición
		err := ApplyMove(gs, "O", 0, 0)

		if err == nil {
			t.Error("Se esperaba error por posición ya ocupada")
		}
	})

	t.Run("Turno incorrecto", func(t *testing.T) {
		gs := NewGameState()
		// Intentar mover con "O" cuando es turno de "X"
		err := ApplyMove(gs, "O", 0, 0)

		if err == nil {
			t.Error("Se esperaba error por turno incorrecto")
		}
	})

	t.Run("Juego terminado", func(t *testing.T) {
		gs := NewGameState()
		// Crear situación ganadora para X
		ApplyMove(gs, "X", 0, 0) // X
		ApplyMove(gs, "O", 1, 0) // O
		ApplyMove(gs, "X", 0, 1) // X
		ApplyMove(gs, "O", 1, 1) // O
		ApplyMove(gs, "X", 0, 2) // X gana

		// Intentar mover después de que el juego terminó
		err := ApplyMove(gs, "O", 2, 2)

		if err == nil {
			t.Error("Se esperaba error por juego terminado")
		}
	})
}

func TestCheckWin(t *testing.T) {
	t.Run("Victoria en fila", func(t *testing.T) {
		gs := NewGameState()
		gs.Board[0][0] = "X"
		gs.Board[0][1] = "X"
		gs.Board[0][2] = "X"

		winner, isDraw := CheckWin(gs)

		if winner != "X" {
			t.Errorf("Se esperaba ganador 'X', se obtuvo '%s'", winner)
		}
		if isDraw {
			t.Error("No debería indicar empate en una victoria")
		}
	})

	t.Run("Victoria en columna", func(t *testing.T) {
		gs := NewGameState()
		gs.Board[0][1] = "O"
		gs.Board[1][1] = "O"
		gs.Board[2][1] = "O"

		winner, isDraw := CheckWin(gs)

		if winner != "O" {
			t.Errorf("Se esperaba ganador 'O', se obtuvo '%s'", winner)
		}
		if isDraw {
			t.Error("No debería indicar empate en una victoria")
		}
	})

	t.Run("Victoria en diagonal principal", func(t *testing.T) {
		gs := NewGameState()
		gs.Board[0][0] = "X"
		gs.Board[1][1] = "X"
		gs.Board[2][2] = "X"

		winner, isDraw := CheckWin(gs)

		if winner != "X" {
			t.Errorf("Se esperaba ganador 'X', se obtuvo '%s'", winner)
		}
		if isDraw {
			t.Error("No debería indicar empate en una victoria")
		}
	})

	t.Run("Victoria en diagonal secundaria", func(t *testing.T) {
		gs := NewGameState()
		gs.Board[0][2] = "O"
		gs.Board[1][1] = "O"
		gs.Board[2][0] = "O"

		winner, isDraw := CheckWin(gs)

		if winner != "O" {
			t.Errorf("Se esperaba ganador 'O', se obtuvo '%s'", winner)
		}
		if isDraw {
			t.Error("No debería indicar empate en una victoria")
		}
	})

	t.Run("Empate", func(t *testing.T) {
		gs := NewGameState()
		// Crear tablero lleno sin ganador
		gs.Board[0][0] = "X"
		gs.Board[0][1] = "O"
		gs.Board[0][2] = "X"
		gs.Board[1][0] = "X"
		gs.Board[1][1] = "O"
		gs.Board[1][2] = "X"
		gs.Board[2][0] = "O"
		gs.Board[2][1] = "X"
		gs.Board[2][2] = "O"

		winner, isDraw := CheckWin(gs)

		if winner != "" {
			t.Errorf("No debería haber ganador en empate, se obtuvo '%s'", winner)
		}
		if !isDraw {
			t.Error("Se esperaba empate")
		}
	})

	t.Run("Juego no terminado", func(t *testing.T) {
		gs := NewGameState()
		gs.Board[0][0] = "X"
		gs.Board[1][1] = "O"

		winner, isDraw := CheckWin(gs)

		if winner != "" {
			t.Errorf("No debería haber ganador en juego no terminado, se obtuvo '%s'", winner)
		}
		if isDraw {
			t.Error("No debería indicar empate en juego no terminado")
		}
	})
}

func TestGameComplete(t *testing.T) {
	// Probar un juego completo con victoria de X
	gs := NewGameState()

	// X en (0,0)
	err := ApplyMove(gs, "X", 0, 0)
	if err != nil {
		t.Fatalf("Error en movimiento 1: %v", err)
	}

	// O en (1,1)
	err = ApplyMove(gs, "O", 1, 1)
	if err != nil {
		t.Fatalf("Error en movimiento 2: %v", err)
	}

	// X en (0,1)
	err = ApplyMove(gs, "X", 0, 1)
	if err != nil {
		t.Fatalf("Error en movimiento 3: %v", err)
	}

	// O en (2,2)
	err = ApplyMove(gs, "O", 2, 2)
	if err != nil {
		t.Fatalf("Error en movimiento 4: %v", err)
	}

	// X en (0,2) - Victoria en fila
	err = ApplyMove(gs, "X", 0, 2)
	if err != nil {
		t.Fatalf("Error en movimiento 5: %v", err)
	}

	// Verificar estado final
	if !gs.IsGameOver {
		t.Error("El juego debería haber terminado")
	}

	if gs.Winner != "X" {
		t.Errorf("El ganador debería ser 'X', se obtuvo '%s'", gs.Winner)
	}

	if gs.IsDraw {
		t.Error("El juego no debería ser empate")
	}
}
