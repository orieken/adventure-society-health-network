package openapidocs

import (
	"encoding/json"
	"html/template"
	"net/http"
)

type Operation struct {
	Summary     string
	Description string
	Tags        []string
	RequestBody bool
}

type Service struct {
	Title       string
	Description string
	Version     string
	Paths       map[string]map[string]Operation
}

func Spec(service Service) map[string]any {
	paths := map[string]any{}
	for path, methods := range service.Paths {
		pathItem := map[string]any{}
		for method, operation := range methods {
			item := map[string]any{
				"summary":     operation.Summary,
				"description": operation.Description,
				"tags":        operation.Tags,
				"responses": map[string]any{
					"200": map[string]any{"description": "Successful response"},
					"400": map[string]any{"description": "Invalid request"},
					"404": map[string]any{"description": "Resource not found"},
					"500": map[string]any{"description": "Service error"},
				},
			}
			if operation.RequestBody {
				item["requestBody"] = map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{"schema": map[string]string{"type": "object"}},
						"application/xml":  map[string]any{"schema": map[string]string{"type": "string"}},
					},
				}
			}
			pathItem[method] = item
		}
		paths[path] = pathItem
	}
	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]string{
			"title":       service.Title,
			"description": service.Description,
			"version":     service.Version,
		},
		"paths": paths,
	}
}

func JSONHandler(spec map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(spec)
	}
}

func HTMLHandler(title string) http.HandlerFunc {
	page := template.Must(template.New("docs").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>{{.Title}}</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    body { margin: 0; background: #120c1d; }
    .topbar { display: none; }
    .ashn-header {
      padding: 22px 32px;
      background: linear-gradient(135deg, #120c1d, #22142f);
      color: #fff7eb;
      font-family: Inter, system-ui, sans-serif;
    }
    .ashn-header h1 { margin: 0 0 8px; }
    .ashn-header a { color: #91d7ff; }
  </style>
</head>
<body>
  <header class="ashn-header">
    <h1>{{.Title}}</h1>
    <a href="/openapi.json">OpenAPI JSON</a>
  </header>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({ url: "/openapi.json", dom_id: "#swagger-ui" });
  </script>
</body>
</html>`))
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = page.Execute(w, map[string]string{"Title": title})
	}
}
