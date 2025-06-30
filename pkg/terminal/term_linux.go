//go:build aix || linux || solaris || zos
package terminal

import "golang.org/x/sys/unix"

const ioReadTermios = unix.TCGETS
const ioWriteTermios = unix.TCSETS
