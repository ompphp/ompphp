package spl

import "os"

type splFileStat struct {
	dev, ino, mode, nlink, uid, gid, rdev, blksize, blocks int64
}

func fallbackSplFileStat(info os.FileInfo) splFileStat {
	return splFileStat{mode: int64(info.Mode())}
}
