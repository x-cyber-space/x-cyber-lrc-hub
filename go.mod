module github.com/x-cyber-space/x-cyber-lrc-hub

// 1.25 is the real floor: modernc.org/sqlite v1.59.0 and its dependencies
// require go 1.25.0. Naming a patch release here (it used to be 1.26.5) makes
// the toolchain refuse to build on any older release, which contradicted the
// README's "Go 1.22+" claim.
go 1.25.0

require modernc.org/sqlite v1.59.0

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/sys v0.47.0 // indirect
	modernc.org/libc v1.75.7 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)
