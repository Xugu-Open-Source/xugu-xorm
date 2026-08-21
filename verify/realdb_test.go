package main

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "gitee.com/XuguDB/go-xugu-driver"
	_ "github.com/Xugu-Open-Source/xugu-xorm"
	"xorm.io/xorm"
	"xorm.io/xorm/names"
	"xorm.io/xorm/schemas"
)

var realDBTablePrefix string

func TestMain(m *testing.M) {
	realDBTablePrefix = newRealDBTablePrefix()
	if os.Getenv("XUGU_IT") == "1" {
		required := []string{
			"XUGU_TEST_DSN",
			"XUGU_TEST_SECONDARY_DSN",
			"XUGU_TEST_ORDINARY_DSN",
			"XUGU_TEST_PRIMARY_SCHEMA",
			"XUGU_TEST_SECONDARY_SCHEMA",
		}
		var missing []string
		for _, name := range required {
			if strings.TrimSpace(os.Getenv(name)) == "" {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			fmt.Fprintf(os.Stderr, "REAL_DB_BLOCKED: XUGU_IT=1 requires %s\n", strings.Join(missing, ", "))
			os.Exit(2)
		}
	}
	os.Exit(m.Run())
}

// newRealDBTablePrefix produces a lowercase identifier-safe prefix shared by
// one test process. This keeps concurrent or repeated IT runs from touching
// each other's tables in the same dedicated database or schema.
func newRealDBTablePrefix() string {
	var token [6]byte
	if _, err := cryptorand.Read(token[:]); err == nil {
		return "xgit_" + hex.EncodeToString(token[:])
	}

	return fmt.Sprintf("xgit_%x", time.Now().UnixNano())
}

func realDBTableName(suffix string) string {
	return realDBTablePrefix + "_" + suffix
}

// TestUser 测试用结构体
type TestUser struct {
	Id        int64     `xorm:"pk autoincr"`
	Name      string    `xorm:"varchar(100) notnull"`
	Age       int       `xorm:"int"`
	Email     string    `xorm:"varchar(255) index"`
	Score     float64   `xorm:"decimal(10,2)"`
	CreatedAt time.Time `xorm:"created"`
}

func (TestUser) TableName() string {
	return realDBTableName("users")
}

// TestUserV2 keeps the same table name to exercise Sync2's incremental
// add-column path against a table created from the previous model.
type TestUserV2 struct {
	Id        int64     `xorm:"pk autoincr"`
	Name      string    `xorm:"varchar(100) notnull"`
	Age       int       `xorm:"int"`
	Email     string    `xorm:"varchar(255) index"`
	Score     float64   `xorm:"decimal(10,2)"`
	CreatedAt time.Time `xorm:"created"`
	Nickname  string    `xorm:"varchar(64)"`
}

func (TestUserV2) TableName() string {
	return realDBTableName("users")
}

// TestUserV3 keeps the same table name while widening Name. The real-database
// DDL test proves the alteration through a value that does not fit V1.
type TestUserV3 struct {
	Id        int64     `xorm:"pk autoincr"`
	Name      string    `xorm:"varchar(200) notnull"`
	Age       int       `xorm:"int"`
	Email     string    `xorm:"varchar(255) index"`
	Score     float64   `xorm:"decimal(10,2)"`
	CreatedAt time.Time `xorm:"created"`
}

func (TestUserV3) TableName() string {
	return realDBTableName("users")
}

// TestOrder 复合主键测试
type TestOrder struct {
	OrderId  int    `xorm:"pk"`
	ItemId   int    `xorm:"pk"`
	Quantity int    `xorm:"int notnull default 0"`
	Remark   string `xorm:"varchar(500)"`
}

func (TestOrder) TableName() string {
	return realDBTableName("orders")
}

// TestDefaultValue exercises the xorm default tag and the Xugu metadata
// reader. The insert explicitly omits Status so the server evaluates the DDL
// default instead of receiving Go's zero-value empty string.
type TestDefaultValue struct {
	Id     int64  `xorm:"pk autoincr"`
	Name   string `xorm:"varchar(64) notnull"`
	Status string `xorm:"varchar(16) notnull default 'pending'"`
}

func (TestDefaultValue) TableName() string {
	return realDBTableName("defaults")
}

func testUserTableMeta(t *testing.T, engine *xorm.Engine) *schemas.Table {
	return testTableMeta(t, engine, new(TestUser).TableName())
}

func testTableMeta(t *testing.T, engine *xorm.Engine, want string) *schemas.Table {
	t.Helper()
	tables, err := engine.DBMetas()
	if err != nil {
		t.Fatalf("DBMetas failed: %v", err)
	}

	for _, table := range tables {
		if strings.EqualFold(table.Name, want) {
			return table
		}
	}
	t.Fatalf("DBMetas did not include test table %q", want)
	return nil
}

func TestDefaultValueMetadataAndWriteRead(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	_ = engine.DropTables(&TestDefaultValue{})
	if err := engine.Sync2(new(TestDefaultValue)); err != nil {
		t.Fatalf("create default-value fixture: %v", err)
	}
	defer engine.DropTables(&TestDefaultValue{})

	table := testTableMeta(t, engine, new(TestDefaultValue).TableName())
	var status *schemas.Column
	for _, column := range table.Columns() {
		if strings.EqualFold(column.Name, "status") {
			status = column
			break
		}
	}
	if status == nil {
		t.Fatalf("default-value fixture metadata is missing status column")
	}
	if status.DefaultIsEmpty || strings.Trim(strings.TrimSpace(status.Default), "'") != "pending" {
		t.Fatalf("status default metadata = %q (empty=%v), want pending", status.Default, status.DefaultIsEmpty)
	}

	row := &TestDefaultValue{Name: "default-write"}
	if _, err := engine.Omit("status").Insert(row); err != nil {
		t.Fatalf("insert row omitting default status: %v", err)
	}

	var got TestDefaultValue
	has, err := engine.ID(row.Id).Get(&got)
	if err != nil {
		t.Fatalf("read default-value row: %v", err)
	}
	if !has || got.Status != "pending" {
		t.Fatalf("default-value read-back = %+v (has=%v), want status pending", got, has)
	}
}

// newEngine 创建已连接的 xorm Engine
func newEngine(t *testing.T) *xorm.Engine {
	t.Helper()
	if os.Getenv("XUGU_IT") != "1" {
		t.Skip("set XUGU_IT=1 and XUGU_TEST_DSN to run real Xugu integration tests")
	}

	testDSN := os.Getenv("XUGU_TEST_DSN")
	if testDSN == "" {
		t.Fatal("XUGU_TEST_DSN is required when XUGU_IT=1")
	}

	engine, err := xorm.NewEngine("xugu", testDSN)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	engine.SetMapper(names.GonicMapper{})
	if os.Getenv("XUGU_IT_SQL") == "1" {
		engine.ShowSQL(true)
	}
	if err := engine.Ping(); err != nil {
		engine.Close()
		t.Fatalf("Ping failed: %v", err)
	}
	t.Log("REAL_DB_CONNECTED")
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

	table := testUserTableMeta(t, engine)
	t.Logf("✅ DBMetas found test table %s (columns=%d)", table.Name, len(table.Columns()))
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

	table := testUserTableMeta(t, engine)
	expectedColumns := []string{"id", "name", "age", "email", "score", "created_at"}
	columns := make(map[string]*schemas.Column, len(table.Columns()))
	for _, column := range table.Columns() {
		columns[strings.ToLower(column.Name)] = column
		t.Logf("   - %s (type=%s, nullable=%v, pk=%v)", column.Name, column.SQLType.Name, column.Nullable, column.IsPrimaryKey)
	}
	for _, name := range expectedColumns {
		if columns[name] == nil {
			t.Errorf("test table %s is missing expected column %q; got %v", table.Name, name, table.ColumnsSeq())
		}
	}
	if id := columns["id"]; id != nil && !id.IsPrimaryKey {
		t.Errorf("test table %s column id is not reported as a primary key", table.Name)
	}
	if name := columns["name"]; name != nil && name.Nullable {
		t.Errorf("test table %s column name is reported nullable despite the notnull tag", table.Name)
	}
}

func TestDBMetas_GetIndexes(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	_ = engine.DropTables(&TestUser{})
	if err := engine.Sync2(new(TestUser)); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	defer engine.DropTables(&TestUser{})

	table := testUserTableMeta(t, engine)
	for _, index := range table.Indexes {
		t.Logf("   - %s (cols=%v, type=%d)", index.Name, index.Cols, index.Type)
		for _, column := range index.Cols {
			if strings.EqualFold(column, "email") {
				return
			}
		}
	}
	t.Errorf("test table %s is missing the declared email index", table.Name)
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
	if found.Name != "张三" || found.Age != 28 || found.Score != 95.50 {
		t.Errorf("数据不一致: name=%s, age=%d, score=%v", found.Name, found.Age, found.Score)
	}
	if found.CreatedAt.IsZero() {
		t.Error("Get(id) 读回的 CreatedAt 为空")
	}
	t.Logf("✅ Get(id) 成功: name=%s, age=%d, score=%v, created_at=%s", found.Name, found.Age, found.Score, found.CreatedAt)

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

func TestTransactionCommit(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	_ = engine.DropTables(&TestUser{})
	if err := engine.Sync2(new(TestUser)); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	defer engine.DropTables(&TestUser{})

	session := engine.NewSession()
	defer session.Close()
	if err := session.Begin(); err != nil {
		t.Fatalf("Begin 失败: %v", err)
	}
	if _, err := session.Insert(&TestUser{Name: "提交测试", Age: 26}); err != nil {
		_ = session.Rollback()
		t.Fatalf("事务 Insert 失败: %v", err)
	}
	if err := session.Commit(); err != nil {
		t.Fatalf("Commit 失败: %v", err)
	}

	count, err := engine.Count(&TestUser{})
	if err != nil {
		t.Fatalf("Count 失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("Commit 后期望 1 条记录，实际 %d", count)
	}
}

func TestQueryPaginationAndCount(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	_ = engine.DropTables(&TestUser{})
	if err := engine.Sync2(new(TestUser)); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	defer engine.DropTables(&TestUser{})

	users := []TestUser{
		{Name: "alpha", Age: 20},
		{Name: "beta", Age: 21},
		{Name: "gamma", Age: 22},
		{Name: "delta", Age: 23},
	}
	if affected, err := engine.Insert(&users); err != nil || affected != int64(len(users)) {
		t.Fatalf("批量 Insert: affected=%d err=%v", affected, err)
	}

	count, err := engine.Where("age >= ?", 20).Count(&TestUser{})
	if err != nil {
		t.Fatalf("Count 失败: %v", err)
	}
	if count != int64(len(users)) {
		t.Fatalf("Count 期望 %d，实际 %d", len(users), count)
	}

	var page []TestUser
	if err := engine.Where("age >= ?", 20).Asc("id").Limit(2, 1).Find(&page); err != nil {
		t.Fatalf("分页 Find 失败: %v", err)
	}
	if len(page) != 2 || page[0].Name != "beta" || page[1].Name != "gamma" {
		t.Fatalf("分页结果异常: %+v", page)
	}
}

func TestSync2AddColumn(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	_ = engine.DropTables(&TestUser{})
	if err := engine.Sync2(new(TestUser)); err != nil {
		t.Fatalf("初始建表失败: %v", err)
	}
	defer engine.DropTables(&TestUser{})

	if err := engine.Sync2(new(TestUserV2)); err != nil {
		t.Fatalf("增量 Sync2 加列失败: %v", err)
	}

	user := &TestUserV2{Name: "增量字段", Age: 30, Nickname: "nick"}
	if _, err := engine.Insert(user); err != nil {
		t.Fatalf("写入新增列失败: %v", err)
	}

	var found TestUserV2
	has, err := engine.Where("nickname = ?", "nick").Get(&found)
	if err != nil {
		t.Fatalf("查询新增列失败: %v", err)
	}
	if !has || found.Nickname != "nick" {
		t.Fatalf("新增列读回异常: has=%v user=%+v", has, found)
	}
}

func TestModifyColumn(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	_ = engine.DropTables(&TestUser{})
	if err := engine.Sync2(new(TestUser)); err != nil {
		t.Fatalf("初始建表失败: %v", err)
	}
	defer engine.DropTables(&TestUser{})

	column := &schemas.Column{
		Name:           "name",
		SQLType:        schemas.SQLType{Name: schemas.Varchar},
		Length:         200,
		Nullable:       false,
		DefaultIsEmpty: true,
	}
	modifySQL := engine.Dialect().ModifyColumnSQL(new(TestUser).TableName(), column)
	if _, err := engine.Exec(modifySQL); err != nil {
		t.Fatalf("改列失败: %v", err)
	}

	longName := strings.Repeat("n", 150)
	user := &TestUserV3{Name: longName, Age: 30}
	if _, err := engine.Insert(user); err != nil {
		t.Fatalf("写入加宽列失败: %v", err)
	}

	var found TestUserV3
	has, err := engine.ID(user.Id).Get(&found)
	if err != nil {
		t.Fatalf("读取加宽列失败: %v", err)
	}
	if !has || found.Name != longName {
		t.Fatalf("改列后读回异常: has=%v name length=%d", has, len(found.Name))
	}
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

func TestInvalidSQLReturnsError(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	if _, err := engine.Exec("SELECT 1 FROM"); err == nil {
		t.Fatal("执行无效 SQL 未返回错误")
	}
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
