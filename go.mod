module git.home.anatofuz.net/anatofuz/homelab-signage

go 1.23.0

require (
	github.com/barefootjs/runtime/bf v0.0.0
	golang.org/x/oauth2 v0.30.0
	golang.org/x/sync v0.16.0
)

require cloud.google.com/go/compute/metadata v0.3.0 // indirect

// The BarefootJS Go runtime ships vendored under ./bf-runtime so this
// scaffold runs without depending on a published Go module.
replace github.com/barefootjs/runtime/bf => ./bf-runtime
