# Domain glossary (ubiquitous language)

Terms come from the SRD 5.1 and the assignment README.

| Term | Definition | Context | Implementation reference |
|---|---|---|---|
| Character | A player character with a name, race, class, background, level and ability scores | Character sheet | `domain.Character` (`internal/domain/character.go`) |
| Character name | Unique identifier of a character | Character sheet | `Character.Name()`; uniqueness in `CharacterService.Create` |
| Race | Species of the character (elf, dwarf, half-orc, ...); grants ability score increases | Catalogue / sheet | `Character.Race()`; `data/RaceAbilityBonuses.csv` |
| Ability score increase (racial bonus) | Points a race adds to an ability, e.g. "+2 Dexterity" | Catalogue | `domain.RaceAbilityBonus`, `Character.ApplyAbilityBonus` |
| Class | The character's profession (wizard, fighter, ...); decides skills and spellcasting | Catalogue / sheet | `domain.Class`, `data/Classes.csv` |
| Subclass | Specialisation of a class – **out of scope** (assignment) | – | – |
| Background | The character's history; Acolyte grants Insight and Religion | Character sheet | `backgroundSkills` (`rules.go`) |
| Level | 1–20; drives proficiency bonus and spell slots | Character sheet | `Character.Level()`, `ChangeLevel` |
| Ability score | Strength, Dexterity, Constitution, Intelligence, Wisdom, Charisma | Character sheet | `domain.AbilityScores` |
| Ability modifier | floor((score − 10) / 2) | Rules | `domain.AbilityModifier` |
| Standard Array | Scores 15, 14, 13, 12, 10, 8, each used once | Character sheet | `AbilityScores.HasStandardArray` |
| Proficiency bonus | +2 at levels 1–4 up to +6 at 17–20 | Rules | `domain.ProficiencyBonus` |
| Skill proficiency | A trained skill; the class gives the first N options, the background adds its own | Character sheet | `Character.AddSkillProficiencies`, `AddBackgroundSkills` |
| Armor class (AC) | How hard the character is to hit: armor formula + Dex (capped) + shield | Rules | `armorClass` (`rules.go`), `Character.ArmorClass` |
| Unarmored Defense | Barbarian: 10 + Dex + Con; Monk: 10 + Dex + Wis (no armor, no shield) | Rules | `armorClass` |
| Initiative | Dexterity modifier | Rules | `Character.Initiative` |
| Passive perception ("passive Wisdom" on the sheet) | 10 + Wisdom modifier + proficiency bonus if proficient in Perception | Rules | `Character.PassivePerception` |
| Main hand / off hand | The two weapon slots | Character sheet | `MainHandSlot`, `OffHandSlot`, `EquipWeapon` |
| Armor / shield slot | Worn body armor; carried shield (+2 AC) | Character sheet | `EquipArmor`, `EquipShield` |
| Equipment (item) | An entry of the SRD equipment list with a type (Weapon, Armor, ...) | Catalogue | `domain.Equipment` |
| Inventory | Unequipped gear – **out of scope** (assignment) | – | – |
| Weight | Weight of an item in pounds, from the SRD (third column of the equipment CSV); items without a listed weight (packs, mounts, vehicles) count as 0 | Catalogue | `Equipment.Weight`, `WithWeight` |
| Carrying capacity | 20 + 2 × Strength modifier, in pounds (exam rule, not the SRD's 15 × Strength) | Rules | `domain.CarryingCapacity` |
| Encumbrance / condition | Carried weight (all equipped items) compared with the capacity: up to the capacity *none*, up to twice the capacity *encumbered*, more *heavily encumbered* | Rules / character sheet | `domain.EncumbranceFor`, `Character.Encumbrance`, `CharacterQueries.Encumbrance` |
| Spell | A spell with a level (0 = cantrip) and the classes that may use it | Catalogue | `domain.Spell` |
| Cantrip | Level 0 spell; shown as "Level 0" in the CLI | Rules | `cantripsKnown` |
| Spell slot | Capacity to cast a spell of a given level | Rules | `spellSlots`, `Character.SpellSlots` |
| Pact Magic | Warlock slots: few slots, all of the highest level; can cast lower-level spells | Rules | `pactMagicSlots`, `highestSpellLevel` |
| Learn (known spells) | Bards, rangers, sorcerers, warlocks add spells permanently | Character sheet | `Character.LearnSpell` |
| Prepare (prepared spells) | Clerics, druids, paladins, wizards choose spells to prepare | Character sheet | `Character.PrepareSpell` |
| Spellcasting ability | The ability a class casts with (Int, Wis or Cha) | Rules | `Character.SpellcastingAbility` |
| Spell save DC | 8 + proficiency bonus + spellcasting modifier | Rules | `Character.SpellSaveDC` |
| Spell attack bonus | proficiency bonus + spellcasting modifier | Rules | `Character.SpellAttackBonus` |
| Enrichment | Extra details (school, range, category, AC) from the external SRD API | External SRD | `CharacterQueries.EnrichCharacter`, `SRDAPIClient` |

## Ambiguities and naming notes

| Observation | Decision |
|---|---|
| The sheet template says "Passive Wisdom", the code says "passive perception" | Same concept; the code uses the SRD term *passive Wisdom (Perception)* → `PassivePerception`. The web DTO keeps the field name `PassiveWisdom` because the page script uses it |
| "Half Plate Armor" (CSV) vs "half plate" (armor rules and stored data) | Treated as synonyms; stored as `half plate` (constant `halfPlate`) |
| "half orc" vs "half-orc" | Treated as the same race (`normalizeRaceName`) |
| CLI prints cantrips as "Level 0" spell slots | Kept for the CLI contract; strictly, cantrips are not slots |
| `SpellRepository` lives in `spells_repository.go` but the type is `Spell` (singular) | File names kept from the original project |
| *Proposed convention*: "sheet" = the calculated view of a character; "character" = the stored aggregate | Used in the documentation |
| The encumbrance rule says "less than" the capacity and "more than" it, but not what exactly the capacity is | Exactly the capacity → *none*; exactly twice the capacity → *encumbered* (it is not "more than"). Tested in `TestEncumbranceBoundaries` |
| "Carries" – inventory is out of scope, so only equipped items are carried | The carried weight is the sum of the main hand, off hand, armor and shield; a weapon in both hands counts twice |
