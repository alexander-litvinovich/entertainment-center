//go:build !windows

package main

import "fmt"

func main() { fmt.Println("hdmi-cec host service is available on Windows only") }
