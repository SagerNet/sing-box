package locale

import (
	"strings"
	"sync/atomic"

	"golang.org/x/text/language"
)

var (
	localeRegistry = map[string]*Locale{
		"en": defaultLocale,
	}
	localeMatcher = language.NewMatcher(
		[]language.Tag{
			language.English,
			language.SimplifiedChinese,
			language.TraditionalChinese,
			language.Persian,
			language.Russian,
		},
		language.PreferSameScript(true),
	)
	localeNames = []string{"en", "zh-Hans", "zh-Hant", "fa", "ru"}
	current     atomic.Pointer[Locale]
)

type Locale struct {
	Locale                  string
	DeprecatedMessage       string
	DeprecatedMessageNoLink string
	InsecureFeatureMessage  string
	ExternalPathFeature     string
}

var defaultLocale = &Locale{
	Locale:                  "en",
	DeprecatedMessage:       "%s is deprecated in sing-box %s and will be removed in sing-box %s. Please check the documentation for migration.",
	DeprecatedMessageNoLink: "%s is deprecated in sing-box %s and will be removed in sing-box %s.",
	InsecureFeatureMessage:  "%s is considered insecure in the graphical client for sing-box on Windows. Enable Insecure Mode in `Settings - Core - Insecure Mode` to use it.",
	ExternalPathFeature:     "Access to %s (outside of the working directory) is considered insecure in the graphical client for sing-box on Windows. Enable Insecure Mode in `Settings - Core - Insecure Mode` to use it.",
}

func init() {
	current.Store(defaultLocale)
}

func Current() *Locale {
	return current.Load()
}

func Set(localeID string) bool {
	localeEntries := strings.Split(localeID, ",")
	for i, localeEntry := range localeEntries {
		languageID, options, hasOptions := strings.Cut(localeEntry, ";")
		languageID, _, _ = strings.Cut(strings.TrimSpace(languageID), "@")
		languageID = strings.ReplaceAll(languageID, "_", "-")
		if !hasOptions {
			languageID, _, _ = strings.Cut(languageID, ".")
		}
		switch {
		case strings.EqualFold(languageID, "C"), strings.EqualFold(languageID, "POSIX"):
			languageID = "en"
		case strings.EqualFold(languageID, "zh-CHS"):
			languageID = "zh-Hans"
		case strings.EqualFold(languageID, "zh-CHT"):
			languageID = "zh-Hant"
		}
		localeEntries[i] = languageID
		if hasOptions {
			localeEntries[i] += ";" + options
		}
	}
	localeID = strings.Join(localeEntries, ",")
	tags, _, err := language.ParseAcceptLanguage(localeID)
	if err != nil || len(tags) == 0 {
		return false
	}
	for i, tag := range tags {
		base, script, region := tag.Raw()
		if base.String() != "zh" && base.String() != "cmn" {
			continue
		}
		if script.String() == "Hans" || script.String() == "Hant" {
			continue
		}
		languageID := "zh-Hans"
		if region.String() == "TW" || region.String() == "HK" || region.String() == "MO" {
			languageID = "zh-Hant"
		}
		if region.String() != "ZZ" {
			languageID += "-" + region.String()
		}
		tags[i] = language.MustParse(languageID)
	}
	_, localeIndex, _ := localeMatcher.Match(tags...)
	selectedLocale, loaded := localeRegistry[localeNames[localeIndex]]
	if !loaded {
		return false
	}
	current.Store(selectedLocale)
	return true
}
