package standard

import "os"

type fileStat struct {
	dev, ino, mode, nlink, uid, gid, rdev int64
	size, atime, mtime, ctime             int64
	blksize, blocks                       int64
}

type resourceUsage struct {
	userSec, userUsec, systemSec, systemUsec int64
	maxRSS, ixRSS, idRSS, isRSS              int64
	minorFaults, majorFaults, swaps          int64
	inputBlocks, outputBlocks                int64
	messagesSent, messagesReceived, signals  int64
	voluntarySwitches, involuntarySwitches   int64
}

func fallbackFileStat(info os.FileInfo) fileStat {
	size := info.Size()
	modified := info.ModTime().Unix()
	return fileStat{
		mode: int64(info.Mode()), size: size, atime: modified, mtime: modified,
		ctime: modified, blksize: 4096, blocks: (size + 511) / 512,
	}
}
