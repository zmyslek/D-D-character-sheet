package racedomain

type Race struct {
	// Race identifies a character's ancestry and any ability score adjustments.
	RaceName  string
	Abilities map[string]int
}
