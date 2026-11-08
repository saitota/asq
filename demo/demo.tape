# Rendered by `task demo`; all sessions are fake data from demo/sessions.
# The GIF is encoded on the host because ffmpeg's GIF pass runs out of memory in a 2GB Docker VM.
Output dist/demo/demo.mp4

Set Shell bash
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 1280
Set Height 640
Set Padding 20
Set Margin 24
Set MarginFill "#6B50FF"
Set BorderRadius 12
Set WindowBar Colorful
Set Theme "Catppuccin Mocha"
Set TypingSpeed 70ms
Set Framerate 20

Hide
Type "source demo/env.sh"
Enter
Sleep 500ms
Show

Sleep 800ms
Type "asq ls"
Sleep 500ms
Enter
Sleep 3.5s
Hide
Ctrl+L
Show

Type "asq"
Sleep 500ms
Enter
Sleep 2s
Down@700ms 3
Sleep 1s
Up@250ms 3
Sleep 600ms
Type@150ms "retry"
Sleep 1.8s
Type@150ms " ci"
Sleep 2.5s
Enter
Sleep 2.5s
Type@80ms "Run it with -race too, then push."
Sleep 3s
