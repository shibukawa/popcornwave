package main

import (
	"os"

	"github.com/shibukawa/popcornweb/internal/pwcli"
)

func main() { os.Exit(pwcli.Main(os.Args[1:], os.Stdout, os.Stderr)) }
