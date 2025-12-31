package render

import "regexp"

type Renderer interface {
	Render(content []byte) []byte
}

type renderEntry struct {
	Regex *regexp.Regexp
	Value []byte
}

type renderer struct {
	Replacements []renderEntry
}

func NewRenderer(vars map[string]string) (Renderer, error) {
	replacements := []renderEntry{}
	for key, repl := range vars {
		patt := "{{[\\s]*\\." + key + "[\\s]*}}"
		rg, err := regexp.Compile(patt)
		if err != nil {
			return nil, err
		}
		replacements = append(replacements, renderEntry{
			Regex: rg,
			Value: []byte(repl),
		})
	}
	return &renderer{Replacements: replacements}, nil
}

func (r *renderer) Render(content []byte) []byte {
	for _, e := range r.Replacements {
		content = e.Regex.ReplaceAll(content, e.Value)
	}
	return content
}
