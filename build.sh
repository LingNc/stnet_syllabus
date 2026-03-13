go mod tidy

# 获取版本号和构建时间（时间格式不含空格）
VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
BUILD_TIME=$(date +"%Y%m%d-%H%M%S")
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

# Linux release
GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o build/stnet_syllabus ./cmd

# Windows release
GOOS=windows GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o build/stnet_syllabus.exe ./cmd

echo "Build complete: ${VERSION}"