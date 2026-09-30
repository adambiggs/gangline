//go:build linux

package tmux

import (
	"context"
	"net"
	"os"
	"strconv"
	"syscall"
)

func nativeProcessNamespace() (string, error) {
	return os.Readlink("/proc/self/ns/pid")
}

func readCallerProcess(pid int) (processRecord, error) { return readCurrentProcess(pid) }

func serverProcessVisible(ctx context.Context, socket string, pid int) (bool, error) {
	var dialer net.Dialer
	connection, err := dialer.DialContext(ctx, "unix", socket)
	if err != nil {
		return false, err
	}
	defer connection.Close()
	raw, err := connection.(*net.UnixConn).SyscallConn()
	if err != nil {
		return false, err
	}
	var credential *syscall.Ucred
	var credentialErr error
	err = raw.Control(func(fd uintptr) {
		credential, credentialErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if err != nil {
		return false, err
	}
	if credentialErr != nil {
		return false, credentialErr
	}
	return peerProcessVisible(pid, int(credential.Pid), samePIDNamespace, readCurrentProcess)
}

func peerProcessVisible(reported, peer int, sameNamespace func(int) bool, read func(int) (processRecord, error)) (bool, error) {
	if peer <= 0 || peer != reported || !sameNamespace(peer) {
		return false, nil
	}
	_, err := read(peer)
	return processReadVisibility(err)
}

func samePIDNamespace(pid int) bool {
	self, err := os.Stat("/proc/self/ns/pid")
	if err != nil {
		return false
	}
	peer, err := os.Stat("/proc/" + strconv.Itoa(pid) + "/ns/pid")
	return err == nil && os.SameFile(self, peer)
}
