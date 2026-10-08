#!/usr/bin/env bash
# Regenerates docs/screenshot.png and docs/demo.gif by driving the real app
# inside a detached tmux session.
#
# Needs: go, tmux, python3, node with the `playwright` package (and its
# Chromium, or CHROMIUM=/path/to/chrome) and ffmpeg for the GIF.
# Usage: scripts/screenshot/run.sh   (or: make screenshot)
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
work=$(mktemp -d)
demo=/tmp/super-shell-demo # fixed so paths in the image don't change
session=super-shell-demo
trap 'tmux kill-session -t $session 2>/dev/null || true; rm -rf "$work"' EXIT

for tool in go tmux python3 node ffmpeg; do
	command -v $tool >/dev/null || { echo "falta $tool" >&2; exit 1; }
done

go build -o "$work/super-shell" "$repo"

# A clean home with a short prompt, holding a clone of the repo to show.
rm -rf "$demo" && mkdir -p "$demo"
git clone -q "$repo" "$demo/super-shell-tui"
cat >"$demo/.bashrc" <<'EOF'
PS1='\[\e[1;32m\]dev\[\e[0m\] \[\e[1;34m\]\w\[\e[0m\] $ '
alias ls='ls --color=auto'
EOF

# start launches the app in a cols x rows terminal with a fresh state.
start() {
	tmux kill-session -t $session 2>/dev/null || true
	rm -rf "$demo/.config" "$demo/.state"
	tmux new-session -d -s $session -x "$1" -y "$2" \
		"cd $demo/super-shell-tui && env HOME=$demo USER=dev SHELL=/bin/bash \
		XDG_CONFIG_HOME=$demo/.config XDG_STATE_HOME=$demo/.state \
		TERM=xterm-256color COLORTERM=truecolor \
		GOCACHE=$(go env GOCACHE) GOMODCACHE=$(go env GOMODCACHE) GOPATH=$(go env GOPATH) \
		$work/super-shell -shell /bin/bash"
	sleep 2
}
type_() { tmux send-keys -t $session -l "$1"; sleep 0.4; }
enter() { tmux send-keys -t $session Enter; sleep "${1:-0.6}"; }
prefix() { tmux send-keys -t $session C-]; sleep 0.2; tmux send-keys -t $session -l "$1"; sleep 0.8; }
snap() { tmux capture-pane -e -p -t $session >"$work/$1.ans"; }

# --- Screenshot: two tabs, three panes, one of them named.
start 200 46
prefix r; type_ 'código'; enter
type_ 'git log --oneline --no-decorate --color -24'; enter
prefix '|'
prefix R; type_ 'tests'; enter
type_ 'go test -count=1 ./internal/...'; enter 30
prefix '-'
type_ 'python3 -m http.server 8080'; enter
prefix c; prefix r; type_ 'servidor'; enter
prefix 1; prefix o; prefix o
snap shot

# --- GIF: split, name, zoom, one frame per step.
start 140 34
frames=()
step() { snap "f${#frames[@]}"; frames+=("f${#frames[@]}:$1"); }
type_ 'ls'; enter; step 1.2
prefix '|'; step 1.2
prefix R; type_ 'tests'; step 1.5
enter; type_ 'go test ./internal/...'; enter 20; step 1.5
prefix '-'; type_ 'python3 -m http.server 8080'; enter 2; step 1.5
prefix z; step 1.5
prefix z; step 1.2

# --- Render.
pages=()
for name in shot "${frames[@]%%:*}"; do
	python3 -I "$here/ans2html.py" "$work/$name.ans" >"$work/$name.html"
done
node "$here/render.js" 2 "$work/shot.html" "$repo/docs/screenshot.png"
for f in "${frames[@]}"; do
	pages+=("$work/${f%%:*}.html" "$work/${f%%:*}.png")
done
node "$here/render.js" 1 "${pages[@]}"

: >"$work/list.txt"
for f in "${frames[@]}"; do
	printf "file '%s'\nduration %s\n" "$work/${f%%:*}.png" "${f##*:}" >>"$work/list.txt"
done
printf "file '%s'\n" "$work/${frames[-1]%%:*}.png" >>"$work/list.txt" # concat needs the last frame twice
ffmpeg -loglevel error -y -f concat -safe 0 -i "$work/list.txt" \
	-vf "split[a][b];[a]palettegen=max_colors=96[p];[b][p]paletteuse=dither=none" \
	"$repo/docs/demo.gif"
rm -rf "$demo"
echo "docs/screenshot.png y docs/demo.gif actualizados"
