package platform

// Linux describes a generic Linux host: GNU userland and a POSIX shell.
//
// The dialect is deliberately generic rather than distribution-specific; the
// validator catches the commands a particular distribution does not have.
var Linux Profile = &unixProfile{
	name:  "linux",
	shell: []string{"/bin/sh", "-c"},
	coreutils: "- Target Linux with GNU core utilities; the shell is sh.\n" +
		"- Prefer GNU-flavoured invocations (sed -i, stat -c, date -d) and avoid macOS-only flags.",
	destructive: unixDestructivePatterns(),
}
