module github.com/shibukawa/petitweb-go

go 1.26

require (
	github.com/klauspost/compress v1.19.0
	github.com/mattn/go-sqlite3 v1.14.48
	github.com/shibukawa/httpbind-go v0.0.0
	modernc.org/sqlite v1.54.0
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/sys v0.46.0 // indirect
	modernc.org/libc v1.74.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)

replace github.com/shibukawa/httpbind-go => ../httpbind-go
