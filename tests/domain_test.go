package tests

import (
	"reflect"
	"testing"

	"mpp-zmyslek/internal/domain"
)

// Only the standard "testing" package is used. Pattern: table-driven tests.

func TestAbilityModifier(t *testing.T) {
	tests := []struct{ score, want int }{
		{0, -5}, {1, -5}, {3, -4}, {7, -2}, {8, -1}, {9, -1}, {10, 0}, {11, 0}, {12, 1}, {15, 2}, {16, 3}, {20, 5}, {30, 10},
	}
	for _, tt := range tests {
		if got := domain.AbilityModifier(tt.score); got != tt.want {
			t.Errorf("AbilityModifier(%d) = %d, want %d", tt.score, got, tt.want)
		}
	}
}

func TestProficiencyBonus(t *testing.T) {
	tests := []struct{ level, want int }{{0, 0}, {1, 2}, {4, 2}, {5, 3}, {9, 4}, {13, 5}, {17, 6}, {20, 6}}
	for _, tt := range tests {
		if got := domain.ProficiencyBonus(tt.level); got != tt.want {
			t.Errorf("ProficiencyBonus(%d) = %d, want %d", tt.level, got, tt.want)
		}
	}
}

func TestHasStandardArray(t *testing.T) {
	tests := []struct {
		name   string
		scores domain.AbilityScores
		want   bool
	}{
		{"valid", domain.AbilityScores{Str: 15, Dex: 14, Con: 13, Int: 12, Wis: 10, Cha: 8}, true},
		{"valid shuffled", domain.AbilityScores{Str: 8, Dex: 10, Con: 12, Int: 13, Wis: 14, Cha: 15}, true},
		{"duplicate value", domain.AbilityScores{Str: 15, Dex: 15, Con: 13, Int: 12, Wis: 10, Cha: 8}, false},
		{"value outside array", domain.AbilityScores{Str: 16, Dex: 14, Con: 13, Int: 12, Wis: 10, Cha: 8}, false},
		{"all zero", domain.AbilityScores{}, false},
	}
	for _, tt := range tests {
		if got := tt.scores.HasStandardArray(); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

var standardScores = domain.AbilityScores{Str: 15, Dex: 14, Con: 13, Int: 12, Wis: 10, Cha: 8}

// character builds a valid character or fails the test.
func character(t *testing.T, class string, level int, scores domain.AbilityScores) *domain.Character {
	t.Helper()
	c, err := domain.NewCharacter("Hero", "human", class, "acolyte", level, scores)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNewCharacterValidates(t *testing.T) {
	tests := []struct {
		name, charName, race, class string
		level                       int
		scores                      domain.AbilityScores
		wantErr                     string
	}{
		{"missing name", " ", "elf", "wizard", 1, standardScores, "name is required"},
		{"missing race", "A", "", "wizard", 1, standardScores, "race and class are required"},
		{"level 0", "A", "elf", "wizard", 0, standardScores, "level must be between 1 and 20"},
		{"level 21", "A", "elf", "wizard", 21, standardScores, "level must be between 1 and 20"},
		{"score 31", "A", "elf", "wizard", 1, domain.AbilityScores{Str: 31}, "ability scores must be between 0 and 30"},
		{"negative score", "A", "elf", "wizard", 1, domain.AbilityScores{Dex: -1}, "ability scores must be between 0 and 30"},
	}
	for _, tt := range tests {
		_, err := domain.NewCharacter(tt.charName, tt.race, tt.class, "", tt.level, tt.scores)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("%s: err = %v, want %q", tt.name, err, tt.wantErr)
		}
	}
}

// Encapsulation: getters hand out copies, so callers cannot change a
// character's state by editing a returned slice.
func TestGettersReturnCopies(t *testing.T) {
	c := character(t, "wizard", 1, standardScores)
	c.AddSkillProficiencies([]string{"Arcana"}, 1)
	skills := c.SkillProficiencies()
	skills[0] = "hacked"
	data := c.Data()
	data.SkillProficiencies[0] = "hacked"
	data.Level = 20
	if c.SkillProficiencies()[0] != "arcana" || c.Level() != 1 {
		t.Error("character state changed through a returned value")
	}
	restored := domain.RestoreCharacter(c.Data())
	if !reflect.DeepEqual(restored.Data(), c.Data()) {
		t.Error("RestoreCharacter(Data()) should give the same character")
	}
}

func TestApplyAbilityBonus(t *testing.T) {
	c := character(t, "fighter", 1, standardScores)
	for _, ability := range []string{"Strength", "dexterity", " CONSTITUTION ", "intelligence", "wisdom", "charisma"} {
		if err := c.ApplyAbilityBonus(ability, 1); err != nil {
			t.Fatal(err)
		}
	}
	if want := (domain.AbilityScores{Str: 16, Dex: 15, Con: 14, Int: 13, Wis: 11, Cha: 9}); c.Scores() != want {
		t.Errorf("scores = %+v", c.Scores())
	}
	if err := c.ApplyAbilityBonus("luck", 1); err == nil {
		t.Error("unknown ability should fail")
	}
	if err := c.ApplyAbilityBonus("strength", 15); err == nil {
		t.Error("a score above 30 should fail")
	}
}

func TestSkillProficiencies(t *testing.T) {
	c := character(t, "fighter", 1, standardScores)
	c.AddSkillProficiencies([]string{" Stealth ", ""}, 5)
	c.AddSkillProficiencies([]string{"acrobatics", "athletics", "history"}, 2)
	c.AddBackgroundSkills()
	want := []string{"stealth", "acrobatics", "athletics", "insight", "religion"}
	if got := c.SkillProficiencies(); !reflect.DeepEqual(got, want) {
		t.Errorf("skills = %v, want %v", got, want)
	}
}

func TestChangeLevel(t *testing.T) {
	c := character(t, "wizard", 3, standardScores)
	if err := c.ChangeLevel(5); err != nil || c.Level() != 5 || c.ProficiencyBonus() != 3 {
		t.Errorf("ChangeLevel(5): level %d, err %v", c.Level(), err)
	}
	for _, level := range []int{4, 0, 21} {
		if err := c.ChangeLevel(level); err == nil {
			t.Errorf("ChangeLevel(%d) should fail", level)
		}
	}
	if c.Level() != 5 {
		t.Error("a failed change must keep the level")
	}
}

// The expected values come from the SRD 5.1 class tables.
func TestSpellSlotsMatchTheSRD(t *testing.T) {
	tests := []struct {
		class string
		level int
		want  map[int]int
	}{
		{"wizard", 1, map[int]int{0: 3, 1: 2}},
		{"wizard", 3, map[int]int{0: 3, 1: 4, 2: 2}},
		{"wizard", 5, map[int]int{0: 4, 1: 4, 2: 3, 3: 2}},
		{"wizard", 18, map[int]int{0: 5, 1: 4, 2: 3, 3: 3, 4: 3, 5: 3, 6: 1, 7: 1, 8: 1, 9: 1}},
		{"wizard", 20, map[int]int{0: 5, 1: 4, 2: 3, 3: 3, 4: 3, 5: 3, 6: 2, 7: 2, 8: 1, 9: 1}},
		{"sorcerer", 1, map[int]int{0: 4, 1: 2}},
		{"druid", 1, map[int]int{0: 2, 1: 2}},
		{"bard", 10, map[int]int{0: 4, 1: 4, 2: 3, 3: 3, 4: 3, 5: 2}},
		{"paladin", 1, nil},
		{"paladin", 2, map[int]int{1: 2}},
		{"paladin", 5, map[int]int{1: 4, 2: 2}},
		{"ranger", 20, map[int]int{1: 4, 2: 3, 3: 3, 4: 3, 5: 2}},
		{"warlock", 1, map[int]int{0: 2, 1: 1}},
		{"warlock", 3, map[int]int{0: 2, 2: 2}},
		{"warlock", 11, map[int]int{0: 4, 5: 3}},
		{"fighter", 5, nil},
	}
	for _, tt := range tests {
		if got := character(t, tt.class, tt.level, standardScores).SpellSlots(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s %d slots = %v, want %v", tt.class, tt.level, got, tt.want)
		}
	}
}

func TestArmorClass(t *testing.T) {
	scores := domain.AbilityScores{Str: 10, Dex: 16, Con: 14, Int: 10, Wis: 14, Cha: 10}
	tests := []struct {
		name, class, armor, shield string
		want                       int
	}{
		{"unarmored", "fighter", "", "", 13},
		{"padded armor", "fighter", "Padded Armor", "", 14},
		{"medium armor caps dex", "fighter", "breastplate", "", 16},
		{"half plate alias", "fighter", "half plate", "", 17},
		{"heavy armor ignores dex", "fighter", "plate armor", "", 18},
		{"shield", "fighter", "chain mail", "Shield", 18},
		{"barbarian unarmored", "barbarian", "", "", 15},
		{"barbarian keeps it with a shield", "barbarian", "", "Shield", 17},
		{"monk unarmored", "monk", "", "", 15},
		{"monk loses it with a shield", "monk", "", "Shield", 15},
	}
	for _, tt := range tests {
		c := domain.RestoreCharacter(domain.CharacterData{
			Name: "A", Race: "human", Class: tt.class, Level: 1, Armor: tt.armor, Shield: tt.shield,
			Str: scores.Str, Dex: scores.Dex, Con: scores.Con, Int: scores.Int, Wis: scores.Wis, Cha: scores.Cha,
		})
		if got := c.ArmorClass(); got != tt.want {
			t.Errorf("%s: AC = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestCombatStats(t *testing.T) {
	c := character(t, "cleric", 5, domain.AbilityScores{Str: 10, Dex: 12, Con: 14, Int: 8, Wis: 16, Cha: 13})
	if c.Initiative() != 1 || c.SpellSaveDC() != 14 || c.SpellAttackBonus() != 6 {
		t.Errorf("initiative %d, DC %d, attack %d", c.Initiative(), c.SpellSaveDC(), c.SpellAttackBonus())
	}
	if ability, score := c.SpellcastingAbility(); ability != "wisdom" || score != 16 {
		t.Errorf("spellcasting ability = %s %d", ability, score)
	}
	if c.PassivePerception() != 13 {
		t.Errorf("passive perception = %d, want 10 + 3", c.PassivePerception())
	}
	c.AddSkillProficiencies([]string{"perception"}, 1)
	if c.PassivePerception() != 16 {
		t.Errorf("passive perception with proficiency = %d, want 10 + 3 + 3", c.PassivePerception())
	}
	if ability, _ := character(t, "rogue", 1, standardScores).SpellcastingAbility(); ability != "" {
		t.Error("rogues have no spellcasting ability")
	}
}

func TestEquipment(t *testing.T) {
	sword := domain.NewEquipment("Longsword", "Weapon")
	dagger := domain.NewEquipment("Dagger", "Weapon")
	mail := domain.NewEquipment("Chain Mail", "Armor")
	shield := domain.NewEquipment("Shield", "Armor")

	c := character(t, "fighter", 1, standardScores)
	steps := []struct {
		name string
		do   func() error
		want string
	}{
		{"armor as weapon", func() error { return c.EquipWeapon(mail, "main hand") }, domain.ErrNotAWeapon.Error()},
		{"bad slot", func() error { return c.EquipWeapon(sword, "head") }, `slot must be "main hand" or "off hand"`},
		{"main hand", func() error { return c.EquipWeapon(sword, " Main Hand ") }, ""},
		{"main hand taken", func() error { return c.EquipWeapon(dagger, "main hand") }, "main hand already occupied"},
		{"off hand", func() error { return c.EquipWeapon(dagger, "off hand") }, ""},
		{"off hand taken", func() error { return c.EquipWeapon(dagger, "off hand") }, "off hand already occupied"},
		{"shield as armor", func() error { return c.EquipArmor(shield) }, domain.ErrNotArmor.Error()},
		{"armor as shield", func() error { return c.EquipShield(mail) }, domain.ErrNotAShield.Error()},
		{"armor", func() error { return c.EquipArmor(mail) }, ""},
		{"shield", func() error { return c.EquipShield(shield) }, ""},
	}
	for _, step := range steps {
		err := step.do()
		if (step.want == "" && err != nil) || (step.want != "" && (err == nil || err.Error() != step.want)) {
			t.Errorf("%s: err = %v, want %q", step.name, err, step.want)
		}
	}
	if c.MainHand() != "Longsword" || c.OffHand() != "Dagger" || c.Armor() != "Chain Mail" || c.Shield() != "Shield" {
		t.Errorf("loadout = %+v", c.Data())
	}
	c.RemoveEquipment()
	if c.MainHand()+c.OffHand()+c.Armor()+c.Shield() != "" {
		t.Error("RemoveEquipment should empty every slot")
	}
}

func TestSpells(t *testing.T) {
	fireBolt := domain.NewSpell("Fire Bolt", 0, []string{"Sorcerer", " Wizard"})
	missile := domain.NewSpell("Magic Missile", 1, []string{"Sorcerer", "Wizard"})
	fireball := domain.NewSpell("Fireball", 3, []string{"Sorcerer", "Wizard"})
	cure := domain.NewSpell("Cure Wounds", 1, []string{"Cleric"})
	charm := domain.NewSpell("Charm Person", 1, []string{"Warlock"})

	tests := []struct {
		name    string
		class   string
		level   int
		spell   *domain.Spell
		prepare bool
		want    string
	}{
		{"non caster", "fighter", 5, fireBolt, false, "this class can't cast spells"},
		{"paladin at level 1", "paladin", 1, cure, true, "this class can't cast spells"},
		{"wizard cannot learn", "wizard", 1, fireBolt, false, "this class prepares spells and can't learn them"},
		{"sorcerer cannot prepare", "sorcerer", 1, fireBolt, true, "this class learns spells and can't prepare them"},
		{"wrong class list", "wizard", 1, cure, true, "spell cure wounds is not available to this class"},
		{"level too high", "wizard", 1, fireball, true, "the spell has higher level than the available spell slots"},
		{"cantrip", "wizard", 1, fireBolt, true, ""},
		{"first level", "sorcerer", 1, missile, false, ""},
		// A level 3 warlock only has 2nd-level Pact Magic slots, which can
		// still cast 1st-level spells.
		{"pact magic", "warlock", 3, charm, false, ""},
	}
	for _, tt := range tests {
		c := character(t, tt.class, tt.level, standardScores)
		var err error
		if tt.prepare {
			err = c.PrepareSpell(tt.spell)
		} else {
			err = c.LearnSpell(tt.spell)
		}
		if (tt.want == "" && err != nil) || (tt.want != "" && (err == nil || err.Error() != tt.want)) {
			t.Errorf("%s: err = %v, want %q", tt.name, err, tt.want)
		}
	}

	c := character(t, "wizard", 1, standardScores)
	for range 2 {
		if err := c.PrepareSpell(fireBolt); err != nil {
			t.Fatal(err)
		}
	}
	if got := c.PreparedSpells(); !reflect.DeepEqual(got, []string{"fire bolt"}) {
		t.Errorf("preparing twice should store the spell once, got %v", got)
	}
	c.RemoveSpells()
	if len(c.PreparedSpells())+len(c.KnownSpells()) != 0 {
		t.Error("RemoveSpells should empty both lists")
	}
}
