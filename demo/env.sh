# Sourced by demo.tape: a fake home and stub agent CLIs keep real sessions out of the recording.
demo=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
export HOME=/home/you
export PATH="$demo/bin:$demo/../dist/demo:$PATH"
export PS1='\[\e[38;2;137;180;250m\]\w\[\e[0m\] \[\e[38;2;203;166;247m\]❯\[\e[0m\] '
mkdir -p ~/work/api ~/work/web ~/work/infra ~/work/cli
cd ~/work/api || return
clear
