package handlers

import (
	"os"
	"testing"

	"github.com/containous/plugin-service/pkg/logger"
)

func TestMain(m *testing.M) {
	logger.Setup()
	code := m.Run()
	os.Exit(code)
}
