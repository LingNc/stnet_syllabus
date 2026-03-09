package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "strings"
)

func GetAPIKey(configDir string) (string, error) {
    keyPath := filepath.Join(configDir, "api.key")
    data, err := os.ReadFile(keyPath)
    if err != nil {
        return "", err
    }

    lines := strings.Split(string(data), "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        return line, nil
    }
    return "", fmt.Errorf("no key found")
}

type ChatRequest struct {
    Model    string    `json:"model"`
    Messages []Message `json:"messages"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

func main() {
    key, err := GetAPIKey("config")
    if err != nil {
        fmt.Println("Error reading key:", err)
        return
    }

    reqBody := ChatRequest{
        Model: "deepseek-chat",
        Messages: []Message{
            {Role: "user", Content: "Hello"},
        },
    }

    jsonData, _ := json.Marshal(reqBody)
    req, _ := http.NewRequest("POST", "https://api.deepseek.com/chat/completions", bytes.NewBuffer(jsonData))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+key)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        fmt.Println("API Error:", err)
        return
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    fmt.Println("Status:", resp.Status)
    fmt.Println("Response:", string(body)[:200])
}
