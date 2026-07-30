# xugu-xorm（go-xorm v0.7.0 适配版）

虚谷数据库的 **go-xorm v0.7.0** 方言插件。

> 本分支：`adapt/go-xorm-0.7.0`  
> 版本建议：`v0.7.0-goxorm-xugu`  
> `main` / `v1.3.x-xugu` 面向 `xorm.io/xorm`，与本分支**不通用**。

---

## 依赖

| 组件 | 版本 | 说明 |
|------|------|------|
| github.com/go-xorm/xorm | v0.7.0 | 客户工程引入 |
| github.com/Xugu-Open-Source/xugu-xorm | 本分支 / 本 tag | blank-import 注册方言 |
| gitee.com/XuguDB/go-xugu-driver | 客户侧已有驱动即可 | database/sql 驱动 |
| 虚谷库 | 如 `192.168.x.x:5138` | 真实库地址 |

---

## 接入步骤

### 1. 引入模块

若代码已推送到 GitHub：

```bash
go get github.com/Xugu-Open-Source/xugu-xorm@v0.7.0-goxorm-xugu
```

若暂时用本地/内网仓库，在客户 `go.mod` 中：

```go
require github.com/Xugu-Open-Source/xugu-xorm v0.0.0

replace github.com/Xugu-Open-Source/xugu-xorm => ../xugu-xorm
// 或 => git+ssh://...@adapt/go-xorm-0.7.0
```

同时确保：

```bash
go get github.com/go-xorm/xorm@v0.7.0
go get gitee.com/XuguDB/go-xugu-driver@最新可用版本
```

### 2. 代码示例

```go
package main

import (
	"fmt"
	"log"

	_ "gitee.com/XuguDB/go-xugu-driver"           // 虚谷 database/sql 驱动
	_ "github.com/Xugu-Open-Source/xugu-xorm"    // 注册 xugu 方言（必须 blank import）
	"github.com/go-xorm/core"
	"github.com/go-xorm/xorm"
)

type User struct {
	Id   int64  `xorm:"pk autoincr"`
	Name string `xorm:"varchar(100) notnull"`
	Age  int    `xorm:"int"`
}

func (User) TableName() string { return "demo_user" }

func main() {
	// 注意：虚谷官方驱动的 DSN 键名通常大小写敏感
	dsn := "IP=192.168.2.216;Port=5138;DB=SYSTEM;User=SYSDBA;PWD=SYSDBA"

	engine, err := xorm.NewEngine("xugu", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer engine.Close()

	engine.SetMapper(core.LintGonicMapper)

	if err := engine.Ping(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("ping ok")

	_ = engine.DropTables(new(User))
	if err := engine.Sync2(new(User)); err != nil {
		log.Fatal(err)
	}

	u := &User{Name: "alice", Age: 20}
	if _, err := engine.Insert(u); err != nil {
		log.Fatal(err)
	}
	fmt.Println("inserted id =", u.Id)
}
```

### 3. 要点

1. **驱动名必须是 `"xugu"`**：`xorm.NewEngine("xugu", dsn)`
2. **必须 blank-import** 本包，否则方言未注册会报 `Unsupported driver/dialect`
3. **自增列** 方言使用虚谷 `IDENTITY`
4. **表名** 建议用 `TableName()` 显式指定，避免和默认结构体名不一致

---

## DSN 说明

分号分隔键值对。对接官方驱动时建议使用如下大小写：

| 键 | 含义 | 示例 |
|----|------|------|
| IP | 主机 | 192.168.2.216 |
| Port | 端口 | 5138 |
| DB | 库名 | SYSTEM |
| User | 用户 | SYSDBA |
| PWD | 密码 | SYSDBA |
| CHAR_SET | 字符集（可选） | utf8 |

示例：

```text
IP=192.168.2.216;Port=5138;DB=SYSTEM;User=SYSDBA;PWD=SYSDBA
```

---

## 本地自测（维护者）

```bash
# 单元测试（不连库）
go test ./...

# 活库集成（需改 verify/realdb_test.go 中 DSN，或已指向目标库）
cd verify
go test -tags=realdb -v -count=1 -run TestRealDBSyncAndCRUD .
```

当前已在 `192.168.2.216:5138` 验证：Ping / Sync2 / Insert / Get / Update / Delete 通过。

---

## 目录说明

| 路径 | 说明 |
|------|------|
| xugu.go | 方言与驱动 Parse 实现 |
| xugu_test.go | 注册 / SqlType / 建表 SQL 单测 |
| verify/ | 活库冒烟与集成验证 |

---

## 常见问题

**Q: `Unsupported driver name: xugu`**  
A: 未 blank-import `xugu-xorm`，或 import 了错误版本（xorm.io 分支）。

**Q: 建表报 IDENTITY 相关默认值错误**  
A: 请使用本分支实现；旧逻辑会对空默认值拼 `DEFAULT ''`，虚谷会拒绝。

**Q: 和 `xorm.io/xorm` 能共用吗？**  
A: 不能。`main` 给 xorm.io；本分支只给 go-xorm v0.7.0。
