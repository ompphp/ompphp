//go:build windows

package spl

import "os"

func splFileStatFor(info os.FileInfo) splFileStat { return fallbackSplFileStat(info) }
