// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

<<<<<<< HEAD:vendor/golang.org/x/sys/unix/asm_bsd_arm.s
//go:build (freebsd || netbsd || openbsd) && gc
// +build freebsd netbsd openbsd
=======
//go:build (darwin || freebsd || netbsd || openbsd) && gc
// +build darwin freebsd netbsd openbsd
>>>>>>> f9a33a2 (build(deps): bump github.com/aws/aws-sdk-go from 1.34.34 to 1.40.39):vendor/golang.org/x/sys/unix/asm_netbsd_arm.s
// +build gc

#include "textflag.h"

// System call support for ARM BSD

// Just jump to package syscall's implementation for all these functions.
// The runtime may know about them.

TEXT	·Syscall(SB),NOSPLIT,$0-28
	B	syscall·Syscall(SB)

TEXT	·Syscall6(SB),NOSPLIT,$0-40
	B	syscall·Syscall6(SB)

TEXT	·Syscall9(SB),NOSPLIT,$0-52
	B	syscall·Syscall9(SB)

TEXT	·RawSyscall(SB),NOSPLIT,$0-28
	B	syscall·RawSyscall(SB)

TEXT	·RawSyscall6(SB),NOSPLIT,$0-40
	B	syscall·RawSyscall6(SB)
