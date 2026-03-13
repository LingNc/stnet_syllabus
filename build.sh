go mod tidy
# Linux
GOOS=linux GOARCH=amd64 go build -o build/stnet_syllabus ./cmd
# Windows
GOOS=windows GOARCH=amd64 go build -o build/stnet_syllabus.exe ./cmd