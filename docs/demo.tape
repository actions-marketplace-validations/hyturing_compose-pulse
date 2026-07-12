# cpulse README demo
#
# Regenerate with:
#   brew install vhs
#   docker compose -f testdata/demo-broken/docker-compose.yml up -d
#   (wait ~15s for states to settle)
#   ln -sf "$(pwd)/bin/cpulse" /opt/homebrew/bin/cpulse   # put cpulse on PATH
#   vhs docs/demo.tape
#   docker compose -f testdata/demo-broken/docker-compose.yml down -v

Output "docs/demo.gif"

Set Shell "bash"
Set FontSize 16
Set Width 1280
Set Height 960
Set Padding 20
Set Theme "Dracula"
Set TypingSpeed 60ms

Type "cpulse"
Enter
Sleep 2s

Down
Sleep 700ms
Down
Sleep 1200ms

Type "3"
Sleep 1800ms

Type "2"
Sleep 1500ms

Type "1"
Sleep 1000ms

Type "f"
Sleep 1800ms

Type "f"
Sleep 700ms

Type "b"
Sleep 1800ms

Escape
Sleep 700ms

Type "?"
Sleep 2500ms

Escape
Sleep 700ms

Type "q"
Sleep 500ms
