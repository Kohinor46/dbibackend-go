package i18n

import (
	"regexp"
	"slices"
	"testing"
)

var verb = regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z%]`)

// Every language must have exactly the English keys and the same format verbs.
func TestLocalesComplete(t *testing.T) {
	en := mustLoad("en")
	for _, l := range Languages {
		m, err := load(l.Code)
		if err != nil {
			t.Errorf("%s: %v", l.Code, err)
			continue
		}
		for k, v := range en {
			tv, ok := m[k]
			if !ok {
				t.Errorf("%s: missing %q", l.Code, k)
				continue
			}
			if !slices.Equal(verb.FindAllString(v, -1), verb.FindAllString(tv, -1)) {
				t.Errorf("%s: %q format verbs differ: %q vs %q", l.Code, k, v, tv)
			}
		}
		for k := range m {
			if _, ok := en[k]; !ok {
				t.Errorf("%s: unknown key %q", l.Code, k)
			}
		}
	}
}

func TestMatch(t *testing.T) {
	for in, want := range map[string]string{"pt-BR": "pt", "zh_Hans_CN": "zh", "ru": "ru", "sv-SE": "en", "AR": "ar",
		"zh-CN": "zh", "zh-Hans-TW": "zh", "zh-Hant": "en", "zh_TW": "en", "zh-Hant-HK": "en", "zh-HK": "en"} {
		if got := Match(in); got != want {
			t.Errorf("Match(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFallback(t *testing.T) {
	Set("xx")
	if Current() != "en" || T("start") != "Start" {
		t.Errorf("fallback failed: %s %s", Current(), T("start"))
	}
	Set("ru")
	if T("files.count", 3, "1 МБ") != "3 шт., 1 МБ" {
		t.Errorf("got %q", T("files.count", 3, "1 МБ"))
	}
	Set("en")
}
