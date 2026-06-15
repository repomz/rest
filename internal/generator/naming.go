package generator

import (
	"strings"
	"unicode"
)

func kebab(s string) string {
	return strings.ReplaceAll(lowerSnake(s), "_", "-")
}

func lowerSnake(s string) string {
	words := splitNameWords(s)
	for i := range words {
		words[i] = strings.ToLower(words[i])
	}
	return strings.Join(words, "_")
}

func splitNameWords(s string) []string {
	s = normalizeNameAcronymBoundaries(s)
	var words []string
	var current []rune
	runes := []rune(s)
	for i, r := range runes {
		if r == '_' || r == '-' || r == ' ' {
			if len(current) > 0 {
				words = append(words, string(current))
				current = nil
			}
			continue
		}
		if len(current) > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			var next rune
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			if !unicode.IsUpper(prev) || (next != 0 && unicode.IsLower(next)) {
				words = append(words, string(current))
				current = nil
			}
		}
		current = append(current, r)
	}
	if len(current) > 0 {
		words = append(words, string(current))
	}
	return words
}

func normalizeNameAcronymBoundaries(s string) string {
	acronyms := []string{"UUID", "HTTP", "SQL", "URL", "API", "DB", "ID"}
	for _, acronym := range acronyms {
		s = strings.ReplaceAll(s, acronym+"and", acronym+"And")
		s = strings.ReplaceAll(s, acronym+"or", acronym+"Or")
	}
	return s
}

func cleanIdent(s string) string {
	return strings.Trim(s, "\"")
}

func singular(s string) string {
	if strings.HasSuffix(s, "ies") {
		return strings.TrimSuffix(s, "ies") + "y"
	}
	if strings.HasSuffix(s, "s") {
		return strings.TrimSuffix(s, "s")
	}
	return s
}

func exported(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		if hasUpper(part) {
			runes := []rune(part)
			runes[0] = unicode.ToUpper(runes[0])
			b.WriteString(string(runes))
			continue
		}
		lower := strings.ToLower(part)
		if initialism, ok := commonInitialisms[lower]; ok {
			b.WriteString(initialism)
			continue
		}
		runes := []rune(lower)
		runes[0] = unicode.ToUpper(runes[0])
		b.WriteString(string(runes))
	}
	return b.String()
}

func hasUpper(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

var commonInitialisms = map[string]string{
	"api":   "API",
	"db":    "DB",
	"dns":   "DNS",
	"html":  "HTML",
	"http":  "HTTP",
	"https": "HTTPS",
	"id":    "ID",
	"ip":    "IP",
	"json":  "JSON",
	"sql":   "SQL",
	"uid":   "UID",
	"url":   "URL",
	"uuid":  "UUID",
	"xml":   "XML",
}
