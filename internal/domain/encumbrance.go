package domain

// Condition is a condition a character can gain from carrying too much.
type Condition string

// The encumbrance conditions, from lightest to heaviest load.
const (
	NotEncumbered     Condition = "none"
	Encumbered        Condition = "encumbered"
	HeavilyEncumbered Condition = "heavily encumbered"
)

// BaseCarryingCapacity is the capacity, in pounds, of a character with a +0
// Strength modifier.
const BaseCarryingCapacity = 20

// CarryingCapacity is the base capacity plus two pounds per point of the
// Strength modifier, so Strength 8 (-1) gives 18 and Strength 16 (+3) gives 26.
func CarryingCapacity(strength int) int {
	return BaseCarryingCapacity + 2*AbilityModifier(strength)
}

// Encumbrance is how much a character carries compared with what they can
// carry. It is a value object: it is calculated, never stored.
type Encumbrance struct {
	CarriedWeight float64   // pounds
	Capacity      int       // pounds
	Condition     Condition // follows from the two numbers above
}

// EncumbranceFor applies the encumbrance rule to a carried weight:
//   - up to the capacity: no condition;
//   - more than the capacity, up to twice the capacity: encumbered;
//   - more than twice the capacity: heavily encumbered.
//
// The rule only says "less than" and "more than"; carrying exactly the
// capacity (or exactly twice it) is not "more than", so it gives the lighter
// condition.
func EncumbranceFor(strength int, carriedWeight float64) Encumbrance {
	capacity := CarryingCapacity(strength)
	condition := NotEncumbered
	switch limit := float64(capacity); {
	case carriedWeight > 2*limit:
		condition = HeavilyEncumbered
	case carriedWeight > limit:
		condition = Encumbered
	}
	return Encumbrance{CarriedWeight: carriedWeight, Capacity: capacity, Condition: condition}
}

// EquippedItems returns the names of the equipped items in slot order: main
// hand, off hand, armor, shield. Empty slots are left out; the same weapon in
// both hands is listed twice, because the character carries two of them.
func (character *Character) EquippedItems() []string {
	var items []string
	for _, item := range []string{character.mainHand, character.offHand, character.armor, character.shield} {
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

// Encumbrance totals the weight of the given items (the character's equipped
// items, looked up in the equipment list) against the character's Strength.
func (character *Character) Encumbrance(carried []*Equipment) Encumbrance {
	total := 0.0
	for _, item := range carried {
		total += item.Weight()
	}
	return EncumbranceFor(character.scores.Str, total)
}
