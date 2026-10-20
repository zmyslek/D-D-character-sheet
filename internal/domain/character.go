package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

const (
	// MainHandSlot identifies the character's primary weapon slot.
	MainHandSlot = "main hand"
	// OffHandSlot identifies the character's secondary weapon slot.
	OffHandSlot = "off hand"
	// MaxLevel is the highest character level in the SRD.
	MaxLevel = 20
	// MaxAbilityScore is the highest ability score in the SRD.
	MaxAbilityScore = 30
)

// Errors that callers check with errors.Is.
var (
	ErrNotFound   = errors.New("not found")
	ErrNotAWeapon = errors.New("equipment is not a weapon")
	ErrNotArmor   = errors.New("equipment is not armor")
	ErrNotAShield = errors.New("equipment is not a shield")
)

// AbilityScores holds the six ability scores. It is a value object: it is
// copied, never shared, so changing a copy never changes a character.
type AbilityScores struct {
	Str, Dex, Con, Int, Wis, Cha int
}

// HasStandardArray reports whether the scores use 15, 14, 13, 12, 10 and 8 once each.
func (scores AbilityScores) HasStandardArray() bool {
	values := []int{scores.Str, scores.Dex, scores.Con, scores.Int, scores.Wis, scores.Cha}
	slices.Sort(values)
	return slices.Equal(values, []int{8, 10, 12, 13, 14, 15})
}

func (scores AbilityScores) valid() bool {
	for _, value := range []int{scores.Str, scores.Dex, scores.Con, scores.Int, scores.Wis, scores.Cha} {
		if value < 0 || value > MaxAbilityScore {
			return false
		}
	}
	return true
}

// Character is the aggregate root of the character sheet. Its fields are
// unexported, so other packages can only change a character through the
// methods below, which keep the sheet valid:
//   - the name is set and the level stays between 1 and 20 and never decreases;
//   - weapons only go in a free hand, armor in the armor slot, shields in the shield slot;
//   - spells must be on the class list and castable at the current level, and
//     are learned or prepared according to the class.
type Character struct {
	name               string
	race               string
	class              string
	level              int
	background         string
	scores             AbilityScores
	skillProficiencies []string
	mainHand           string
	offHand            string
	armor              string
	shield             string
	knownSpells        []string
	preparedSpells     []string
}

// NewCharacter creates a level 1-20 character with valid ability scores.
func NewCharacter(name, race, class, background string, level int, scores AbilityScores) (*Character, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("name is required")
	}
	if strings.TrimSpace(race) == "" || strings.TrimSpace(class) == "" {
		return nil, errors.New("race and class are required")
	}
	if level < 1 || level > MaxLevel {
		return nil, errors.New("level must be between 1 and 20")
	}
	if !scores.valid() {
		return nil, errors.New("ability scores must be between 0 and 30")
	}
	return &Character{name: name, race: race, class: class, background: background, level: level, scores: scores}, nil
}

// CharacterData is a plain copy of a character's state. Repositories use it to
// store and load characters; changing it does not change any Character.
type CharacterData struct {
	Name                         string
	Race                         string
	Class                        string
	Level                        int
	Background                   string
	SkillProficiencies           []string
	MainHand                     string
	OffHand                      string
	Armor                        string
	Shield                       string
	KnownSpells                  []string
	PreparedSpells               []string
	Str, Dex, Con, Int, Wis, Cha int
}

// RestoreCharacter rebuilds a stored character. It does not re-apply creation
// rules such as racial bonuses, which were applied when it was created.
func RestoreCharacter(data CharacterData) *Character {
	return &Character{
		name: data.Name, race: data.Race, class: data.Class, level: data.Level, background: data.Background,
		scores:             AbilityScores{data.Str, data.Dex, data.Con, data.Int, data.Wis, data.Cha},
		skillProficiencies: slices.Clone(data.SkillProficiencies),
		mainHand:           data.MainHand, offHand: data.OffHand, armor: data.Armor, shield: data.Shield,
		knownSpells:    slices.Clone(data.KnownSpells),
		preparedSpells: slices.Clone(data.PreparedSpells),
	}
}

// Data returns a copy of the character's state. Slices are cloned so the
// caller cannot reach the character's own slices.
func (character *Character) Data() CharacterData {
	scores := character.scores
	return CharacterData{
		Name: character.name, Race: character.race, Class: character.class, Level: character.level,
		Background:         character.background,
		SkillProficiencies: slices.Clone(character.skillProficiencies),
		MainHand:           character.mainHand, OffHand: character.offHand, Armor: character.armor, Shield: character.shield,
		KnownSpells:    slices.Clone(character.knownSpells),
		PreparedSpells: slices.Clone(character.preparedSpells),
		Str:            scores.Str, Dex: scores.Dex, Con: scores.Con, Int: scores.Int, Wis: scores.Wis, Cha: scores.Cha,
	}
}

// Name returns the character's unique name.
func (character *Character) Name() string { return character.name }

// Race returns the race as entered by the player.
func (character *Character) Race() string { return character.race }

// Class returns the class as entered by the player.
func (character *Character) Class() string { return character.class }

