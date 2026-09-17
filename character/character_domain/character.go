package characterdomain

import (
	classdomain "mpp-zmyslek/class/class_domain"
	racedomain "mpp-zmyslek/race/race_domain"
)

type Character struct {
	Name  string
	Race  racedomain.Race
	Class classdomain.Class
	Level int
	// Height      float64
	// Weight      float64
	// Eyes        string
	// Skin        string
	// Hair        string
	// Personality string
	// Alignment   string
	// Perception  int
	// Money       [5]int
	// Speed       int
	// HitPoints   int
	Str int //strength
	Dex int //dexterity
	Con int //constitution
	Int int //intelligence
	Wis int //wisdom
	Cha int //charisma
}
