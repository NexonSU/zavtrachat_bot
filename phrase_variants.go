package main

import (
	"errors"
	"html"
	"regexp"
	"strings"
	"unicode"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PhraseVariants struct {
	Male   string `gorm:"column:male_text;not null;default:''"`
	Female string `gorm:"column:female_text;not null;default:''"`
}

func (p PhraseVariants) forGender(female bool, original, fallback string) string {
	if female {
		if p.Female != "" {
			return p.Female
		}
		if original != "" {
			return fallback
		}
		return ""
	}
	if p.Male != "" {
		return p.Male
	}
	return original
}

func (p PhraseVariants) confirmation() string {
	if p.Male == p.Female {
		return ""
	}
	return "\nМужской: <code>" + html.EscapeString(p.Male) + "</code>" +
		"\nЖенский: <code>" + html.EscapeString(p.Female) + "</code>"
}

var explicitPhraseForm = regexp.MustCompile(`\{([^{}|]*)\|([^{}|]*)\}`)
var phraseStart = regexp.MustCompile(`(?i)^[^\pL]*(?:(?:вдруг|случайно|чудом|снова|опять|почти|едва|уже|наконец|просто|удачно|неудачно|не)\s+)*([\p{Cyrillic}]+)`)
var phraseCoordination = regexp.MustCompile(`(?i)(?:\s|,)(?:и|но|зато|&)\s+(?:не\s+)?([\p{Cyrillic}]+)`)
var phraseSubordinate = regexp.MustCompile(`(?i)[.!?…;\n]|(?:,|\s)\s*(?:что|чтобы|хотя|если|когда|пока|который|которая|которое|где|будто|потому|словно)(?:\s|,)`)

type gameWordForms struct{ male, female string }

var gamePhraseWords = func() map[string]gameWordForms {
	words := make(map[string]gameWordForms)
	for _, male := range strings.Fields(`
		сел присел упал пал попал прыгнул спрыгнул запрыгнул отскочил
		выжил ожил спал уснул заснул встал летел полетел улетел прилетел
		взлетел отлетел взял принял поднял сделал сказал забыл решил
		съел выпил получил поймал избежал пережил выдержал победил сбежал
		убежал пробежал дожил угодил наступил тронул схватил проглотил
		утонул сгорел был стал
	`) {
		pair := gameWordForms{male, male + "а"}
		words[pair.male], words[pair.female] = pair, pair
	}
	for _, forms := range strings.Fields(`
		пришёл/пришла пришел/пришла ушёл/ушла ушел/ушла нашёл/нашла нашел/нашла
		пошёл/пошла пошел/пошла вышел/вышла зашёл/зашла зашел/зашла
		дошёл/дошла дошел/дошла прошёл/прошла прошел/прошла
		смог/смогла помог/помогла лёг/легла лег/легла сжёг/сожгла сжег/сожгла
		спас/спасла погиб/погибла умер/умерла замёрз/замёрзла замерз/замерзла
		спасся/спаслась обжёгся/обожглась обжегся/обожглась
		спасён/спасена спасен/спасена жив/жива рад/рада цел/цела
	`) {
		male, female, _ := strings.Cut(forms, "/")
		pair := gameWordForms{male, female}
		words[pair.male], words[pair.female] = pair, pair
	}
	return words
}()

func preservePhraseCase(source, form string) string {
	if source == strings.ToUpper(source) {
		return strings.ToUpper(form)
	}
	first := []rune(source)[0]
	if unicode.IsUpper(first) {
		runes := []rune(form)
		runes[0] = unicode.ToUpper(runes[0])
		return string(runes)
	}
	return form
}

func inferPhraseVariants(text string) (PhraseVariants, bool) {
	start := phraseStart.FindStringSubmatchIndex(text)
	if start == nil {
		return PhraseVariants{Male: text, Female: text}, false
	}
	limit := len(text)
	if subordinate := phraseSubordinate.FindStringIndex(text[start[3]:]); subordinate != nil {
		limit = start[3] + subordinate[0]
	}
	indices := [][2]int{{start[2], start[3]}}
	tail := text[start[3]:limit]
	// Short forms immediately after a copula agree with the same subject.
	switch strings.ToLower(text[start[2]:start[3]]) {
	case "был", "была", "стал", "стала":
		if next := phraseStart.FindStringSubmatchIndex(tail); next != nil {
			switch strings.ToLower(tail[next[2]:next[3]]) {
			case "спасён", "спасен", "спасена", "жив", "жива", "рад", "рада", "цел", "цела":
				indices = append(indices, [2]int{start[3] + next[2], start[3] + next[3]})
			}
		}
	}
	for _, match := range phraseCoordination.FindAllStringSubmatchIndex(tail, -1) {
		indices = append(indices, [2]int{start[3] + match[2], start[3] + match[3]})
	}
	var male, female strings.Builder
	cursor := 0
	for i, index := range indices {
		word := text[index[0]:index[1]]
		lower := strings.ToLower(word)
		pair, ok := gamePhraseWords[lower]
		if !ok {
			if stem, matched := strings.CutSuffix(lower, "лся"); matched && stem != "" {
				pair, ok = gameWordForms{lower, stem + "лась"}, true
			} else if stem, matched := strings.CutSuffix(lower, "лась"); matched && stem != "" {
				pair, ok = gameWordForms{stem + "лся", lower}, true
			}
		}
		if !ok {
			if i == 0 {
				return PhraseVariants{Male: text, Female: text}, false
			}
			break
		}
		male.WriteString(text[cursor:index[0]])
		female.WriteString(text[cursor:index[0]])
		male.WriteString(preservePhraseCase(word, pair.male))
		female.WriteString(preservePhraseCase(word, pair.female))
		cursor = index[1]
	}
	male.WriteString(text[cursor:])
	female.WriteString(text[cursor:])
	return PhraseVariants{Male: male.String(), Female: female.String()}, true
}

func makePhraseVariants(text string) (variants PhraseVariants, explicit bool, err error) {
	text = strings.TrimSpace(text)
	variants, explicit, err = parseExplicitPhraseForms(text)
	if !explicit && err == nil {
		variants, _ = inferPhraseVariants(text)
	}
	return variants, explicit, err
}

func parseExplicitPhraseForms(text string) (PhraseVariants, bool, error) {
	matches := explicitPhraseForm.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return PhraseVariants{Male: text, Female: text}, false, nil
	}
	var male, female strings.Builder
	cursor := 0
	for _, match := range matches {
		m, f := strings.TrimSpace(text[match[2]:match[3]]), strings.TrimSpace(text[match[4]:match[5]])
		if m == "" || f == "" {
			return PhraseVariants{}, false, errors.New("Укажи обе формы в скобках, например {сел|села}.")
		}
		male.WriteString(text[cursor:match[0]])
		female.WriteString(text[cursor:match[0]])
		male.WriteString(m)
		female.WriteString(f)
		cursor = match[1]
	}
	male.WriteString(text[cursor:])
	female.WriteString(text[cursor:])
	return PhraseVariants{Male: male.String(), Female: female.String()}, true, nil
}

func savePhraseVariants(row any, explicit bool) error {
	if explicit {
		// An edited phrase keeps its original masculine text as the record key.
		return DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "text"}},
			DoUpdates: clause.AssignmentColumns([]string{"male_text", "female_text"}),
		}).Create(row).Error
	}
	return DB.Create(row).Error
}