// Level returns the character level.
func (character *Character) Level() int { return character.level }

// Background returns the background.
func (character *Character) Background() string { return character.background }

// Scores returns a copy of the ability scores.
func (character *Character) Scores() AbilityScores { return character.scores }

// SkillProficiencies returns a copy of the skill proficiencies.
func (character *Character) SkillProficiencies() []string {
	return slices.Clone(character.skillProficiencies)
}

// MainHand returns the weapon in the main hand, or "".
func (character *Character) MainHand() string { return character.mainHand }

// OffHand returns the weapon in the off hand, or "".
func (character *Character) OffHand() string { return character.offHand }

// Armor returns the worn armor, or "".
func (character *Character) Armor() string { return character.armor }

// Shield returns the carried shield, or "".
func (character *Character) Shield() string { return character.shield }

// KnownSpells returns a copy of the learned spells.
func (character *Character) KnownSpells() []string { return slices.Clone(character.knownSpells) }

// PreparedSpells returns a copy of the prepared spells.
func (character *Character) PreparedSpells() []string { return slices.Clone(character.preparedSpells) }

// ApplyAbilityBonus applies a racial ability-score increase to the character.
func (character *Character) ApplyAbilityBonus(ability string, points int) error {
	var score *int
	switch normalize(ability) {
	case "strength":
		score = &character.scores.Str
	case "dexterity":
		score = &character.scores.Dex
	case "constitution":
		score = &character.scores.Con
	case "intelligence":
		score = &character.scores.Int
	case "wisdom":
		score = &character.scores.Wis
	case "charisma":
		score = &character.scores.Cha
	default:
		return fmt.Errorf("unknown ability %q", ability)
	}
	if *score+points > MaxAbilityScore {
		return errors.New("ability scores must be between 0 and 30")
	}
	*score += points
	return nil
}

// AddSkillProficiencies adds the first `limit` non-empty skills (assignment
// rule: "pick the first N"). Duplicates are kept, as the assignment says to
// ignore that problem.
func (character *Character) AddSkillProficiencies(skills []string, limit int) {
	added := 0
	for _, skill := range skills {
		if added >= limit {
			break
		}
		if normalized := normalize(skill); normalized != "" {
			character.skillProficiencies = append(character.skillProficiencies, normalized)
			added++
		}
	}
}

// AddBackgroundSkills adds the skills granted by the character's background.
func (character *Character) AddBackgroundSkills() {
	character.AddSkillProficiencies(backgroundSkills(character.background), 2)
}

// ChangeLevel sets a new level. Levels only go up: the SRD has no level loss,
// and this guarantees that spells already chosen stay castable.
func (character *Character) ChangeLevel(level int) error {
	if level < 1 || level > MaxLevel {
		return errors.New("level must be between 1 and 20")
	}
	if level < character.level {
		return errors.New("level cannot decrease")
	}
	character.level = level
	return nil
}

// EquipWeapon stores a weapon in an empty main-hand or off-hand slot.
func (character *Character) EquipWeapon(weapon *Equipment, slot string) error {
	if !weapon.IsWeapon() {
		return ErrNotAWeapon
	}
	switch normalize(slot) {
	case MainHandSlot:
		if character.mainHand != "" {
			return errors.New("main hand already occupied")
		}
		character.mainHand = weapon.Name()
	case OffHandSlot:
		if character.offHand != "" {
			return errors.New("off hand already occupied")
		}
		character.offHand = weapon.Name()
	default:
		return fmt.Errorf("slot must be %q or %q", MainHandSlot, OffHandSlot)
	}
	return nil
}

// EquipArmor puts on armor, replacing any armor already worn.
func (character *Character) EquipArmor(armor *Equipment) error {
	if !armor.IsArmor() {
		return ErrNotArmor
	}
	character.armor = armor.Name()
	return nil
}

// EquipShield takes up a shield, replacing any shield already carried.
func (character *Character) EquipShield(shield *Equipment) error {
	if !shield.IsShield() {
		return ErrNotAShield
	}
	character.shield = shield.Name()
	return nil
}

// Unequip empties one slot: "main hand", "off hand", "armor" or "shield".
func (character *Character) Unequip(slot string) error {
	switch normalize(slot) {
	case MainHandSlot:
		character.mainHand = ""
	case OffHandSlot:
		character.offHand = ""
	case "armor":
		character.armor = ""
	case "shield":
		character.shield = ""
	default:
		return fmt.Errorf("slot must be %q, %q, \"armor\" or \"shield\"", MainHandSlot, OffHandSlot)
	}
	return nil
}

// RemoveEquipment empties every equipment slot.
func (character *Character) RemoveEquipment() {
	character.mainHand, character.offHand, character.armor, character.shield = "", "", "", ""
}

// RemoveSpells empties the known and prepared spell lists.
func (character *Character) RemoveSpells() {
	character.knownSpells, character.preparedSpells = nil, nil
}

