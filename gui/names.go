package gui

import (
	"path"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// nameMode is what an upload does with names a backend can't store (DBI's
// FTP server only handles Latin letters, digits and ASCII punctuation;
// other names are refused or saved invisible).
type nameMode int

const (
	namesAsIs  nameMode = iota // the backend takes any name
	namesLatin                 // transliterate them (latinName)
	namesSkip                  // leave those files and folders out
)

// latinOnly is implemented by backends that need Latin names.
type latinOnly interface{ LatinNamesOnly() }

// isLatinName reports whether name has only printable ASCII that FAT and
// exFAT allow.
func isLatinName(name string) bool {
	for _, r := range name {
		if r < 32 || r > 126 || strings.ContainsRune(`<>:"\|?*`, r) {
			return false
		}
	}
	return true
}

// hasLatinPath reports whether every element of the slash-separated rel is
// a Latin name.
func hasLatinPath(rel string) bool {
	for _, p := range strings.Split(rel, "/") {
		if !isLatinName(p) {
			return false
		}
	}
	return true
}

var cyrillic = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "zh",
	'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o",
	'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "kh", 'ц': "ts",
	'ч': "ch", 'ш': "sh", 'щ': "shch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu",
	'я': "ya", 'є': "ye", 'і': "i", 'ї': "yi", 'ґ': "g", 'ў': "u",
}

// latinName turns name into a Latin one: Cyrillic is transliterated
// ("Паспорт" → "Pasport"), accents are dropped ("café" → "cafe"), and any
// other character becomes "_".
func latinName(name string) string {
	var b strings.Builder
	for _, r := range norm.NFC.String(name) { // macOS file names are decomposed
		if isLatinName(string(r)) {
			b.WriteRune(r)
			continue
		}
		if s, ok := cyrillic[unicode.ToLower(r)]; ok {
			if unicode.IsUpper(r) && s != "" {
				s = strings.ToUpper(s[:1]) + s[1:]
			}
			b.WriteString(s)
			continue
		}
		base := "" // the letter without its accents
		for _, d := range norm.NFD.String(string(r)) {
			if !unicode.Is(unicode.Mn, d) {
				base += string(d)
			}
		}
		if base != "" && isLatinName(base) {
			b.WriteString(base)
		} else {
			b.WriteByte('_')
		}
	}
	s := b.String()
	if t := strings.TrimRight(s, ". "); t != s {
		s = t + strings.Repeat("_", len(s)-len(t))
	}
	if s == "" || s == "." || s == ".." {
		return "_"
	}
	return s
}

// numbered returns name with " (n)" before its extension.
func numbered(name string, n int) string {
	ext := path.Ext(name)
	if ext == name {
		ext = ""
	}
	return strings.TrimSuffix(name, ext) + " (" + strconv.Itoa(n) + ")" + ext
}
