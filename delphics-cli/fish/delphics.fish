# installed to /usr/share/fish/vendor_conf.d/; user config in ~/.config/fish overrides it

# Debian ships bat and fd as batcat and fdfind
fish_add_path -g /usr/lib/delphics/bin
set -gx EDITOR fresh
set -g fish_greeting
test -e ~/.config/starship.toml; or set -gx STARSHIP_CONFIG /usr/share/delphics/starship.toml

status is-interactive; or return

starship init fish | source
zoxide init fish --cmd cd | source

alias cat bat
alias ls eza
alias find fd
alias du dust
alias df duf
alias ps procs
alias dig doggo
alias grep 'grep --color=auto'

alias wanip 'curl -s ifconfig.me'
alias localip "ip -j -4 route get 1.1.1.1 | jq -r '.[0].prefsrc'"
alias ports 'lsof -i -P -n | grep LISTEN'
alias flush 'resolvectl flush-caches'
alias path 'printf "%s\n" $PATH'
alias now "date '+%Y-%m-%d %H:%M:%S'"
alias reload 'exec fish'
alias hosts 'sudoedit /etc/hosts'

abbr -a -- .. 'cd ..'
abbr -a -- cd.. 'cd ..'
abbr -a -- ... 'cd ../..'
abbr -a -- .... 'cd ../../..'
abbr -a -- ..... 'cd ../../../..'

abbr -a la 'eza -la'
abbr -a lt 'eza -l --sort=modified'
abbr -a lS 'eza -l --sort=size'
abbr -a h history
abbr -a hs 'history | grep'
abbr -a c clear
abbr -a q exit

abbr -a g git
abbr -a ga 'git add'
abbr -a gaa 'git add --all'
abbr -a gc 'git commit'
abbr -a gcm 'git commit -m'
abbr -a gco 'git checkout'
abbr -a gcb 'git checkout -b'
abbr -a gd 'git diff'
abbr -a gds 'git diff --staged'
abbr -a gl 'git log --oneline -20'
abbr -a gp 'git push'
abbr -a gpl 'git pull'
abbr -a gpf 'git push --force-with-lease'
abbr -a gb 'git branch'
abbr -a gba 'git branch -a'
abbr -a gbd 'git branch -d'
abbr -a gf 'git fetch'
abbr -a grb 'git rebase'
abbr -a stash 'git stash'
abbr -a pop 'git stash pop'

abbr -a bi 'bun install'
abbr -a bs 'bun start'
abbr -a brd 'bun run dev'
abbr -a bx bunx
abbr -a ba 'bun add'
abbr -a bad 'bun add -d'
abbr -a brm 'bun remove'
abbr -a bup 'bun update'
