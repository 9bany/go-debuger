package main

import (
	"debug/gosym"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

var targetfile string
var line int
var pc uint64
var fn *gosym.Func
var symTable *gosym.Table
var regs unix.PtraceRegs
var ws syscall.WaitStatus
var originalCode []byte
var breakpointSet bool

var interruptCode = []byte{0xCC}

func main() {
	target := os.Args[1]
	symTable = getSymbolTable(target)
	fn = symTable.LookupFunc("main.main")
	targetfile, line, fn = symTable.PCToLine(fn.Entry)
	run(target)
}

func run(target string) {
	var filename string

	cmd := exec.Command(target)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Ptrace: true,
	}

	cmd.Start()
	err := cmd.Wait()
	if err != nil {
		fmt.Printf("Wait returned: %v\n\n", err)
	}

	pid := cmd.Process.Pid
	pgid, _ := syscall.Getpgid(pid)

	must(syscall.PtraceSetOptions(pid, syscall.PTRACE_O_TRACECLONE))

	if inputContinue(pid) {
		must(syscall.PtraceCont(pid, 0))
	} else {
		must(syscall.PtraceSingleStep(pid))
	}

	for {
		wpid, err := syscall.Wait4(-1*pgid, &ws, 0, nil)
		must(err)
		if ws.Exited() {
			if wpid == pid {
				break
			}
		} else {
			// We are only interested in tracing if we're stopped by a trap and
			// if the trap was generated by our breakpoint.
			// Cloning a child process also generates a trap, and we want to ignore that.
			if ws.StopSignal() == syscall.SIGTRAP && ws.TrapCause() != syscall.PTRACE_EVENT_CLONE {
				must(syscall.PtraceGetRegs(wpid, &regs))
				filename, line, fn = symTable.PCToLine(regs.Rip)
				fmt.Printf("Stopped at %s at %d in %s\n", fn.Name, line, filename)
				outputStack(symTable, wpid, regs.Rip, regs.Rsp, regs.Rbp)

				if breakpointSet {
					replaceCode(wpid, pc, originalCode)
					breakpointSet = false
				}

				if inputContinue(wpid) {
					must(syscall.PtraceCont(wpid, 0))
				} else {
					must(syscall.PtraceSingleStep(wpid))
				}
			} else {
				must(syscall.PtraceCont(wpid, 0))
			}
		}
	}
}

func replaceCode(pid int, breakpoint uint64, code []byte) []byte {
	original := make([]byte, len(code))
	syscall.PtracePeekData(pid, uintptr(breakpoint), original)
	syscall.PtracePokeData(pid, uintptr(breakpoint), code)
	return original
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}