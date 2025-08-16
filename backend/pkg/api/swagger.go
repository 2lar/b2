// Package api provides API utilities and swagger specification management.
package api

import (
	_ "embed"
	"encoding/json"
	"net/http"
	
	"gopkg.in/yaml.v3"
)

//go:embed swagger.yaml
var swaggerYAML []byte

// GetSwaggerSpec returns the embedded swagger specification as bytes
func GetSwaggerSpec() []byte {
	return swaggerYAML
}

// GetSwaggerSpecAsJSON returns the swagger specification converted to JSON
func GetSwaggerSpecAsJSON() ([]byte, error) {
	var spec interface{}
	if err := yaml.Unmarshal(swaggerYAML, &spec); err != nil {
		return nil, err
	}
	return json.Marshal(spec)
}

// SwaggerHandler returns an HTTP handler that serves the swagger specification
func SwaggerHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if JSON is requested
		if r.Header.Get("Accept") == "application/json" {
			jsonSpec, err := GetSwaggerSpecAsJSON()
			if err != nil {
				http.Error(w, "Failed to convert swagger spec to JSON", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonSpec)
			return
		}
		
		// Default to YAML
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(swaggerYAML)
	}
}

// SwaggerUIHandler returns an HTTP handler that serves Swagger UI
func SwaggerUIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Brain2 API Documentation</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui.css">
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            window.ui = SwaggerUIBundle({
                url: "/api/swagger",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout"
            });
        };
    </script>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
	}
}