// CanAddSpell reports whether this class may learn (prepare=false) or prepare
// (prepare=true) spells at its current level.
func (character *Character) CanAddSpell(prepare bool) error {
	if !character.CanCastSpells() {
		return errors.New("this class can't cast spells")
	}
	if prepare && !character.IsPreparedCaster() {
		return errors.New("this class learns spells and can't prepare them")
	}
	if !prepare && character.IsPreparedCaster() {
		return errors.New("this class prepares spells and can't learn them")
	}
	return nil
}

// LearnSpell adds a spell to the known spells.
func (character *Character) LearnSpell(spell *Spell) error {
	return character.addSpell(spell, false)
}

// PrepareSpell adds a spell to the prepared spells.
func (character *Character) PrepareSpell(spell *Spell) error {
	return character.addSpell(spell, true)
}

func (character *Character) addSpell(spell *Spell, prepare bool) error {
	if err := character.CanAddSpell(prepare); err != nil {
		return err
	}
	name := strings.ToLower(spell.Name())
	if !spell.AvailableTo(character.class) {
		return fmt.Errorf("spell %s is not available to this class", name)
	}
	if spell.Level() > highestSpellLevel(character.class, character.level) {
		return errors.New("the spell has higher level than the available spell slots")
	}
	// Paladins and rangers have no cantrips in the SRD.
	if spell.Level() == 0 && cantripsKnown(character.class, character.level) == 0 {
		return errors.New("this class can't cast cantrips")
	}
	list := &character.knownSpells
	if prepare {
		list = &character.preparedSpells
	}
	// Adding a spell twice changes nothing, so retrying is harmless.
	if !slices.Contains(*list, name) {
		*list = append(*list, name)
	}
	return nil
}

// CanCastSpells reports whether the character's class and level grant spellcasting.
func (character *Character) CanCastSpells() bool {
	return canCastSpells(character.class, character.level)
}

// IsPreparedCaster reports whether the class prepares spells rather than learning them.
func (character *Character) IsPreparedCaster() bool {
	return isPreparedCaster(character.class)
}

// SpellSlots returns spell slots by spell level, using level zero for cantrips.
func (character *Character) SpellSlots() map[int]int {
	return spellSlots(character.class, character.level)
}

// ProficiencyBonus is derived from the level.
func (character *Character) ProficiencyBonus() int {
	return ProficiencyBonus(character.level)
}

// ArmorClass calculates armor class from armor, Dexterity, class features and a shield.
func (character *Character) ArmorClass() int {
	scores := character.scores
	return armorClass(character.class, character.armor, character.shield != "", scores.Dex, scores.Con, scores.Wis)
}

// abilityScore returns the score of an ability by its full name, or 10 (a
// +0 modifier) for an unknown name.
func (character *Character) abilityScore(ability string) int {
	scores := character.scores
	switch normalize(ability) {
	case "strength":
		return scores.Str
	case "dexterity":
		return scores.Dex
	case "constitution":
		return scores.Con
	case "intelligence":
		return scores.Int
	case "wisdom":
		return scores.Wis
	case "charisma":
		return scores.Cha
	default:
		return 10
	}
}

// IsProficient reports whether the character is proficient in a skill.
func (character *Character) IsProficient(skill string) bool {
	return slices.Contains(character.skillProficiencies, normalize(skill))
}

// SkillModifier is the modifier of the skill's ability, plus the proficiency
// bonus when the character is proficient in the skill.
func (character *Character) SkillModifier(skill string) int {
	modifier := AbilityModifier(character.abilityScore(skillAbility(skill)))
	if character.IsProficient(skill) {
		modifier += character.ProficiencyBonus()
	}
	return modifier
}

// SkillModifiers returns the modifier of every SRD skill by skill name.
func (character *Character) SkillModifiers() map[string]int {
	modifiers := make(map[string]int, len(Skills))
	for _, skill := range Skills {
		modifiers[skill.Name] = character.SkillModifier(skill.Name)
	}
	return modifiers
}

// Initiative is the Dexterity modifier.
func (character *Character) Initiative() int {
	return AbilityModifier(character.scores.Dex)
}

// PassivePerception is 10 + Wisdom modifier, plus the proficiency bonus when
// the character is proficient in Perception (SRD "passive Wisdom (Perception)").
func (character *Character) PassivePerception() int {
	return 10 + character.SkillModifier("perception")
}

// SpellcastingAbility returns the governing ability name and score for the class.
func (character *Character) SpellcastingAbility() (string, int) {
	scores := character.scores
	switch ability := spellcastingAbility(character.class); ability {
	case "charisma":
		return ability, scores.Cha
	case "wisdom":
		return ability, scores.Wis
	case "intelligence":
		return ability, scores.Int
	default:
		return "", 0
	}
}

// SpellSaveDC is 8 + proficiency bonus + spellcasting ability modifier, or 0
// when the character can't cast spells.
func (character *Character) SpellSaveDC() int {
	if !character.CanCastSpells() {
		return 0
	}
	return 8 + character.SpellAttackBonus()
}

// SpellAttackBonus is proficiency bonus + spellcasting ability modifier, or 0
// when the character can't cast spells.
func (character *Character) SpellAttackBonus() int {
	if !character.CanCastSpells() {
		return 0
	}
	_, score := character.SpellcastingAbility()
	return character.ProficiencyBonus() + AbilityModifier(score)
}
