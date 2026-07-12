# cpulse README demo
#
# Regenerate with:
#   brew install vhs
#   make build
#   docker compose -f testdata/demo-broken/docker-compose.yml up -d
#   (wait ~15s for states to settle)
#   PATH="$(pwd)/bin:$PATH" vhs docs/demo.tape
#   docker compose -f testdata/demo-broken/docker-compose.yml down -v
#
# Note: all `Set` directives must come before `Env` / Type / Sleep, or VHS
# ignores them and falls back to 1200×600.
#
# Beat sheet (~45s): launch → scan → filter failed/waiting → api deps →
# jump to postgres → doctor → timeline → graph → quit.

Output "docs/demo.gif"

Require "cpulse"

Set Shell "bash"
Set FontSize 14
Set Width 1600
Set Height 1100
Set Padding 16
Set Theme "Dracula"
Set TypingSpeed 50ms

# Force truecolor so lipgloss hex styles render (VHS/ttyd often looks
# colorless to termenv without these — local terminals usually advertise them).
Env COLORTERM "truecolor"
Env TERM "xterm-256color"
Env CLICOLOR_FORCE "1"

Type "cpulse"
Enter
Sleep 2.5s

# Scan the stuck tree (project → crasher → migrate → api)
Down
Sleep 550ms
Down
Sleep 550ms
Down
Sleep 900ms

# Filter: failed (crasher + postgres)
Type "f"
Sleep 2s

# Filter: waiting (api + worker), land on api
Type "f"
Sleep 900ms
Up
Up
Up
Sleep 250ms
Down
Sleep 1.4s

# Back to full tree, keep api selected
Escape
Sleep 900ms

# Deps: waits-on postgres, blocks worker
Type "3"
Sleep 2.2s

# Jump from deps → unhealthy postgres
Tab
Sleep 350ms
Down
Sleep 800ms
Enter
Sleep 1.6s

# Doctor names the root cause
Type "d"
Sleep 3.2s

# Timeline Gantt
Type "t"
Sleep 2.4s

# Graph pstree + edge conditions
Type "3"
Sleep 2.4s

Type "q"
Sleep 500ms
