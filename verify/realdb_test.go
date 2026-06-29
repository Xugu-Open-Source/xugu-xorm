package main

import (
	"bytes"
	"context"
	"testing"
	"time"

	_ "gitee.com/XuguDB/go-xugu-driver"
	_ "github.com/Xugu-Open-Source/xugu-xorm"
	"xorm.io/xorm"
	"xorm.io/xorm/names"
)

// 虚谷 Go 驱动的 DSN key 是大小写敏感的：
// IP / Port / DB / User / PWD（不是小写的 ip/port/db/user/pwd）
const (
	testDSN     = "IP=127.0.0.1;Port=5138;DB=SYSTEM;User=SYSDBA;PWD=SYSDBA"
	testTable   = "xugu_xorm_integration_test"
	testTable2  = "xugu_xorm_test_composite"
)

// TestUser 测试用结构体
type TestUser struct {
	Id        int64     `xorm:"pk autoincr"`
	Name      string    `xorm:"varchar(100) notnull"`
	Age       int       `xorm:"int"`
	Email     string    `xorm:"varchar(255)"`
	Score     float64   `xorm:"decimal(10,2)"`
	CreatedAt time.Time `xorm:"created"`
}

// TestOrder 复合主键测试
type TestOrder struct {
	OrderId  int    `xorm:"pk"`
	ItemId   int    `xorm:"pk"`
	Quantity int    `xorm:"int notnull default 0"`
	Remark   string `xorm:"varchar(500)"`
}

// newEngine 创建已连接的 xorm Engine
func newEngine(t *testing.T) *xorm.Engine {
	t.Helper()
	engine, err := xorm.NewEngine("xugu", testDSN)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	engine.SetMapper(names.GonicMapper{})
	if err := engine.Ping(); err != nil {
		engine.Close()
		t.Fatalf("Ping failed: %v", err)
	}
	return engine
}

// ============================================================
// 1. 基础连接验证
// ============================================================

func TestRealConnection(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()
	t.Log("✅ 虚谷数据库连接成功")
}

// ============================================================
// 2. 方言注册验证
// ============================================================

func TestDialectRegisteredInRealContext(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	name := engine.Dialect().URI().DBType
	if name != "xugusql" {
		t.Errorf("期望 DBType=xugusql, 实际=%s", name)
	}
	t.Logf("✅ 方言注册成功: DBType=%s", name)
}

// ============================================================
// 3. DBMetas — 综合验证 GetTables + GetColumns + GetIndexes
// ============================================================

func TestDBMetas_GetTables(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	// 确保至少有 1 张用户表可供内省（空实例无用户表）
	_ = engine.DropTables(&TestUser{})
	if err := engine.Sync2(new(TestUser)); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	defer engine.DropTables(&TestUser{})

	tables, err := engine.DBMetas()
	if err != nil {
		t.Fatalf("DBMetas failed: %v", err)
	}
	if len(tables) == 0 {
		t.Fatal("DBMetas 返回 0 张表——GetTables 可能未正确实现")
	}
	t.Logf("✅ 发现 %d 张表", len(tables))

	// 至少 SYSTEM schema 有表
	found := false
	for _, tb := range tables {
		if tb.Name != "" {
			found = true
			t.Logf("   - %s (columns=%d)", tb.Name, len(tb.Columns()))
			break
		}
	}
	if !found {
		t.Error("所有表名为空——GetTables 结果异常")
	}
}

func TestDBMetas_GetColumns(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	// 建一张测试表以确保有列可供内省
	_ = engine.DropTables(&TestUser{})
	if err := engine.Sync2(new(TestUser)); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	defer engine.DropTables(&TestUser{})

	tables, err := engine.DBMetas()
	if err != nil {
		t.Fatalf("DBMetas failed: %v", err)
	}

	// 随便选一张有列的表验证 GetColumns
	var target *TestUser // just for reflection
	_ = target

	for _, tb := range tables {
		cols := tb.Columns()
		if len(cols) > 0 {
			t.Logf("✅ 表 %s 有 %d 列", tb.Name, len(cols))
			for _, col := range cols {
				t.Logf("   - %s (type=%s, nullable=%v, pk=%v)", col.Name, col.SQLType.Name, col.Nullable, col.IsPrimaryKey)
			}
			return
		}
	}
	t.Error("所有表都没有列——GetColumns 可能未正确实现")
}

