package platform

// Darwin describes macOS: BSD userland, zsh, Homebrew under /opt/homebrew.
var Darwin Profile = &unixProfile{
	name:  "darwin",
	shell: []string{"/bin/zsh", "-c"},
	coreutils: "- Target macOS with BSD core utilities; zsh is the shell. " +
		"Assume Homebrew is at /opt/homebrew.\n" +
		"- Prefer BSD-flavoured invocations (sed -i '', stat -f, date -v) over GNU-only forms.",
	destructive: unixDestructivePatterns(),
}
