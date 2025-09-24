package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"golang.org/x/sys/unix"
)

const (
	// Extra mount option to pass Nydus specific information from snapshotter to runtime through containerd.
	extraOptionKey = "extraoption="
	// Kata virtual volume infmation passed from snapshotter to runtime through containerd, superset of `extraOptionKey`.
	// Please refer to `KataVirtualVolume` in https://github.com/kata-containers/kata-containers/blob/main/src/libs/kata-types/src/mount.rs
	kataVolumeOptionKey = "io.katacontainers.volume="
)

var (
	Version   = "v0.1"
	BuildTime = "unknown"
)

/*
containerd run fuse.mount format: nydus-overlayfs overlay /tmp/ctd-volume107067851
-o lowerdir=/foo/lower2:/foo/lower1,upperdir=/foo/upper,workdir=/foo/work,extraoption={...},dev,suid]
*/
type mountArgs struct {
	fsType  string
	target  string
	options []string
}

func parseArgs(args []string) (*mountArgs, error) {
	margs := &mountArgs{
		fsType: args[0],
		target: args[1],
	}
	if margs.fsType != "overlay" {
		return nil, errors.Errorf("invalid filesystem type %s for overlayfs", margs.fsType)
	}
	if len(margs.target) == 0 {
		return nil, errors.New("empty overlayfs mount target")
	}

	if args[2] == "-o" && len(args[3]) != 0 {
		for _, opt := range strings.Split(args[3], ",") {
			// filter Nydus specific options
			if strings.HasPrefix(opt, extraOptionKey) || strings.HasPrefix(opt, kataVolumeOptionKey) {
				continue
			}
			margs.options = append(margs.options, opt)
		}
	}
	if len(margs.options) == 0 {
		return nil, errors.New("empty overlayfs mount options")
	}

	return margs, nil
}

func parseOptions(options []string) (int, string) {
	flagsTable := map[string]int{
		"async":         0,
		"atime":         0,
		"bind":          unix.MS_BIND,
		"defaults":      0,
		"dev":           0,
		"diratime":      0,
		"dirsync":       unix.MS_DIRSYNC,
		"exec":          0,
		"mand":          unix.MS_MANDLOCK,
		"noatime":       unix.MS_NOATIME,
		"nodev":         unix.MS_NODEV,
		"nodiratime":    unix.MS_NODIRATIME,
		"noexec":        unix.MS_NOEXEC,
		"nomand":        0,
		"norelatime":    0,
		"nostrictatime": 0,
		"nosuid":        unix.MS_NOSUID,
		"rbind":         unix.MS_BIND | unix.MS_REC,
		"relatime":      unix.MS_RELATIME,
		"remount":       unix.MS_REMOUNT,
		"ro":            unix.MS_RDONLY,
		"rw":            0,
		"strictatime":   unix.MS_STRICTATIME,
		"suid":          0,
		"sync":          unix.MS_SYNCHRONOUS,
	}

	var (
		flags int
		data  []string
	)
	for _, o := range options {
		if f, exist := flagsTable[o]; exist {
			flags |= f
		} else {
			data = append(data, o)
		}
	}
	return flags, strings.Join(data, ",")
}

func run(args cli.Args) error {
	margs, err := parseArgs(args.Slice())
	if err != nil {
		return errors.Wrap(err, "parse mount options")
	}

	flags, data := parseOptions(margs.options)
	err = syscall.Mount(margs.fsType, margs.target, margs.fsType, uintptr(flags), data)
	if err != nil {
		return errors.Wrapf(err, "mount overlayfs by syscall")
	}
	return nil
}

func main() {
	app := &cli.App{
		Name:      "NydusOverlayfs",
		Usage:     "FUSE mount helper for containerd to filter out Nydus specific options",
		Version:   fmt.Sprintf("%s.%s", Version, BuildTime),
		UsageText: "[Usage]: nydus-overlayfs overlay <target> -o <options>",
		Action: func(c *cli.Context) error {
			return run(c.Args())
		},
		Before: func(c *cli.Context) error {
			if c.NArg() != 4 {
				cli.ShowAppHelpAndExit(c, 1)
			}
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
}
