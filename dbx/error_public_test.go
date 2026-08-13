package dbx

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestNormalizeGORMErrorMapsRecordNotFound(t *testing.T) {
	err := NormalizeGORMError(gorm.ErrRecordNotFound)
	if !errors.Is(err, ErrNoRows) {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}

func TestNormalizeGORMErrorNil(t *testing.T) {
	if err := NormalizeGORMError(nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
