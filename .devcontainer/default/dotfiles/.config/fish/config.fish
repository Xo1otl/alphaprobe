alias c="clear"
alias e="exit"
alias phs="python3 -m http.server"
# alias gemini="bunx @google/gemini-cli"

fish_vi_key_bindings

set -g fish_greeting

fish_add_path $HOME/.bun/bin
fish_add_path $HOME/.local/bin
fish_add_path $HOME/.juliaup/bin
fish_add_path /usr/local/cuda-13.0/bin # cuda-toolkitはここにインストールされる

function cd
  builtin cd $argv
  and ls -a
end

mise activate fish | source

# temporary workaround for `https://github.com/google-gemini/gemini-cli/issues/6297`
function gemini
  set -l HOSTS_ENTRY "127.0.0.1 host.docker.internal"
  set -l HOSTS_FILE "/etc/hosts"

  echo $HOSTS_ENTRY | sudo tee -a $HOSTS_FILE > /dev/null
  command bunx @google/gemini-cli $argv
  set -l exit_status $status

  tac $HOSTS_FILE | \
    awk -v entry="$HOSTS_ENTRY" '!f && $0 == entry {f=1; next} 1' | \
    tac | \
    sudo tee $HOSTS_FILE > /dev/null

  return $exit_status
end