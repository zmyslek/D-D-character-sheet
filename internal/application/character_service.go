package application

import (
	"errors"
	"reflect"
	"sync"

	"mpp-zmyslek/internal/domain"
)

// Errors returned by the service that callers check with errors.Is.
var (
	ErrAlreadyExists        = errors.New("a character with this name already exists")
	ErrIdempotencyKeyReused = errors.New("idempotency key was already used for a different request")
	ErrNotStandardArray     = errors.New("ability scores must use the Standard Array once each: 15, 14, 13, 12, 10, 8")
	ErrFixedAfterCreation   = errors.New("race, class and ability scores cannot be changed after creation")
)

// CharacterService contains the commands (the "C" in CQRS): the use cases that
// change characters, shared by the CLI and the web server. Each method loads a
// character, calls domain methods (which enforce the rules) and saves it.
// Reading is done by CharacterQueries.
//
// The mutex makes every load-change-save cycle run alone, so two requests that
// change the same character at the same time cannot overwrite each other.
type CharacterService struct {
	characters  domain.CharacterRepository
	classes     domain.ClassRepository
	raceBonuses domain.RaceAbilityRepository
	equipment   domain.EquipmentRepository
	spells      domain.SpellRepository

	mu sync.Mutex
	// created remembers create requests by idempotency key (see Create).
	created map[string]CreateRequest
}

// NewCharacterService creates the service with its repositories.
func NewCharacterService(
	characters domain.CharacterRepository,
	classes domain.ClassRepository,
	raceBonuses domain.RaceAbilityRepository,
	equipment domain.EquipmentRepository,
	spells domain.SpellRepository,
) *CharacterService {
	return &CharacterService{
		characters: characters, classes: classes, raceBonuses: raceBonuses,
		equipment: equipment, spells: spells,
		created: map[string]CreateRequest{},
	}
}

// CreateRequest describes a new character.
type CreateRequest struct {
	Name, Race, Class, Background string
	Level                         int
	Scores                        domain.AbilityScores
	Skills                        []string
	// RequireStandardArray is set by the web API. The CLI keeps its original
	// behaviour and accepts any scores.
	RequireStandardArray bool
	// RejectDuplicate is set by the web API. The CLI keeps its original
	// behaviour, where creating an existing name replaces that character.
	RejectDuplicate bool
}

// Create builds a character: racial bonuses, class skills and background
// skills are applied once, here.
//
// A non-empty idempotencyKey makes the request safe to retry: repeating the
// same request with the same key returns the character created the first
// time (replayed=true) instead of creating it again. Keys are kept in memory,
// so they only protect retries until the process restarts.
func (s *CharacterService) Create(request CreateRequest, idempotencyKey string) (character *domain.Character, replayed bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if idempotencyKey != "" {
		if previous, ok := s.created[idempotencyKey]; ok {
			if !reflect.DeepEqual(previous, request) {
				return nil, false, ErrIdempotencyKeyReused
			}
			character, err := s.characters.GetByName(request.Name)
			return character, true, err
		}
	}
	if request.RequireStandardArray && !request.Scores.HasStandardArray() {
		return nil, false, ErrNotStandardArray
	}
	if request.RejectDuplicate {
		if _, err := s.characters.GetByName(request.Name); err == nil {
			return nil, false, ErrAlreadyExists
		}
	}

	character, err = s.buildCharacter(request)
	if err != nil {
		return nil, false, err
	}
	if err := s.characters.Create(character); err != nil {
		return nil, false, err
	}
	if idempotencyKey != "" {
		s.created[idempotencyKey] = request
	}
	return character, false, nil
}

// Delete removes a character.
func (s *CharacterService) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.characters.Delete(name)
}

// buildCharacter creates the character and applies the creation rules once:
// racial ability bonuses, the chosen skills, the first N class skills and the
// background skills.
func (s *CharacterService) buildCharacter(request CreateRequest) (*domain.Character, error) {
	character, err := domain.NewCharacter(request.Name, request.Race, request.Class, request.Background, request.Level, request.Scores)
	if err != nil {
		return nil, err
	}
	bonuses, err := s.raceBonuses.GetByRace(request.Race)
	if err != nil {
		return nil, err
	}
	for _, bonus := range bonuses {
		if err := character.ApplyAbilityBonus(bonus.Ability, bonus.Points); err != nil {
			return nil, err
		}
	}
	class, err := s.classes.GetByName(request.Class)
	if err != nil {
		return nil, err
	}
	character.AddSkillProficiencies(request.Skills, len(request.Skills))
	character.AddSkillProficiencies(class.SkillOptions(), class.SkillChoiceCount())
	character.AddBackgroundSkills()
	return character, nil
}

