# VHS tape for the marketing homepage's first screencast: the terminal-side
# invocation of `ditty top` and its startup banner. Regenerate with:
#   cd <repo root> && vhs docs/scripts/ditty-top.tape
# (needs `bin/ditty` already built — `make build` — and vhs on PATH).
Output docs/public/assets/ditty-top.gif

Set FontSize 18
Set Width 1000
Set Height 480
Set Theme "Dracula"
Set TypingSpeed 60ms
Set Padding 20

Type "./bin/ditty top"
Sleep 500ms
Enter
Sleep 4s
Ctrl+C
Sleep 500ms