func TestDBMetas_GetIndexes(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	tables, err := engine.DBMetas()
	if err != nil {
		t.Fatalf("DBMetas failed: %v", err)
	}

	for _, tb := range tables {
		indexes := tb.Indexes
		// 系统表不一定有索引，但这个调用至少不 panic / 不返回 error
		t.Logf("表 %s: %d 个索引", tb.Name, len(indexes))
		if len(indexes) > 0 {
			for _, idx := range indexes {
				t.Logf("   - %s (cols=%v, type=%d)", idx.Name, idx.Cols, idx.Type)
			}
			return
		}
	}
	t.Log("✅ GetIndexes 调用成功（系统表可能无用户索引）")
}

// ============================================================
// 4. Sync2 — DDL 生成与执行
// ============================================================

func TestSync2_CreateTable(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	// 先确保表不存在
	_ = engine.DropTables(&TestUser{})

	err := engine.Sync2(new(TestUser))
	if err != nil {
		t.Fatalf("Sync2 建表失败: %v", err)
	}
	t.Log("✅ Sync2 建表成功")

	// 验证表存在
	has, err := engine.IsTableExist(&TestUser{})
	if err != nil {
		t.Fatalf("IsTableExist failed: %v", err)
	}
	if !has {
		t.Fatal("IsTableExist 返回 false——表未成功创建")
	}
	t.Log("✅ IsTableExist 确认表存在")

	// 清理
	if err := engine.DropTables(&TestUser{}); err != nil {
		t.Errorf("清理失败: %v", err)
	}
}

func TestSync2_CompositePK(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	_ = engine.DropTables(&TestOrder{})

	err := engine.Sync2(new(TestOrder))
	if err != nil {
		t.Fatalf("Sync2 复合主键建表失败: %v", err)
	}
	t.Log("✅ 复合主键建表成功")

	if err := engine.DropTables(&TestOrder{}); err != nil {
		t.Errorf("清理失败: %v", err)
	}
}

// ============================================================
// 5. CRUD 全流程
// ============================================================

