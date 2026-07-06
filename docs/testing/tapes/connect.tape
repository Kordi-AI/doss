Output docs/testing/tapes/connect.gif

Set Shell "bash"
Set Width 960
Set Height 540
Set FontSize 16
Set TypingSpeed 35ms

Type "tmp=$(mktemp -d); export HOME=$tmp/home; export DOSS_HOME=$tmp/vault; mkdir -p \"$HOME\""
Enter
Type "go run ./cmd/doss init --no-connect --git-name \"Test Owner\" --git-email owner@example.com"
Enter
Sleep 3s
Type "go run ./cmd/doss connect --file \"$tmp/AGENTS.md\""
Enter
Sleep 3s
Type "sed -n '/doss:begin/,/doss:end/p' \"$tmp/AGENTS.md\""
Enter
Sleep 4s
