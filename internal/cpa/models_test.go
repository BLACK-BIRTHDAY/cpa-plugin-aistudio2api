package cpa

import (
	"strings"
	"testing"
)

func TestCleanAIStudioDisplayName(t *testing.T) {
	tests := []struct {
		id       string
		rawName  string
		expected string
	}{
		{"gemini-3.1-flash-image", "Nano Banana 2", "Gemini 3.1 Flash Image"},
		{"models/gemini-3.1-flash-image", "Nano Banana 2", "Gemini 3.1 Flash Image"},
		{"gemini-3-pro-image", "Nano Banana Pro", "Gemini 3 Pro Image"},
		{"gemini-3.1-flash-lite-image", "Nano Banana 2 Lite", "Gemini 3.1 Flash Lite Image"},
		{"gemini-nano-banana-2.1", "Nano Banana 2.1", "Gemini Nano Banana 2.1"},
		{"gemini-2.5-flash-image", "Nano Banana", "Gemini 2.5 Flash Image"},
		{"gemini-2.5-flash-image-preview", "Gemini 2.5 Flash Image Preview (Nano Banana)", "Gemini 2.5 Flash Image Preview"},
		{"imagen-3.0-generate-002", "Imagen 3", "Imagen 3"},
		{"gemini-3.8-flash", "Gemini 3.8 Flash", "Gemini 3.8 Flash"},
		{"custom-unknown-model", "", "custom-unknown-model"},
	}

	for _, tc := range tests {
		actual := cleanAIStudioDisplayName(tc.id, tc.rawName)
		if actual != tc.expected {
			t.Errorf("cleanAIStudioDisplayName(%q, %q) = %q, expected %q", tc.id, tc.rawName, actual, tc.expected)
		}
		if strings.Contains(actual, "(") || strings.Contains(actual, ")") {
			t.Errorf("cleanAIStudioDisplayName(%q, %q) = %q contains parentheses", tc.id, tc.rawName, actual)
		}
	}
}

func TestDefaultFallbackModelsIncludeCompatibilityEndpoints(t *testing.T) {
	models := defaultFallbackModels()
	hasImagen := false
	hasEmbedding := false

	for _, m := range models {
		if m.ID == "models/imagen-3.0-generate-002" {
			hasImagen = true
			if strings.Contains(m.Name, "(") || strings.Contains(m.Name, ")") {
				t.Errorf("imagen model name %q contains parentheses", m.Name)
			}
		}
		if m.ID == "models/gemini-embedding-001" {
			hasEmbedding = true
		}
		if strings.Contains(m.Name, "(") || strings.Contains(m.Name, ")") {
			t.Errorf("model name %q contains parentheses", m.Name)
		}
	}

	if !hasImagen {
		t.Errorf("expected defaultFallbackModels to include models/imagen-3.0-generate-002")
	}
	if !hasEmbedding {
		t.Errorf("expected defaultFallbackModels to include models/gemini-embedding-001")
	}
}
func TestCleanAIStudioDescription(t *testing.T) {
	desc := cleanAIStudioDescription("imagen-3.0-generate-002", "")
	if !strings.Contains(desc, "Gemini 3.1 Flash Image") {
		t.Errorf("expected imagen description to reference underlying engine, got: %q", desc)
	}

	embDesc := cleanAIStudioDescription("gemini-embedding-001", "")
	if !strings.Contains(embDesc, "向量嵌入") {
		t.Errorf("expected embedding description to explain embedding usage, got: %q", embDesc)
	}

	bananaDesc := cleanAIStudioDescription("gemini-3.1-flash-image", "")
	if !strings.Contains(bananaDesc, "Nano Banana 2") {
		t.Errorf("expected banana description to reference internal codename, got: %q", bananaDesc)
	}
}
