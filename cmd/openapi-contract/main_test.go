package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeDocumentProducesKernelConsumerContract(t *testing.T) {
	document := fixtureDocument()
	normalizeDocument(document, "Example Service API", "v1")

	if err := validateDocument(document, "Example Service API"); err != nil {
		t.Fatal(err)
	}
	info := document["info"].(map[string]any)
	if info["title"] != "Example Service API" || info["version"] != "v1" {
		t.Fatalf("unexpected info: %#v", info)
	}
	operation := document["paths"].(map[string]any)["/v1/things"].(map[string]any)["get"].(map[string]any)
	responses := operation["responses"].(map[string]any)
	defaultSchema := responses["default"].(map[string]any)["schema"].(map[string]any)
	if defaultSchema["$ref"] != "#/definitions/KernelErrorResponse" {
		t.Fatalf("default error ref = %v", defaultSchema["$ref"])
	}
	definitions := document["definitions"].(map[string]any)
	properties := definitions["KernelErrorResponse"].(map[string]any)["properties"].(map[string]any)
	for _, field := range []string{"code", "message", "request_id", "trace_id", "metadata"} {
		if _, ok := properties[field]; !ok {
			t.Errorf("KernelErrorResponse has no %s", field)
		}
	}
}

func TestValidateDocumentRejectsMissingConsumerMetadata(t *testing.T) {
	document := fixtureDocument()
	normalizeDocument(document, "Example Service API", "v1")
	operation := document["paths"].(map[string]any)["/v1/things"].(map[string]any)["get"].(map[string]any)
	delete(operation, "operationId")
	delete(operation, "tags")

	err := validateDocument(document, "Example Service API")
	if err == nil || !strings.Contains(err.Error(), "has no operationId") || !strings.Contains(err.Error(), "has no tags") {
		t.Fatalf("validateDocument() error = %v", err)
	}
}

func TestNormalizedEncodingIsDeterministic(t *testing.T) {
	document := fixtureDocument()
	normalizeDocument(document, "Example Service API", "v1")
	first, err := marshalDocument(document)
	if err != nil {
		t.Fatal(err)
	}
	second, err := marshalDocument(document)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("normalized OpenAPI encoding is not deterministic")
	}
}

func TestDecodeDocumentAcceptsPowerShellUTF8BOM(t *testing.T) {
	raw := append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"swagger":"2.0"}`)...)
	if _, err := decodeDocument(raw); err != nil {
		t.Fatalf("decodeDocument() error = %v", err)
	}
}

func TestRunWritesAStableTrackedContract(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "generated.swagger.json")
	output := filepath.Join(directory, "service.swagger.json")
	raw := `{"swagger":"2.0","info":{"title":"generated.proto","version":"version not set"},"paths":{"/v1/things":{"get":{"operationId":"ThingService_GetThing","tags":["ThingService"],"responses":{"default":{"description":"Error"}}}}},"definitions":{}}`
	if err := os.WriteFile(input, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	opts := options{input: input, output: output, title: "Example Service API", version: "v1"}
	if err := run(opts); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := run(opts); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("repeated normalization changed the tracked contract")
	}
}

func fixtureDocument() map[string]any {
	return map[string]any{
		"swagger": "2.0",
		"info":    map[string]any{"title": "generated.proto", "version": "version not set"},
		"paths": map[string]any{
			"/v1/things": map[string]any{
				"get": map[string]any{
					"operationId": "ThingService_GetThing",
					"tags":        []any{"ThingService"},
					"responses": map[string]any{
						"200":     map[string]any{"description": "OK"},
						"default": map[string]any{"description": "Error", "schema": map[string]any{"$ref": "#/definitions/rpcStatus"}},
					},
				},
			},
		},
		"definitions": map[string]any{},
	}
}
