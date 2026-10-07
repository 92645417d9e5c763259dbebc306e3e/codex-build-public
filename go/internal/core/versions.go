package core

import (
	"strings"
)

func ValidReleaseTag(tag string) bool {
	value, present := strings.CutPrefix(tag, "rust-v")
	if !present || strings.HasSuffix(tag, "-prev") || strings.HasSuffix(tag, ".pending") {
		return false
	}
	if strings.Contains(value, "+") {
		return false
	}
	numeric, prerelease, hasPrerelease := strings.Cut(value, "-")
	numbers := strings.Split(numeric, ".")
	if len(numbers) != 3 {
		return false
	}
	for _, number := range numbers {
		if !releaseNumeric(number) || len(number) > 1 && number[0] == '0' {
			return false
		}
	}
	return !hasPrerelease || releaseIdentifiers(prerelease)
}

func releaseIdentifiers(value string) bool {
	for _, part := range strings.Split(value, ".") {
		if part == "" || releaseNumeric(part) && len(part) > 1 && part[0] == '0' {
			return false
		}
		for _, character := range part {
			if character != '-' && (character < '0' || character > '9') && (character < 'A' || character > 'Z') && (character < 'a' || character > 'z') {
				return false
			}
		}
	}
	return true
}

func releaseNumeric(value string) bool {
	return value != "" && strings.Trim(value, "0123456789") == ""
}
