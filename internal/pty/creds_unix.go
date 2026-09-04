//go:build linux || darwin || freebsd

package pty

import "syscall"

// credential builds a syscall.Credential from optional uid/gid. The caller
// (Spawn) has already verified ditty itself is running as root.
func credential(uid, gid *uint32) *syscall.Credential {
	cred := &syscall.Credential{}
	if uid != nil {
		cred.Uid = *uid
	}
	if gid != nil {
		cred.Gid = *gid
	}
	return cred
}
