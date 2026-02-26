package banggin_test

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin/binding"
	"github.com/smdmitry/bang"
	"github.com/smdmitry/bang/bangval"
)

type payload struct {
	Name string `json:"name" binding:"required"`
}

func TestWrapValidationErrorFromGinSliceValidationError(t *testing.T) {
	err := binding.Validator.ValidateStruct([]payload{{}, {}})
	if err == nil {
		t.Fatal("expected validation error")
	}

	wrapped := bangval.WrapValidationError(err)
	if wrapped == nil {
		t.Fatal("expected wrapped error")
	}
	if wrapped.GetClass() != bang.ClassValidation {
		t.Fatalf("class=%v", wrapped.GetClass())
	}
	if wrapped.GetCode() != "request.validation" {
		t.Fatalf("code=%q", wrapped.GetCode())
	}

	fields := wrapped.GetValidationFields()
	if len(fields) != 2 {
		t.Fatalf("fields=%d", len(fields))
	}
	for i, field := range fields {
		if field.Key != "name" {
			t.Fatalf("fields[%d].Key=%q", i, field.Key)
		}
		if field.Rule != "required" {
			t.Fatalf("fields[%d].Rule=%q", i, field.Rule)
		}
	}
}

func TestWrapValidationErrorFromWrappedGinSliceValidationError(t *testing.T) {
	err := binding.Validator.ValidateStruct([]payload{{}})
	if err == nil {
		t.Fatal("expected validation error")
	}

	wrapped := bangval.WrapValidationError(fmt.Errorf("outer: %w", err))
	if wrapped == nil {
		t.Fatal("expected wrapped error")
	}

	fields := wrapped.GetValidationFields()
	if len(fields) != 1 {
		t.Fatalf("fields=%d", len(fields))
	}
	if fields[0].Key != "name" {
		t.Fatalf("field key=%q", fields[0].Key)
	}
}
