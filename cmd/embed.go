package main

import (
	"embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

// EmbedFS 嵌入的默认配置文件
//
//go:embed all:config
var EmbedFS embed.FS

// InitConfig 初始化配置目录
// 将嵌入的默认配置文件释放到指定目录
func InitConfig(targetDir string, force bool) error {
	configDir := filepath.Join(targetDir, "config")

	// 创建配置目录
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	// 需要释放的文件列表
	files := []string{
		"config.yaml",
		"二维表.prompt",
		"README.md",
		"api.key",
	}

	for _, fileName := range files {
		targetPath := filepath.Join(configDir, fileName)

		// 检查文件是否已存在
		if _, err := os.Stat(targetPath); err == nil && !force {
			fmt.Printf("  文件已存在，跳过: %s\n", fileName)
			continue
		}

		// 读取嵌入的文件内容
		data, err := EmbedFS.ReadFile(path.Join("config", fileName))
		if err != nil {
			return fmt.Errorf("读取嵌入文件 %s 失败: %w", fileName, err)
		}

		// 写入目标文件
		if err := os.WriteFile(targetPath, data, 0644); err != nil {
			return fmt.Errorf("写入文件 %s 失败: %w", targetPath, err)
		}

		if force {
			fmt.Printf("  已覆盖: %s\n", fileName)
		} else {
			fmt.Printf("  已创建: %s\n", fileName)
		}
	}

	fmt.Printf("\n配置初始化完成！目录: %s\n", configDir)
	fmt.Println("请根据需要修改 config.yaml 和 api.key 文件")

	return nil
}
