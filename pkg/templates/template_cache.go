package templates

import (
	"fmt"
	htmltemplate "html/template"
	"path/filepath"
	"sync"
	texttemplate "text/template"
)

const templateBasePath = "templates"
const templateFileExtension = "tmpl"

var templateCacheMutex sync.RWMutex
var templateCache = make(map[KnownTemplate]*CachedTemplate)
var textTemplateCache = make(map[KnownTemplate]*CachedTextTemplate)

// CachedTemplate represents a cached template
type CachedTemplate struct {
	templateName    KnownTemplate
	templateContent *htmltemplate.Template
}

// CachedTextTemplate represents a cached plain text template
type CachedTextTemplate struct {
	templateName    KnownTemplate
	templateContent *texttemplate.Template
}

// GetTemplate returns a cached html template instance according to the template name
func GetTemplate(templateName KnownTemplate) (*htmltemplate.Template, error) {
	templateCacheMutex.RLock()
	cachedTemplate, exists := templateCache[templateName]
	templateCacheMutex.RUnlock()

	if exists {
		return cachedTemplate.templateContent, nil
	}

	tmpl, err := htmltemplate.ParseFiles(getTemplateFullPath(templateName))

	if err != nil {
		return nil, err
	}

	templateCacheMutex.Lock()
	templateCache[templateName] = &CachedTemplate{
		templateName:    templateName,
		templateContent: tmpl,
	}
	templateCacheMutex.Unlock()

	return tmpl, nil
}

// GetTextTemplate returns a cached plain text template instance according to the template name.
// Unlike GetTemplate, the rendered content is not html-escaped, so it is used for templates whose
// output is not html, like the large language model system prompts.
func GetTextTemplate(templateName KnownTemplate) (*texttemplate.Template, error) {
	templateCacheMutex.RLock()
	cachedTemplate, exists := textTemplateCache[templateName]
	templateCacheMutex.RUnlock()

	if exists {
		return cachedTemplate.templateContent, nil
	}

	tmpl, err := texttemplate.ParseFiles(getTemplateFullPath(templateName))

	if err != nil {
		return nil, err
	}

	templateCacheMutex.Lock()
	textTemplateCache[templateName] = &CachedTextTemplate{
		templateName:    templateName,
		templateContent: tmpl,
	}
	templateCacheMutex.Unlock()

	return tmpl, nil
}

func getTemplateFullPath(templateName KnownTemplate) string {
	return filepath.Join(templateBasePath, fmt.Sprintf("%s.%s", templateName, templateFileExtension))
}
