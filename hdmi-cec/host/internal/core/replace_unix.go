//go:build !windows

package core

import "os"

func replaceFile(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }
