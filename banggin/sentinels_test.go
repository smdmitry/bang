package banggin_test

import (
	"errors"
	"testing"

	"github.com/smdmitry/bang"
)

func TestSentinelMatchByClass(t *testing.T) {
	err := bang.NotFound().Code("users.get")
	if !errors.Is(err, bang.ErrNotFound) {
		t.Fatal("expected class sentinel match")
	}
	if errors.Is(err, bang.ErrConflict) {
		t.Fatal("unexpected class sentinel match")
	}
}

func TestSentinelMatchThroughWrap(t *testing.T) {
	err := bang.InternalServer().Wrap(bang.Timeout())
	if !errors.Is(err, bang.ErrTimeout) {
		t.Fatal("expected wrapped class sentinel match")
	}
}
