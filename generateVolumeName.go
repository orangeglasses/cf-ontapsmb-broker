package main

import "strings"

func generateVolumeName(id string) string {
	return strings.ReplaceAll(id, "-", "_")
}
