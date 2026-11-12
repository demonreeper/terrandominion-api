package httpapi

import (
	"testing"

	"github.com/demonreeper/terrandominion/api/internal/config"
)

// TestRoutesRegister builds the router: two patterns that clash make ServeMux panic at start-up, which once took dev
// down on deploy.
func TestRoutesRegister(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("routes clash: %v", r)
		}
	}()
	New(nil, config.Config{}, true).Handler()
}
