#!/usr/bin/env bash
_script()
{
  local cur prev opts
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"

  if [[ ${COMP_CWORD} -eq 1 ]]; then
    opts=$(connect --completions)
    COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
  fi
  return 0
}
complete -F _script connect