func TestCRUD_FullFlow(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	_ = engine.DropTables(&TestUser{})
	if err := engine.Sync2(new(TestUser)); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	defer engine.DropTables(&TestUser{})

	// === Insert ===
	user := TestUser{
		Name:  "张三",
		Age:   28,
		Email: "zhangsan@test.com",
		Score: 95.50,
	}
	affected, err := engine.Insert(&user)
	if err != nil {
		t.Fatalf("Insert 失败: %v", err)
	}
	if affected != 1 {
		t.Errorf("期望 affected=1, 实际=%d", affected)
	}
	if user.Id == 0 {
		t.Error("Insert 后 Id 仍为 0——自增主键未回填")
	}
	t.Logf("✅ Insert 成功: id=%d, affected=%d", user.Id, affected)

	// === Count ===
	count, err := engine.Count(&TestUser{})
	if err != nil {
		t.Fatalf("Count 失败: %v", err)
	}
	if count != 1 {
		t.Errorf("期望 count=1, 实际=%d", count)
	}
	t.Logf("✅ Count=%d", count)

	// === Get / Find ===
	var found TestUser
	has, err := engine.ID(user.Id).Get(&found)
	if err != nil {
		t.Fatalf("Get(id) 失败: %v", err)
	}
	if !has {
		t.Fatal("Get(id) 未找到已插入的记录")
	}
	if found.Name != "张三" || found.Age != 28 {
		t.Errorf("数据不一致: name=%s, age=%d", found.Name, found.Age)
	}
	t.Logf("✅ Get(id) 成功: name=%s, age=%d", found.Name, found.Age)

	// === Find (多条件) ===
	var users []TestUser
	err = engine.Where("age > ?", 20).Find(&users)
	if err != nil {
		t.Fatalf("Find 失败: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("期望 Find 返回 1 条, 实际=%d", len(users))
	}
	t.Logf("✅ Find 成功: %d 条", len(users))

	// === Update ===
	user.Age = 29
	user.Email = "zhangsan_new@test.com"
	affected, err = engine.ID(user.Id).Update(&user)
	if err != nil {
		t.Fatalf("Update 失败: %v", err)
	}
	if affected != 1 {
		t.Errorf("期望 Update affected=1, 实际=%d", affected)
	}

	// 验证更新
	var updated TestUser
	_, _ = engine.ID(user.Id).Get(&updated)
	if updated.Age != 29 || updated.Email != "zhangsan_new@test.com" {
		t.Errorf("Update 未生效: age=%d, email=%s", updated.Age, updated.Email)
	}
	t.Logf("✅ Update 成功: age=%d, email=%s", updated.Age, updated.Email)

	// === Delete ===
	affected, err = engine.ID(user.Id).Delete(&TestUser{})
	if err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if affected != 1 {
		t.Errorf("期望 Delete affected=1, 实际=%d", affected)
	}

	// 验证删除
	count, _ = engine.Count(&TestUser{})
	if count != 0 {
		t.Errorf("Delete 后仍剩余 %d 条", count)
	}
	t.Log("✅ Delete 成功 & 验证已清空")
}

// ============================================================
// 6. Transaction 事务
// ============================================================

func TestTransaction(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	_ = engine.DropTables(&TestUser{})
	if err := engine.Sync2(new(TestUser)); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	defer engine.DropTables(&TestUser{})

	session := engine.NewSession()
	defer session.Close()

	err := session.Begin()
	if err != nil {
		t.Fatalf("Begin 失败: %v", err)
	}

	_, err = session.Insert(&TestUser{Name: "事务测试", Age: 25})
	if err != nil {
		session.Rollback()
		t.Fatalf("事务 Insert 失败: %v", err)
	}

	err = session.Rollback()
	if err != nil {
		t.Fatalf("Rollback 失败: %v", err)
	}

	// Rollback 后不应有数据
	count, err := engine.Count(&TestUser{})
	if err != nil {
		t.Fatalf("Count 失败: %v", err)
	}
	if count != 0 {
		t.Errorf("事务 Rollback 后仍有 %d 条记录", count)
	}
	t.Log("✅ 事务 Rollback 验证通过")
}

// ============================================================
// 7. 原生 SQL
// ============================================================

func TestRawSQL(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	// 执行一个无害的查询
	results, err := engine.QueryString("SELECT 1 AS one FROM DUAL")
	if err != nil {
		t.Fatalf("QueryString 失败: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("QueryString 返回空结果")
	}
	if results[0]["ONE"] != "1" {
		t.Errorf("期望 ONE=1, 实际=%s", results[0]["ONE"])
	}
	t.Log("✅ 原生 SQL 执行成功")
}

// ============================================================
// 8. DumpTables — Schema 导出
// ============================================================

func TestDumpTables(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	_ = engine.DropTables(&TestUser{})
	if err := engine.Sync2(new(TestUser)); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	defer engine.DropTables(&TestUser{})

	// 插入测试数据
	_, _ = engine.Insert(&TestUser{Name: "dump_test", Age: 30, Email: "dump@test.com"})

	// DumpAll (只验证不 panic)
	var buf bytes.Buffer
	err := engine.DumpAll(&buf)
	if err != nil {
		t.Fatalf("DumpAll 失败: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("DumpAll 返回空")
	}
	t.Logf("✅ DumpAll 成功: %d bytes", buf.Len())
}

// ============================================================
// 9. 边界情况
// ============================================================

func TestEmptyTableOperations(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	_ = engine.DropTables(&TestUser{})
	if err := engine.Sync2(new(TestUser)); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	defer engine.DropTables(&TestUser{})

	// 空表 Count
	count, err := engine.Count(&TestUser{})
	if err != nil {
		t.Fatalf("空表 Count 失败: %v", err)
	}
	if count != 0 {
		t.Errorf("空表 Count 期望 0, 实际=%d", count)
	}

	// 空表 Find
	var users []TestUser
	err = engine.Find(&users)
	if err != nil {
		t.Fatalf("空表 Find 失败: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("空表 Find 期望 0 条, 实际=%d", len(users))
	}

	// 空表 Get (不存在的 id)
	var u TestUser
	has, err := engine.ID(99999).Get(&u)
	if err != nil {
		t.Fatalf("Get 不存在的记录失败: %v", err)
	}
	if has {
		t.Error("Get 不存在的 id 应返回 false")
	}
	t.Log("✅ 空表操作全部正确")
}

func TestInsertMany(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	_ = engine.DropTables(&TestUser{})
	if err := engine.Sync2(new(TestUser)); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	defer engine.DropTables(&TestUser{})

	users := make([]TestUser, 10)
	for i := 0; i < 10; i++ {
		users[i] = TestUser{
			Name:  "批量" + string(rune('A'+i)),
			Age:   20 + i,
			Email: "batch@test.com",
			Score: float64(i) * 10.5,
		}
	}

	// 使用 context
	ctx := context.Background()
	affected, err := engine.Context(ctx).Insert(&users)
	if err != nil {
		t.Fatalf("批量 Insert 失败: %v", err)
	}
	if affected != 10 {
		t.Errorf("批量 Insert 期望 affected=10, 实际=%d", affected)
	}
	t.Logf("✅ 批量 Insert 成功: %d 条", affected)

	count, _ := engine.Count(&TestUser{})
	if count != 10 {
		t.Errorf("Count 期望 10, 实际=%d", count)
	}
}
