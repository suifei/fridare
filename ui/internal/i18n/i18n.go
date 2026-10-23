// Package i18n is a thin in-repo string catalog.
// Chinese text in the source is the message key and the default UI language.
// English lives in en.json. Missing keys fall back to the Chinese original.
package i18n

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// LangZh is the default language (Chinese-first product).
	LangZh = "zh"
	// LangEn is the English UI.
	LangEn = "en"

	// LabelZh and LabelEn are shown in their own language so the selector
	// stays recognizable after a switch.
	LabelZh = "中文"
	LabelEn = "English"
)

//go:embed en.json
var enJSON []byte

var (
	mu        sync.RWMutex
	current   = LangZh
	catalogEN map[string]string
)

func init() {
	catalogEN = map[string]string{}
	if len(strings.TrimSpace(string(enJSON))) == 0 {
		return
	}
	if err := json.Unmarshal(enJSON, &catalogEN); err != nil {
		panic("i18n: en.json: " + err.Error())
	}
}

// SetLanguage selects zh or en. Any other value (including empty) is Chinese.
func SetLanguage(lang string) {
	mu.Lock()
	current = normalize(lang)
	mu.Unlock()
}

// Current returns "zh" or "en".
func Current() string {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Normalize maps user input to "zh" or "en".
func Normalize(lang string) string {
	return normalize(lang)
}

func normalize(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case LangEn, "english":
		return LangEn
	default:
		return LangZh
	}
}

// LanguageLabels is the settings selector order.
func LanguageLabels() []string {
	return []string{LabelZh, LabelEn}
}

// LabelFor returns the selector label for a stored language code.
func LabelFor(lang string) string {
	if normalize(lang) == LangEn && isExplicitEN(lang) {
		return LabelEn
	}
	return LabelZh
}

func isExplicitEN(lang string) bool {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case LangEn, "english":
		return true
	default:
		return false
	}
}

// CodeFromLabel maps a selector label back to a language code.
func CodeFromLabel(label string) string {
	if strings.TrimSpace(label) == LabelEn {
		return LangEn
	}
	return LangZh
}

// T returns the English catalog entry when the UI language is English.
// Otherwise it returns zh unchanged.
func T(zh string) string {
	mu.RLock()
	lang := current
	mu.RUnlock()
	if lang != LangEn {
		return zh
	}
	if en, ok := catalogEN[zh]; ok && en != "" {
		return en
	}
	return zh
}

// Tf translates a format string then applies fmt.Sprintf.
func Tf(zh string, args ...any) string {
	return fmt.Sprintf(T(zh), args...)
}

// InitFromUserConfig reads the saved GUI language, if a config file exists.
// It does not create the file. Tests stay on Chinese unless they call SetLanguage.
func InitFromUserConfig() {
	dir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	data, err := os.ReadFile(filepath.Join(dir, "fridare", "config.json"))
	if err != nil {
		return
	}
	var probe struct {
		Language string `json:"language"`
	}
	if json.Unmarshal(data, &probe) != nil {
		return
	}
	SetLanguage(probe.Language)
}

// CatalogEN returns a copy of the English map (tests).
func CatalogEN() map[string]string {
	out := make(map[string]string, len(catalogEN))
	for k, v := range catalogEN {
		out[k] = v
	}
	return out
}
