go mod tidy

# 获取版本号和构建时间
VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
BUILD_TIME=$(date +"%Y-%m-%d %H:%M:%S")
LDFLAGS="-X main.Version=$VERSION -X main.BuildTime=$BUILD_TIME -s -w"

# Linux release
GOOS=linux GOARCH=amd64 go build -ldflags "$LDFLAGS" -o build/stnet_syllabus ./cmd

# Windows release
GOOS=windows GOARCH=amd64 go build -ldflags "$LDFLAGS" -o build/stnet_syllabus.exe ./cmd

echo "Build complete: $VERSION"