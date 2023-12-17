// Package config provides the configuration for the application.
package config

// LauncherId is the launcher id set during the build process using the -ldflags "-X config.LauncherId=..." flag.
var LauncherId string

// Logging is a flag that indicates whether logging is enabled.
var Logging string
