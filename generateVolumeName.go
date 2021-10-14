package main

import (
	"fmt"
	"strings"
)

func generateVolumeName(prefix, id string) string {
	return fmt.Sprintf("%s%s", prefix, strings.ReplaceAll(id, "-", "_"))
}
