module git.home.anatofuz.net/anatofuz/homelab-signage

go 1.27.1

require (
	github.com/arran4/golang-ical v0.3.6
	github.com/barefootjs/runtime/bf v0.0.0
	github.com/teambition/rrule-go v1.8.2
	golang.org/x/sync v0.16.0
)

// The BarefootJS Go runtime ships vendored under ./bf-runtime so this
// scaffold runs without depending on a published Go module.
replace github.com/barefootjs/runtime/bf => ./bf-runtime
