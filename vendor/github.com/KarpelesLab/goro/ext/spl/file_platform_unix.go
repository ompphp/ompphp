//go:build !windows

package spl

import (
	"os"
	"syscall"
)

func splFileStatFor(info os.FileInfo) splFileStat {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat != nil {
		return splFileStat{
			dev: int64(stat.Dev), ino: int64(stat.Ino), mode: int64(stat.Mode),
			nlink: int64(stat.Nlink), uid: int64(stat.Uid), gid: int64(stat.Gid),
			rdev: int64(stat.Rdev), blksize: int64(stat.Blksize), blocks: stat.Blocks,
		}
	}
	result := fallbackSplFileStat(info)
	result.uid = int64(os.Getuid())
	result.gid = int64(os.Getgid())
	return result
}
