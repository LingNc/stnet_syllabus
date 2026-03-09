package config

import (
    "fmt"
    "strings"
)

// TestAPIKey 测试 API Key 读取
func TestAPIKey() {
    key, err := GetAPIKey("../../config")
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Println("Key found, length:", len(key))
    if strings.HasPrefix(key, "sk-") {
        fmt.Println("Key format valid (starts with sk-)")
        fmt.Println("First 10 chars:", key[:10])
    }
}
