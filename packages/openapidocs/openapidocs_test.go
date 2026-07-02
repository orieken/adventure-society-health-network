package openapidocs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpecBuildsOpenAPIWithRequestBody(t *testing.T) {
	spec := Spec(Service{
		Title:       "ASHN Test Service",
		Description: "Docs for a tiny test guild.",
		Version:     "9.9.9",
		Paths: map[string]map[string]Operation{
			"/items": {
				"get":  {Summary: "List items", Description: "Returns items.", Tags: []string{"items"}},
				"post": {Summary: "Create item", Tags: []string{"items"}, RequestBody: true},
			},
		},
	})

	assert.Equal(t, "3.1.0", spec["openapi"])
	info := spec["info"].(map[string]string)
	assert.Equal(t, "ASHN Test Service", info["title"])
	assert.Equal(t, "Docs for a tiny test guild.", info["description"])
	assert.Equal(t, "9.9.9", info["version"])

	paths := spec["paths"].(map[string]any)
	items := paths["/items"].(map[string]any)
	getOperation := items["get"].(map[string]any)
	assert.Equal(t, "List items", getOperation["summary"])
	assert.NotContains(t, getOperation, "requestBody")

	postOperation := items["post"].(map[string]any)
	requestBody := postOperation["requestBody"].(map[string]any)
	assert.Equal(t, true, requestBody["required"])
	assert.Contains(t, requestBody["content"], "application/json")
	assert.Contains(t, requestBody["content"], "application/xml")
	assert.Contains(t, postOperation, "responses")
}

func TestJSONHandlerServesSpec(t *testing.T) {
	handler := JSONHandler(map[string]any{"openapi": "3.1.0"})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "application/json", response.Header().Get("Content-Type"))
	var payload map[string]string
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	assert.Equal(t, "3.1.0", payload["openapi"])
}

func TestHTMLHandlerServesRootAndRejectsOtherPaths(t *testing.T) {
	handler := HTMLHandler("ASHN Docs")

	rootResponse := httptest.NewRecorder()
	handler.ServeHTTP(rootResponse, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusOK, rootResponse.Code)
	assert.Equal(t, "text/html; charset=utf-8", rootResponse.Header().Get("Content-Type"))
	assert.Contains(t, rootResponse.Body.String(), "<h1>ASHN Docs</h1>")
	assert.Contains(t, rootResponse.Body.String(), "/openapi.json")

	missingResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingResponse, httptest.NewRequest(http.MethodGet, "/missing", nil))
	assert.Equal(t, http.StatusNotFound, missingResponse.Code)
}
