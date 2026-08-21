package main

import (
	"fmt"
	"os"

	_ "gitee.com/XuguDB/go-xugu-driver"
	_ "github.com/Xugu-Open-Source/xugu-xorm" // blank import triggers init() → registers xugu dialect & driver
	"xorm.io/xorm"
	"xorm.io/xorm/dialects"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "[FAIL] %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("=== xugu-xorm standalone plugin verification ===")
	fmt.Println()

	// Step 1: Verify dialect registration without requiring a database.
	if dialects.QueryDriver("xugu") == nil || dialects.QueryDialect("xugusql") == nil {
		return fmt.Errorf("xugu driver or xugusql dialect is not registered")
	}
	fmt.Println("[OK] xugu driver and xugusql dialect are registered")

	dsn := os.Getenv("XUGU_TEST_DSN")
	if dsn == "" {
		if os.Getenv("XUGU_IT") == "1" {
			return fmt.Errorf("XUGU_TEST_DSN is required when XUGU_IT=1")
		}
		fmt.Println("[SKIP] Set XUGU_IT=1 and XUGU_TEST_DSN to verify a real database connection")
		return nil
	}

	engine, err := xorm.NewEngine("xugu", dsn)
	if err != nil {
		return fmt.Errorf("引擎创建失败: %w", err)
	}
	defer engine.Close()
	if err := engine.Ping(); err != nil {
		return fmt.Errorf("Ping 失败: %w", err)
	}
	fmt.Println("REAL_DB_CONNECTED")
	fmt.Println("[OK] 引擎创建和 Ping 成功")

	fmt.Println()
	fmt.Println("=== 验证结论 ===")
	fmt.Println("xugu-xorm 作为独立插件注册到 xorm")
	fmt.Println("使用方式: import _ \"github.com/Xugu-Open-Source/xugu-xorm\"")
	fmt.Println("无需手动拷贝文件到 xorm/dialects/")
	return nil
}
