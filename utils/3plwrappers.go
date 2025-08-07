package utils

import (
	_ "net/http/pprof"
	"strings"

	"github.com/google/uuid"
)

func GetUUID() string {
	return uuid.New().String()
}

func SanitizeText(s string) string {
	return strings.TrimSpace(s)
}
