package cli

// Version is the version of nlsh.
const Version = "0.3.0"

// Usage is the text shown by --help.
const Usage = `nlsh ` + Version + ` — say what you want; it runs the commands.

Usage
  nlsh [options] <request in plain words>      quote wildcards: "clean *.tmp"
  nlsh                                         interactive mode (keeps context)

Options
  -y, --yes      approve automatically (destructive commands still ask)
  -f, --force    never ask, even for destructive commands (careful!)
  -n, --dry-run  show the plan, never execute it
  -p, --print    print commands only, no prompts (exit 2 if validation fails)
  --timeout D    give up on a command after D (30s, 5m); 0 means no limit
  --zsh-init     print the zsh integration script (buffer insertion, Ctrl+O)
  -v, --version  print version
  -h, --help     show this help

Model
  The plan is produced by an OpenAI-compatible server. Point nlsh at one:
    --base-url / NLSH_BASE_URL   e.g. http://localhost:11434/v1
    --api-key  / NLSH_API_KEY (or OPENAI_API_KEY)
    --model    / NLSH_MODEL      e.g. gpt-5-mini, qwen2.5-coder:7b
  If the endpoint is a local one and nothing is listening, nlsh starts
  apfel --serve for you, which serves Apple Foundation Models on-device:
    brew install apfel

Shell integration (zsh)
  eval "$(nlsh --zsh-init)"
  then: type a request on your prompt and press Ctrl+O — the generated command
  is placed in your editable buffer; you review, edit, and press Enter.

Examples
  nlsh symlink folder ./Designs to ./dist
  nlsh create a new sha256 token and copy it to the clipboard
  nlsh -n find the three biggest files in this folder
`
