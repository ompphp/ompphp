//go:build !windows

package standard

import (
	"fmt"
	"log/syslog"
	"os"
	"syscall"
)

func platformSyslog(priority int, message string) error {
	priorities := []syslog.Priority{syslog.LOG_EMERG, syslog.LOG_ALERT, syslog.LOG_CRIT, syslog.LOG_ERR, syslog.LOG_WARNING, syslog.LOG_NOTICE, syslog.LOG_INFO, syslog.LOG_DEBUG}
	if priority < 0 || priority >= len(priorities) {
		priority = len(priorities) - 1
	}
	writer, err := syslog.New(priorities[priority], "")
	if err != nil {
		return err
	}
	defer writer.Close()
	_, err = fmt.Fprint(writer, message)
	return err
}

func platformAccess(path string, mode uint32) error { return syscall.Access(path, mode) }

func platformDevice(info os.FileInfo) (int64, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return 0, false
	}
	return int64(stat.Dev), true
}

func platformUmask(mask int) int { return syscall.Umask(mask) }

func platformDiskSpace(path string) (float64, float64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	return float64(stat.Bavail) * float64(stat.Bsize), float64(stat.Blocks) * float64(stat.Bsize), nil
}

func platformFileStat(info os.FileInfo) fileStat {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat != nil {
		return fileStat{
			dev: int64(stat.Dev), ino: int64(stat.Ino), mode: int64(stat.Mode),
			nlink: int64(stat.Nlink), uid: int64(stat.Uid), gid: int64(stat.Gid),
			rdev: int64(stat.Rdev), size: stat.Size, atime: int64(stat.Atim.Sec),
			mtime: int64(stat.Mtim.Sec), ctime: int64(stat.Ctim.Sec),
			blksize: int64(stat.Blksize), blocks: stat.Blocks,
		}
	}
	return fallbackFileStat(info)
}

func platformResourceUsage(children bool) (resourceUsage, error) {
	who := syscall.RUSAGE_SELF
	if children {
		who = syscall.RUSAGE_CHILDREN
	}
	var usage syscall.Rusage
	if err := syscall.Getrusage(who, &usage); err != nil {
		return resourceUsage{}, err
	}
	return resourceUsage{
		userSec: int64(usage.Utime.Sec), userUsec: int64(usage.Utime.Usec),
		systemSec: int64(usage.Stime.Sec), systemUsec: int64(usage.Stime.Usec),
		maxRSS: int64(usage.Maxrss), ixRSS: int64(usage.Ixrss), idRSS: int64(usage.Idrss), isRSS: int64(usage.Isrss),
		minorFaults: int64(usage.Minflt), majorFaults: int64(usage.Majflt), swaps: int64(usage.Nswap),
		inputBlocks: int64(usage.Inblock), outputBlocks: int64(usage.Oublock),
		messagesSent: int64(usage.Msgsnd), messagesReceived: int64(usage.Msgrcv), signals: int64(usage.Nsignals),
		voluntarySwitches: int64(usage.Nvcsw), involuntarySwitches: int64(usage.Nivcsw),
	}, nil
}

func platformNice(increment int) error {
	priority, err := syscall.Getpriority(syscall.PRIO_PROCESS, 0)
	if err != nil {
		return err
	}
	priority += increment
	if priority > 19 {
		priority = 19
	}
	if priority < -20 {
		priority = -20
	}
	return syscall.Setpriority(syscall.PRIO_PROCESS, 0, priority)
}