// ChangeLevel sets a new level; the proficiency bonus follows from it.
func (s *CharacterService) ChangeLevel(name string, level int) (*domain.Character, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	character, err := s.characters.GetByName(name)
	if err != nil {
		return nil, err
	}
	if err := character.ChangeLevel(level); err != nil {
		return nil, err
	}
	return character, s.characters.Update(character)
}

// EquipWeapon puts a weapon in the "main hand" or "off hand" slot.
func (s *CharacterService) EquipWeapon(name, weaponName, slot string) (*domain.Equipment, error) {
	return s.equip(name, weaponName, func(character *domain.Character, item *domain.Equipment) error {
		return character.EquipWeapon(item, slot)
	})
}

// EquipArmor puts on armor.
func (s *CharacterService) EquipArmor(name, armorName string) (*domain.Equipment, error) {
	return s.equip(name, armorName, (*domain.Character).EquipArmor)
}

// EquipShield takes up a shield.
func (s *CharacterService) EquipShield(name, shieldName string) (*domain.Equipment, error) {
	return s.equip(name, shieldName, (*domain.Character).EquipShield)
}

func (s *CharacterService) equip(name, itemName string, put func(*domain.Character, *domain.Equipment) error) (*domain.Equipment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	character, err := s.characters.GetByName(name)
	if err != nil {
		return nil, err
	}
	item, err := s.putOn(character, itemName, put)
	if err != nil {
		return nil, err
	}
	return item, s.characters.Update(character)
}

// AddSpell learns (prepare=false) or prepares (prepare=true) a spell.
func (s *CharacterService) AddSpell(name, spellName string, prepare bool) (*domain.Spell, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	character, err := s.characters.GetByName(name)
	if err != nil {
		return nil, err
	}
	// Check the class first, so "this class can't cast spells" is reported
	// before a spell lookup error, as the original program did.
	if err := character.CanAddSpell(prepare); err != nil {
		return nil, err
	}
	spell, err := s.addSpell(character, spellName, prepare)
	if err != nil {
		return nil, err
	}
	return spell, s.characters.Update(character)
}

// addSpell looks a spell up and lets the character learn or prepare it.
func (s *CharacterService) addSpell(character *domain.Character, spellName string, prepare bool) (*domain.Spell, error) {
	spell, err := s.spells.GetByName(spellName)
	if err != nil {
		return nil, err
	}
	if prepare {
		return spell, character.PrepareSpell(spell)
	}
	return spell, character.LearnSpell(spell)
}

// putOn looks an item up and lets the character equip it with put.
func (s *CharacterService) putOn(character *domain.Character, itemName string, put func(*domain.Character, *domain.Equipment) error) (*domain.Equipment, error) {
	item, err := s.equipment.GetByName(itemName)
	if err != nil {
		return nil, err
	}
	return item, put(character, item)
}

// CharacterEdit is a saved character sheet from the web edit page.
type CharacterEdit struct {
	Race, Class                      string
	Level                            int
	Scores                           domain.AbilityScores
	MainHand, OffHand, Armor, Shield string
	KnownSpells, PreparedSpells      []string
}

// Update applies an edited sheet. Level, equipment and spells may change, and
// each change goes through the same domain methods as the single commands, so
// the edit page cannot store an invalid sheet. Nothing is saved on error.
func (s *CharacterService) Update(name string, edit CharacterEdit) (*domain.Character, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	character, err := s.characters.GetByName(name)
	if err != nil {
		return nil, err
	}
	if edit.Race != character.Race() || edit.Class != character.Class() || edit.Scores != character.Scores() {
		return nil, ErrFixedAfterCreation
	}
	if err := character.ChangeLevel(edit.Level); err != nil {
		return nil, err
	}

	character.RemoveEquipment()
	for _, item := range []struct {
		name string
		put  func(*domain.Character, *domain.Equipment) error
	}{
		{edit.MainHand, func(c *domain.Character, e *domain.Equipment) error { return c.EquipWeapon(e, domain.MainHandSlot) }},
		{edit.OffHand, func(c *domain.Character, e *domain.Equipment) error { return c.EquipWeapon(e, domain.OffHandSlot) }},
		{edit.Armor, (*domain.Character).EquipArmor},
		{edit.Shield, (*domain.Character).EquipShield},
	} {
		if item.name == "" {
			continue
		}
		if _, err := s.putOn(character, item.name, item.put); err != nil {
			return nil, err
		}
	}

	character.RemoveSpells()
	for _, spellName := range edit.KnownSpells {
		if _, err := s.addSpell(character, spellName, false); err != nil {
			return nil, err
		}
	}
	for _, spellName := range edit.PreparedSpells {
		if _, err := s.addSpell(character, spellName, true); err != nil {
			return nil, err
		}
	}
	return character, s.characters.Update(character)
}
