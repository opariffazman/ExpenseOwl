package web

import (
	"embed"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
)

//go:embed templates
var content embed.FS

// TemplateData holds data to be passed to templates
type TemplateData struct {
	HideSettings bool
}

func GetTemplates() *embed.FS {
	return &content
}

// GetTemplateData returns template data based on environment variables
func GetTemplateData() TemplateData {
	hideSettings := os.Getenv("HIDE_SETTINGS") == "true"
	return TemplateData{
		HideSettings: hideSettings,
	}
}

func ServeTemplate(w http.ResponseWriter, templateName string) error {
	return ServeTemplateWithData(w, templateName, GetTemplateData())
}

func ServeTemplateWithData(w http.ResponseWriter, templateName string, data TemplateData) error {
	tmpl, err := template.ParseFS(content, "templates/"+templateName)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, data)
}

func ServeStatic(w http.ResponseWriter, staticPath string) error {
	staticContent, err := content.ReadFile("templates" + staticPath)
	if err != nil {
		return err
	}
	ext := filepath.Ext(staticPath)
	switch ext {
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	case ".woff", ".woff2":
		w.Header().Set("Content-Type", "font/"+ext[1:])
	case ".ttf":
		w.Header().Set("Content-Type", "font/ttf")
	case ".eot":
		w.Header().Set("Content-Type", "application/vnd.ms-fontobject")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	case ".json":
		w.Header().Set("Content-Type", "application/json")
	}
	_, err = w.Write(staticContent)
	return err
}
