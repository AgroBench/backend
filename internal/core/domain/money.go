package domain

import (
	"fmt"
	"math"
)

// MicroUSDC é o valor em milionésimos de USDC (6 casas, igual ao token SPL na Solana).
// Usamos inteiro para nunca ter erro de arredondamento em somas e splits.
type MicroUSDC int64

const MicroPerUSDC = 1_000_000

func USDC(v float64) MicroUSDC {
	return MicroUSDC(math.Round(v * MicroPerUSDC))
}

func (m MicroUSDC) Float() float64 { return float64(m) / MicroPerUSDC }

func (m MicroUSDC) String() string { return fmt.Sprintf("%.6f USDC", m.Float()) }

// MulFloat multiplica por um fator (ex.: multiplicador de nível 3.5) arredondando pro micro mais próximo.
func (m MicroUSDC) MulFloat(f float64) MicroUSDC {
	return MicroUSDC(math.Round(float64(m) * f))
}

// Pct aplica um percentual (ex.: 15 → 15%).
func (m MicroUSDC) Pct(pct float64) MicroUSDC {
	return m.MulFloat(pct / 100)
}
