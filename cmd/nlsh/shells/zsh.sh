# nlsh zsh integration — add to ~/.zshrc:  eval "$(nlsh --zsh-init)"

# nls <request>  → plan + confirm + run
nls() { nlsh "$@"; }

# nli <request>  → place the generated command(s) on your prompt line,
#                  review/edit freely, press Enter to run (nothing auto-runs)
nli() {
  local out
  if ! out=$(nlsh --print -- "$@"); then
    print -u2 "nlsh: could not produce a valid command"
    return 1
  fi
  print -z -- "$out"
}

# Ctrl+O — type a request directly on your prompt, press Ctrl+O,
# and the generated command replaces the buffer, ready to review and edit
_nlsh_suggest() {
  local req="$BUFFER" out
  if [[ -z "$req" ]]; then
    zle -M "nlsh: type a request first, then press Ctrl+O"
    return
  fi
  if ! out=$(nlsh --print -- "$req"); then
    zle -M "nlsh: could not produce a valid command"
    return
  fi
  BUFFER="$out"
  CURSOR=$#BUFFER
  zle -M "nlsh: review the command, edit freely, then press Enter"
}
zle -N _nlsh_suggest
bindkey '^O' _nlsh_suggest
