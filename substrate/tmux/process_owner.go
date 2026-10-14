package tmux

// Owner is one process's parent, owner, and command name as the kernel
// reports them. Command is the kernel's short process name, kept as its raw
// bytes; a process can set it, so it identifies a process only together with
// an owner the process cannot set.
type Owner struct {
	PID, ParentPID int
	// UID is the effective user ID and RealUID the real one.
	UID, RealUID uint32
	Command      string
}
