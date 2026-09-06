package main

import "os"

// IsDev reports whether the server is running in development mode.
// Driven by APP_ENV — `production` flips into prod mode; anything
// else (including unset) is dev.
func IsDev() bool {
	return os.Getenv("APP_ENV") != "production"
}
