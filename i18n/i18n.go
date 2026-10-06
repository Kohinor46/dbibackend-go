// Package i18n holds the UI translations (locales/<code>.json, English is the
// fallback) and the current language.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed locales/*.json
var files embed.FS

// Language is a supported UI language; Name is in the language itself.
type Language struct {
	Code string
	Name string
}

// Languages in the order shown in the language picker.
var Languages = []Language{
	{Code: "en", Name: "English"},
	{Code: "ru", Name: "Русский"},
	{Code: "es", Name: "Español"},
	{Code: "it", Name: "Italiano"},
	{Code: "de", Name: "Deutsch"},
	{Code: "fr", Name: "Français"},
	{Code: "pt", Name: "Português"},
	{Code: "zh", Name: "中文"},
	{Code: "ja", Name: "日本語"},
	{Code: "hi", Name: "हिन्दी"},
	{Code: "ar", Name: "العربية"},
}

const fallback = "en"

var (
	mu       sync.RWMutex
	current  = fallback
	strs     map[string]string
	fallStrs = mustLoad(fallback)
)

func mustLoad(code string) map[string]string {
	m, err := load(code)
	if err != nil {
		panic(err)
	}
	return m
}

func load(code string) (map[string]string, error) {
	data, err := files.ReadFile("locales/" + code + ".json")
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("locales/%s.json: %w", code, err)
	}
	return m, nil
}

// Set switches the UI language; unknown codes fall back to English.
func Set(code string) {
	m, err := load(code)
	if err != nil {
		code, m = fallback, fallStrs
	}
	mu.Lock()
	current, strs = code, m
	mu.Unlock()
}

// Current returns the active language code.
func Current() string {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Match picks the supported language for a locale like "pt-BR" or "zh_Hans_CN".
func Match(locale string) string {
	parts := strings.FieldsFunc(locale, func(r rune) bool { return r == '-' || r == '_' || r == '.' })
	if len(parts) == 0 {
		return fallback
	}
	base := strings.ToLower(parts[0])
	if base == "zh" && isTraditionalChinese(parts[1:]) {
		return fallback // only Simplified Chinese is translated
	}
	for _, l := range Languages {
		if l.Code == base {
			return l.Code
		}
	}
	return fallback
}

// isTraditionalChinese reports whether the subtags after "zh" select
// Traditional script (zh-Hant, zh-TW, zh-HK, zh-MO).
func isTraditionalChinese(subtags []string) bool {
	for _, t := range subtags {
		switch strings.ToLower(t) {
		case "hant", "tw", "hk", "mo":
			return true
		case "hans":
			return false
		}
	}
	return false
}

// T returns the translation of key, formatted with args like fmt.Sprintf.
func T(key string, args ...any) string {
	mu.RLock()
	s, ok := strs[key]
	mu.RUnlock()
	if !ok {
		if s, ok = fallStrs[key]; !ok {
			s = key
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}
