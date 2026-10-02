#!/usr/bin/env bash
# Diagnostic watchdog for the flake hunt. Every few seconds it looks for a tmux
# client running run-shell that has lived past a ceiling, dumps the process
# table with signal masks, the server's kernel view and a server log, then
# kills that client so the test reports instead of waiting for the package
# timeout. Runs only on the hunt branch; never lands.
set -u
out="$1"
mkdir -p "$out"
ceiling=15
seen="$out/.seen"
: > "$seen"
bounded() { perl -e 'alarm shift; exec @ARGV or die "exec: $!"' "$@"; }
seconds() { # ps etime [[dd-]hh:]mm:ss -> seconds
  awk -v t="$1" 'BEGIN{d=0; if (index(t,"-")) {split(t,a,"-"); d=a[1]; t=a[2]} n=split(t,p,":"); s=0; for(i=1;i<=n;i++) s=s*60+p[i]; print d*86400+s}'
}
dump() {
  local pid="$1" args="$2" file socket server
  file="$out/hang-$(date +%H%M%S)-$pid.txt"
  {
    echo "== trigger pid=$pid args=$args"
    date -u; uname -a; tmux -V
    echo "== ps"
    ps ax -o pid,ppid,pgid,stat,etime,wchan,blocked,pending,command
    socket="$(printf '%s\n' "$args" | awk '{for(i=1;i<NF;i++) if ($i=="-S") {print $(i+1); exit}}')"
    echo "== socket $socket"
    server="$(bounded 5 tmux -S "$socket" display-message -p '#{pid}' 2>&1)"
    echo "== server answer: $server"
    case "$server" in ''|*[!0-9]*) server="";; esac
    for p in $(ps ax -o pid=,command= | awk '$2 ~ /(^|\/)tmux:?$/ {print $1}'); do
      echo "== tmux pid $p"
      if [ -r "/proc/$p/status" ]; then
        grep -E '^(State|PPid|Sig|Shd)' "/proc/$p/status"
        sudo -n cat "/proc/$p/stack" 2>&1 | head -20
        sudo -n cat "/proc/$p/syscall" 2>&1
        ls -l "/proc/$p/fd" 2>&1 | tail -n +2
      fi
    done
    if [ -n "$server" ]; then
      echo "== server $server children"
      ps ax -o pid,ppid,stat,etime,wchan,blocked,pending,command | awk -v s="$server" 'NR==1 || $2==s'
      if [ "$(uname -s)" = Darwin ]; then
        sudo -n sample "$server" 1 -file "$out/sample-$server.txt" >/dev/null 2>&1 && echo "sample: $out/sample-$server.txt"
        cwd="$(lsof -a -p "$server" -d cwd -Fn 2>/dev/null | sed -n 's/^n//p')"
      else
        if command -v strace >/dev/null; then
          sudo -n timeout 4 strace -tt -f -p "$server" -o "$out/strace-$server.txt" &
          sleep 1
        fi
        cwd="$(readlink "/proc/$server/cwd")"
      fi
      kill -USR2 "$server"
      bounded 3 tmux -S "$socket" display-message -p probe-after-log-toggle 2>&1
      sleep 3
      kill -USR2 "$server"
      wait
      echo "== server log in $cwd"
      mv "$cwd/tmux-server-$server.log" "$out/" && echo "log: $out/tmux-server-$server.log"
      ps ax -o pid,ppid,stat,etime,wchan,blocked,pending,command | awk -v s="$server" 'NR==1 || $2==s'
    fi
  } > "$file" 2>&1
  # Kill only the hung client, re-checked as the same run-shell client.
  if ps -o command= -p "$pid" | grep -q '^[^ ]*tmux .* run-shell '; then
    kill -KILL "$pid"
    echo "killed client $pid" >> "$file"
  fi
}
while :; do
  ps ax -o pid=,etime=,command= | while read -r pid etime binary args; do
    case "$binary" in tmux|*/tmux) ;; *) continue ;; esac
    case " $args " in *" run-shell "*) ;; *) continue ;; esac
    args="$binary $args"
    grep -qx "$pid" "$seen" && continue
    [ "$(seconds "$etime")" -ge "$ceiling" ] || continue
    echo "$pid" >> "$seen"
    dump "$pid" "$args"
  done
  sleep 3
done
