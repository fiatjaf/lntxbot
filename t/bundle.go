package t

import (
	"strings"
	"text/template"
)

type Bundle struct {
	defaultLanguage string
	translations    map[string]map[Key]*template.Template
}

func NewBundle(defaultLanguage string) Bundle {
	return Bundle{
		defaultLanguage: defaultLanguage,
		translations:    make(map[string]map[Key]*template.Template),
	}
}

func (bundle *Bundle) AddLanguage(lang string, translations map[Key]string) error {
	bundle.translations[lang] = make(map[Key]*template.Template)

	for key, strtemplate := range translations {
		tmpl := template.New(string(key))

		tmpl, err := tmpl.Parse(strtemplate)
		if err != nil {
			return err
		}

		bundle.translations[lang][key] = tmpl
	}

	return nil
}

func (bundle *Bundle) Check() (missing map[string][]Key) {
	missing = make(map[string][]Key)
	for requiredKey, _ := range bundle.translations[bundle.defaultLanguage] {
		for lang, translations := range bundle.translations {
			_, exists := translations[requiredKey]
			if !exists {
				missing[lang] = append(missing[lang], requiredKey)
			}
		}
	}

	return
}

func (bundle *Bundle) Render(lang string, key Key, data interface{}) (string, error) {
	out := strings.Builder{}

	translationTemplate, exists := bundle.translations[lang][key]
	if !exists {
		translationTemplate = bundle.translations[bundle.defaultLanguage][key]
	}

	err := translationTemplate.Execute(&out, data)
	if err != nil {
		return "", err
	}

	return out.String(), nil
}
