package t

import (
	"strings"
	"text/template"
)

type Bundle struct {
	DefaultLanguage string
	Translations    map[string]map[Key]*template.Template
	Funcs           template.FuncMap
}

func NewBundle(defaultLanguage string) Bundle {
	return Bundle{
		DefaultLanguage: defaultLanguage,
		Translations:    make(map[string]map[Key]*template.Template),
		Funcs:           make(template.FuncMap),
	}
}

func (bundle *Bundle) AddFunc(name string, fn interface{}) {
	funcmap := bundle.Funcs
	funcmap[name] = fn
	bundle.Funcs = funcmap
}

func (bundle *Bundle) AddLanguage(lang string, translations map[Key]string) error {
	bundle.Translations[lang] = make(map[Key]*template.Template)

	for key, strtemplate := range translations {
		tmpl := template.New(string(key))

		tmpl.Funcs(bundle.Funcs)

		tmpl, err := tmpl.Parse(strtemplate)
		if err != nil {
			return err
		}
		tmpl = tmpl.Option("missingkey=zero")

		bundle.Translations[lang][key] = tmpl
	}

	return nil
}

func (bundle *Bundle) Check() (missing map[string][]Key) {
	missing = make(map[string][]Key)
	for requiredKey := range bundle.Translations[bundle.DefaultLanguage] {
		for lang, translations := range bundle.Translations {
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

	translationTemplate, exists := bundle.Translations[lang][key]
	if !exists {
		translationTemplate = bundle.Translations[bundle.DefaultLanguage][key]
	}

	err := translationTemplate.Execute(&out, data)
	if err != nil {
		return "", err
	}

	return out.String(), nil
}
