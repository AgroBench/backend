package domain

import "fmt"

// ContributionLevel é a granularidade do dado contribuído (README raiz §6.2).
type ContributionLevel string

const (
	LevelBasic        ContributionLevel = "basic"
	LevelIntermediate ContributionLevel = "intermediate"
	LevelAdvanced     ContributionLevel = "advanced"
)

func ParseLevel(s string) (ContributionLevel, error) {
	switch ContributionLevel(s) {
	case LevelBasic, LevelIntermediate, LevelAdvanced:
		return ContributionLevel(s), nil
	}
	return "", fmt.Errorf("nível inválido: %q", s)
}

// Rank permite comparar níveis (advanced > intermediate > basic).
func (l ContributionLevel) Rank() int {
	switch l {
	case LevelBasic:
		return 1
	case LevelIntermediate:
		return 2
	case LevelAdvanced:
		return 3
	}
	return 0
}
