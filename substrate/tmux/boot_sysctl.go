package tmux

func darwinBootIdentity(read func(string) (string, error)) (string, error) {
	// kern.boottime is a wall-clock timeval that can change during one boot.
	// The boot session UUID remains an exact identity across clock adjustments.
	return read("kern.bootsessionuuid")
}
