package main

import (
	"fmt"
	"net"
	"syscall"
)

// SCM_RIGHTS-based fd receiver for Android. ProcessBuilder fork+exec
// closes all fds ≥3 in the child even when FD_CLOEXEC has been cleared
// on the parent side, so the TUN fd from VpnService can't ride through
// exec inheritance. Instead, the parent connects to this abstract
// socket after spawn and sends the fd via ancillary data.

// tunFDSocketAddr is the abstract-namespace UNIX socket parvazd binds
// to receive the TUN fd. The leading NUL maps to Linux abstract
// namespace (no filesystem path, no cleanup needed).
const tunFDSocketAddr = "\x00parvaz/tun-fd"

// recvTunFD listens on tunFDSocketAddr for one connection, reads one
// dummy byte, and returns the fd carried as SCM_RIGHTS ancillary data.
// Caller owns the fd. Blocks until the parent connects + sends.
func recvTunFD() (int, error) {
	laddr, err := net.ResolveUnixAddr("unix", tunFDSocketAddr)
	if err != nil {
		return -1, fmt.Errorf("resolve: %w", err)
	}
	ln, err := net.ListenUnix("unix", laddr)
	if err != nil {
		return -1, fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()

	conn, err := ln.AcceptUnix()
	if err != nil {
		return -1, fmt.Errorf("accept: %w", err)
	}
	defer conn.Close()

	// Drop into raw fd to use Recvmsg directly — Go's net package
	// doesn't expose ancillary-data reads on UnixConn.
	f, err := conn.File()
	if err != nil {
		return -1, fmt.Errorf("conn.File: %w", err)
	}
	defer f.Close()

	oob := make([]byte, syscall.CmsgSpace(4)) // single fd = 4 bytes
	buf := make([]byte, 1)
	n, oobN, _, _, err := syscall.Recvmsg(int(f.Fd()), buf, oob, 0)
	if err != nil {
		return -1, fmt.Errorf("recvmsg: %w", err)
	}
	if n < 1 {
		return -1, fmt.Errorf("no payload byte")
	}
	if oobN == 0 {
		return -1, fmt.Errorf("no ancillary data")
	}
	cmsgs, err := syscall.ParseSocketControlMessage(oob[:oobN])
	if err != nil {
		return -1, fmt.Errorf("parse cmsg: %w", err)
	}
	if len(cmsgs) == 0 {
		return -1, fmt.Errorf("empty cmsg list")
	}
	fds, err := syscall.ParseUnixRights(&cmsgs[0])
	if err != nil {
		return -1, fmt.Errorf("parse unix rights: %w", err)
	}
	if len(fds) == 0 {
		return -1, fmt.Errorf("no fds in ancillary")
	}
	return fds[0], nil
}
