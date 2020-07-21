package handlers

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	SetupLogger()
	code := m.Run()
	os.Exit(code)
}
