package model

import (
	"errors"
	"testing"
)

func TestValidateDocument(t *testing.T) {
	ok := map[string]string{
		"solid":    `{"version":"1.0","id":"a","name":"Ink","kind":"solid","params":{"color":"#0f172a"}}`,
		"gradient": `{"kind":"gradient","params":{"type":"linear","colors":["#D81B8C","rgb(106, 13, 173)"],"angle":135}}`,
		"shader":   `{"kind":"shader","shaderType":"mesh-gradient","category":"effects","params":{"colors":["#fff","hsl(300, 50%, 40%)"],"speed":0.4,"distortion":1,"fit":"cover"}}`,
		"video":    `{"kind":"video","params":{"src":"https://cdn.example.com/bg.mp4","muted":true,"loop":true}}`,
		"labels":   `{"kind":"solid","name":"Gece Mavisi — ğüşiöç","createdAt":"2026-10-12T13:00:00.000Z","metadata":{"tags":["koyu","mor"],"description":"Açıklama: güzel"},"params":{"color":"#000"}}`,
		"image":    `{"kind":"shader","shaderType":"image-dithering","category":"imageFilters","params":{"image":"https://cdn.example.com/a.png"}}`,
	}
	for name, doc := range ok {
		if _, err := ValidateDocument([]byte(doc)); err != nil {
			t.Errorf("%s: unexpected %v", name, err)
		}
	}
	bad := map[string]string{
		"not object":      `[1,2]`,
		"unknown kind":    `{"kind":"html","params":{}}`,
		"unknown shader":  `{"kind":"shader","shaderType":"evil","params":{}}`,
		"no params":       `{"kind":"solid"}`,
		"css breakout":    `{"kind":"solid","params":{"color":"red; background:url(https://x)"}}`,
		"javascript url":  `{"kind":"video","params":{"src":"javascript:alert(1)"}}`,
		"http video":      `{"kind":"video","params":{"src":"http://insecure/x.mp4"}}`,
		"markup":          `{"kind":"solid","name":"<script>","params":{"color":"#fff"}}`,
		"deep nesting":    `{"kind":"solid","params":{"a":{"b":{"c":{"d":1}}}}}`,
		"quote in url":    `{"kind":"video","params":{"src":"https://x.com/a\".mp4"}}`,
	}
	for name, doc := range bad {
		if _, err := ValidateDocument([]byte(doc)); !errors.Is(err, ErrInvalidDocument) {
			t.Errorf("%s: expected ErrInvalidDocument, got %v", name, err)
		}
	}
	huge := make([]byte, MaxDocumentBytes+1)
	if _, err := ValidateDocument(huge); err == nil {
		t.Error("oversized document accepted")
	}
}
