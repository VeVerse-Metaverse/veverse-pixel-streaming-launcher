package main

import (
	goRuntime "runtime"
	"strings"
)

// binarySuffixes list of suffixes of known|supported entrypoint binaries
var binarySuffixes = map[string]bool{
	"Debug":     true,
	"DebugGame": true,
	"Test":      true,
	"Shipping":  true,
}

func getBinarySuffix() string {
	env := strings.ToLower(pEnvironment)

	if //goland:noinspection GoBoolExpressions
	goRuntime.GOOS == "windows" {
		if env == "debug" {
			return "DebugGame.exe"
		} else if env == "dev" || env == "development" {
			return ".exe"
		} else if env == "test" {
			return "Test.exe"
		} else if env == "prod" || env == "production" || env == "shipping" {
			return "Shipping.exe"
		}
		return ".exe"
	} else {
		if env == "debug" {
			return "DebugGame"
		} else if env == "dev" || env == "development" {
			return ""
		} else if env == "test" {
			return "Test"
		} else if env == "prod" || env == "production" || env == "shipping" {
			return "Shipping"
		}
		return ""
	}
}
