package mail

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
)

// templatesFS embeds every .html file under templates/ into the compiled
// binary, so there's no filesystem path to configure in production —
// the templates ship inside the binary itself.
//
//go:embed templates/*
var templatesFS embed.FS

var templateCache = map[string]*template.Template{}

// RenderTemplate loads templates/<name>.html (cached after first parse)
// and renders it with data. Use this to build the HTMLBody for
// EmailPayload rather than hand-building HTML strings.
//
// data can be a struct or map[string]any; field/key names must match
// the {{PLACEHOLDER}} names in the template, e.g.:
//
//	mail.RenderTemplate("rotary-kitende-confirmation", map[string]any{
//	    "GUEST_NAME":        "Richard Mujjuzi",
//	    "EVENT_TIME":        "4:00 PM",
//	    "REGISTRATION_ID":   "KB-2026-00417",
//	    "EVENT_DETAILS_URL": "https://rotary.siontravel.co.ug/",
//	    "CONTACT_PHONE":     "+256759939977",
//	})
func RenderTemplate(name string, data any) (string, error) {
	tmpl, ok := templateCache[name]
	if !ok {
		var err error
		tmpl, err = template.ParseFS(templatesFS, "templates/"+name+".html")
		if err != nil {
			return "", fmt.Errorf("mail: failed to parse template %q: %w", name, err)
		}
		templateCache[name] = tmpl
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("mail: failed to render template %q: %w", name, err)
	}
	return buf.String(), nil
}
