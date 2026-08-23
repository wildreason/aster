package main

// CLI-side security assertions that stayed behind when security_test.go
// moved into engine/ (they test server.go / main.go symbols).

import (
	"strings"
	"testing"
)

// --- Server binding ---

func TestServerBindsLocalhost(t *testing.T) {
	// Verify the address format produces localhost binding
	port := 3000
	addr := formatServerAddr(port)
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Errorf("server should bind to 127.0.0.1, got: %s", addr)
	}
}

// --- File size limit ---

func TestMaxFileSizeConstant(t *testing.T) {
	if maxFileSize != 100*1024*1024 {
		t.Errorf("maxFileSize should be 100MB, got %d", maxFileSize)
	}
}
