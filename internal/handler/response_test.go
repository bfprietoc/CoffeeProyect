package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Regresión: writeInternalError se llamaba a sí mismo, así que un solo 500
// desbordaba la pila y mataba todo el servidor. Debe responder una única vez
// con 500 + INTERNAL_ERROR.
func TestWriteInternalError_responds500Once(t *testing.T) {
	rec := httptest.NewRecorder()

	writeInternalError(rec, errors.New("boom"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "INTERNAL_ERROR") {
		t.Errorf("expected INTERNAL_ERROR code in body, got %q", rec.Body.String())
	}
}
