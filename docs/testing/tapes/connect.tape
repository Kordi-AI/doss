Output docs/testing/tapes/connect.gif

Set Shell "bash"
Set Width 960
Set Height 540
Set FontSize 16
Set TypingSpeed 35ms

Type "rm -rf /tmp/doss-vhs-demo; mkdir -p /tmp/doss-vhs-demo/home; export HOME=/tmp/doss-vhs-demo/home; export DOSS_HOME=/tmp/doss-vhs-demo/vault"
Enter
Type "go run ./cmd/doss init --no-connect --git-name 'Test Owner' --git-email owner@example.com"
Enter
Sleep 3s
Type "go run ./cmd/doss connect --file /tmp/doss-vhs-demo/AGENTS.md"
Enter
Sleep 3s
Type "sed -n '/doss:begin/,/doss:end/p' /tmp/doss-vhs-demo/AGENTS.md"
Enter
Sleep 4s
