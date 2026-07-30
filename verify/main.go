package main

import (
	"fmt"

	_ "gitee.com/XuguDB/go-xugu-driver"
	_ "github.com/Xugu-Open-Source/xugu-xorm"
	"github.com/go-xorm/xorm"
)

func main() {
	fmt.Println("=== xugu-xorm (go-xorm v0.7.0) plugin verification ===")
	fmt.Println()

	dsn := "IP=192.168.2.216;Port=5138;DB=SYSTEM;User=SYSDBA;PWD=SYSDBA"
	engine, err := xorm.NewEngine("xugu", dsn)
	if err != nil {
		fmt.Printf("[FAIL] NewEngine: %v\n", err)
		return
	}
	defer engine.Close()
	fmt.Println("[OK] 引擎创建成功")
	if err := engine.Ping(); err != nil {
		fmt.Printf("[FAIL] Ping: %v\n", err)
		return
	}
	fmt.Println("[OK] Ping 成功")

	fmt.Println()
	fmt.Println("=== 验证结论 ===")
	fmt.Println("xugu-xorm 已按 go-xorm v0.7.0 注册 Driver/Dialect")
	fmt.Println(`使用方式: import _ "github.com/Xugu-Open-Source/xugu-xorm"`)
}
