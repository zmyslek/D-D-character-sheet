package domain

// Equipment is the minimal equipment record needed by character management.
type Equipment struct {
	// Name is the equipment's display and lookup name.
	Name string
	// Type distinguishes weapons, armor, and other equipment categories.
	Type string
	// OnHand records whether the item is currently carried or equipped.
	OnHand bool
}
