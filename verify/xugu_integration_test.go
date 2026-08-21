package main

import (
	"fmt"
	"strings"
	"testing"

	_ "github.com/Xugu-Open-Source/xugu-xorm" // blank import → registers xugu dialect & driver
	"xorm.io/xorm/dialects"
	"xorm.io/xorm/schemas"
)

// ── 辅助函数 ────────────────────────────────────────────────

// getDialect 从注册表获取 xugu dialect 实例（含 Init 初始化 quoter）
func getDialect(t *testing.T) dialects.Dialect {
	d := dialects.QueryDialect("xugusql")
	if d == nil {
		t.Fatal("xugu dialect 未注册 — 请确认 blank import 生效")
	}
	// Init 是必须的：设置 quoter → ColumnString / CreateTableSQL 才能正常生成反引号
	if err := d.Init(&dialects.URI{DBType: "xugusql", DBName: "testdb"}); err != nil {
		t.Fatalf("dialect Init 失败: %v", err)
	}
	return d
}

// getDriver 从注册表获取 xugu driver 实例
func getDriver(t *testing.T) dialects.Driver {
	drv := dialects.QueryDriver("xugu")
	if drv == nil {
		t.Fatal("xugu driver 未注册")
	}
	return drv
}

// typeStr 返回值的类型名（如 *sql.NullString）
func typeStr(v interface{}) string {
	full := fmt.Sprintf("%T", v)
	// full looks like: *database/sql.NullString → we want *sql.NullString
	full = strings.Replace(full, "database/sql.", "sql.", 1)
	return full
}

// ── P0-1 修复验证：Nullable 逻辑 ─────────────────────────────

func TestNullableLogicFix(t *testing.T) {
	d := getDialect(t)

	nullCol := &schemas.Column{
		Name:           "desc",
		SQLType:        schemas.SQLType{Name: schemas.Varchar},
		Length:         255,
		Nullable:       true,
		DefaultIsEmpty: true,
	}
	notNullCol := &schemas.Column{
		Name:           "id",
		SQLType:        schemas.SQLType{Name: schemas.Int},
		Nullable:       false,
		DefaultIsEmpty: true,
	}

	s1, err := dialects.ColumnString(d, nullCol, false, false)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := dialects.ColumnString(d, notNullCol, false, false)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(s1, " NULL") {
		t.Errorf("Nullable=true 列应包含 NULL，实际: %s", s1)
	}
	if strings.Contains(s1, "NOT NULL") {
		t.Errorf("Nullable=true 列不应包含 NOT NULL，实际: %s", s1)
	}
	if !strings.Contains(s2, "NOT NULL") {
		t.Errorf("Nullable=false 列应包含 NOT NULL，实际: %s", s2)
	}

	t.Logf("✅ Nullable=true  → %s", s1)
	t.Logf("✅ Nullable=false → %s", s2)
}

// ── P1-3 修复验证：IndexCheckSQL 参数数量 ──────────────────

func TestIndexCheckSQLParamCount(t *testing.T) {
	d := getDialect(t)

	sql, args := d.IndexCheckSQL("users", "idx_name")

	if len(args) != 2 {
		t.Errorf("IndexCheckSQL 应返回 2 个参数，实际 %d 个: %v", len(args), args)
	}
	if !strings.Contains(strings.ToUpper(sql), "TABLE_NAME") || !strings.Contains(strings.ToUpper(sql), "INDEX_NAME") {
		t.Errorf("SQL 缺少预期的列名: %s", sql)
	}

	t.Logf("✅ IndexCheckSQL args 数量 = %d (期望 2)", len(args))
}

// ── P1-4 修复验证：ColumnTypeKind 含 TIME_TYPE ──────────────

func TestColumnTypeKindTimeType(t *testing.T) {
	d := getDialect(t)

	tests := []struct {
		typeName string
		kind     int
	}{
		{"DATETIME", schemas.TIME_TYPE},
		{"TIMESTAMP", schemas.TIME_TYPE},
		{"DATE", schemas.TIME_TYPE},
		{"TIME", schemas.TIME_TYPE},
		{"varchar", schemas.TEXT_TYPE}, // 小写
		{"INT", schemas.NUMERIC_TYPE},
		{"BLOB", schemas.BLOB_TYPE},
		{"UNKNOWN_XYZ", schemas.UNKNOW_TYPE},
	}

	for _, tt := range tests {
		got := d.ColumnTypeKind(tt.typeName)
		if got != tt.kind {
			t.Errorf("ColumnTypeKind(%q) = %d, 期望 %d", tt.typeName, got, tt.kind)
		}
	}

	t.Log("✅ DATETIME / TIMESTAMP / DATE / TIME 全部归入 TIME_TYPE")
}

// ── P2-7 修复验证：Float 不再标记 unsigned ─────────────────

func TestFloatSQLTypeNotUnsigned(t *testing.T) {
	d := getDialect(t)

	col := &schemas.Column{
		Name:    "price",
		SQLType: schemas.SQLType{Name: schemas.Float},
	}
	sql := d.SQLType(col)

	if strings.Contains(strings.ToUpper(sql), "UNSIGNED") {
		t.Errorf("Float 类型不应包含 UNSIGNED，实际: %s", sql)
	}
	if sql != "FLOAT" {
		t.Errorf("Float SQLType 期望 FLOAT，实际: %s", sql)
	}

	t.Logf("✅ Float SQLType = %s", sql)
}

// ── P2-12 修复验证：保留字 ON / OPTIMIZE 独立 ──────────────

func TestReservedWordsOnOptimize(t *testing.T) {
	d := getDialect(t)

	if !d.IsReserved("ON") {
		t.Error("ON 应是保留字")
	}
	if !d.IsReserved("OPTIMIZE") {
		t.Error("OPTIMIZE 应是保留字")
	}
	// 大小写不敏感
	if !d.IsReserved("on") {
		t.Error("'on'（小写）应为保留字")
	}
	if !d.IsReserved("optimize") {
		t.Error("'optimize'（小写）应为保留字")
	}

	t.Log("✅ ON 和 OPTIMIZE 均为独立保留字条目")
}

// ── DSN 解析 ────────────────────────────────────────────────

func TestDSNParsing(t *testing.T) {
	drv := getDriver(t)

	tests := []struct {
		name    string
		dsn     string
		host    string
		port    string
		db      string
		user    string
		charset string
	}{
		{"完整 DSN", "ip=192.0.2.10;port=5138;db=testdb;user=test_user;pwd=example_password;char_set=utf8", "192.0.2.10", "5138", "testdb", "test_user", "utf8"},
		{"带单引号", "ip='127.0.0.1';port='5138';db='testdb';user='test_user';pwd='example_password'", "127.0.0.1", "5138", "testdb", "test_user", ""},
		{"仅必需", "ip=10.0.0.1;port=5159;db=mydb;user=sa", "10.0.0.1", "5159", "mydb", "sa", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri, err := drv.Parse("xugu", tt.dsn)
			if err != nil {
				t.Fatalf("DSN 解析失败: %v", err)
			}
			if uri.Host != tt.host {
				t.Errorf("Host: got %q, want %q", uri.Host, tt.host)
			}
			if uri.Port != tt.port {
				t.Errorf("Port: got %q, want %q", uri.Port, tt.port)
			}
			if uri.DBName != tt.db {
				t.Errorf("DBName: got %q, want %q", uri.DBName, tt.db)
			}
			if uri.User != tt.user {
				t.Errorf("User: got %q, want %q", uri.User, tt.user)
			}
			if uri.Charset != tt.charset {
				t.Errorf("Charset: got %q, want %q", uri.Charset, tt.charset)
			}
			if uri.DBType != "xugusql" {
				t.Errorf("DBType: got %q, want xugusql", uri.DBType)
			}
		})
	}

	t.Log("✅ 3 种 DSN 格式全部解析正确")
}

// ── SQL 类型映射 ────────────────────────────────────────────

func TestSQLTypeMapping(t *testing.T) {
	d := getDialect(t)

	tests := []struct {
		typeName string
		expect   string
		checkPK  bool
	}{
		{schemas.Bool, "TINYINT(1)", false},
		{schemas.Serial, "INT", true},
		{schemas.BigSerial, "BIGINT", true},
		{schemas.Bytea, "BLOB", false},
		{schemas.NVarchar, "VARCHAR", false},
		{schemas.Uuid, "VARCHAR(40)", false},
		{schemas.Json, "TEXT", false},
	}

	for _, tt := range tests {
		col := &schemas.Column{
			Name:    "test_col",
			SQLType: schemas.SQLType{Name: tt.typeName},
		}
		got := d.SQLType(col)
		if got != tt.expect {
			t.Errorf("SQLType(%s) = %q, 期望 %q", tt.typeName, got, tt.expect)
		}
		if tt.checkPK && !col.IsPrimaryKey {
			t.Errorf("SQLType(%s) 应设置 IsPrimaryKey", tt.typeName)
		}
	}

	t.Log("✅ 7 种核心类型映射通过")
}

// ── CreateTableSQL ──────────────────────────────────────────

func TestCreateTableSQL(t *testing.T) {
	d := getDialect(t)

	// 单主键自增
	t.Run("单主键自增", func(t *testing.T) {
		table := schemas.NewTable("users", nil)
		table.AddColumn(&schemas.Column{
			Name: "id", SQLType: schemas.SQLType{Name: schemas.Int},
			IsPrimaryKey: true, IsAutoIncrement: true,
			Nullable: false, DefaultIsEmpty: true,
		})
		table.AddColumn(&schemas.Column{
			Name: "name", SQLType: schemas.SQLType{Name: schemas.Varchar},
			Length: 100, Nullable: true, DefaultIsEmpty: true,
		})
		table.PrimaryKeys = []string{"id"}

		sql, _, err := d.CreateTableSQL(nil, nil, table, "users")
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range []string{"CREATE TABLE", "`id` INT IDENTITY NOT NULL", "`name` VARCHAR(100) NULL"} {
			if !strings.Contains(sql, s) {
				t.Errorf("SQL 缺少 %q", s)
			}
		}
		t.Logf("   SQL: %s", sql)
	})

	// 复合主键
	t.Run("复合主键", func(t *testing.T) {
		table := schemas.NewTable("order_items", nil)
		table.AddColumn(&schemas.Column{
			Name: "order_id", SQLType: schemas.SQLType{Name: schemas.Int},
			IsPrimaryKey: true, Nullable: false, DefaultIsEmpty: true,
		})
		table.AddColumn(&schemas.Column{
			Name: "item_id", SQLType: schemas.SQLType{Name: schemas.Int},
			IsPrimaryKey: true, Nullable: false, DefaultIsEmpty: true,
		})
		table.PrimaryKeys = []string{"order_id", "item_id"}

		sql, _, err := d.CreateTableSQL(nil, nil, table, "order_items")
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range []string{"CREATE TABLE", "CONSTRAINT PK_", "PRIMARY KEY"} {
			if !strings.Contains(sql, s) {
				t.Errorf("SQL 缺少 %q", s)
			}
		}
		t.Logf("   SQL: %s", sql)
	})

	t.Log("✅ CreateTableSQL 单主键/复合主键通过")
}

// ── Features ─────────────────────────────────────────────────

func TestDialectFeatures(t *testing.T) {
	d := getDialect(t)
	feat := d.Features()
	if feat.AutoincrMode != dialects.IncrAutoincrMode {
		t.Errorf("AutoincrMode 期望 IncrAutoincrMode, got %v", feat.AutoincrMode)
	}
	t.Log("✅ Features: AutoincrMode = IncrAutoincrMode")
}

// ── Driver Features ──────────────────────────────────────────

func TestDriverFeatures(t *testing.T) {
	drv := getDriver(t)
	feat := drv.Features()
	if !feat.SupportReturnInsertedID {
		t.Error("xugu driver 应支持 SupportReturnInsertedID")
	}
	t.Log("✅ Driver Features: SupportReturnInsertedID = true")
}

// ── Quoter ───────────────────────────────────────────────────

func TestQuoterBacktick(t *testing.T) {
	d := getDialect(t)
	q := d.Quoter()
	if q.Prefix != '`' || q.Suffix != '`' {
		t.Errorf("期望反引号 quoter，实际 Prefix=%q Suffix=%q", q.Prefix, q.Suffix)
	}
	t.Log("✅ Quoter 使用反引号")
}

// ── Alias ────────────────────────────────────────────────────

func TestAlias(t *testing.T) {
	d := getDialect(t)
	if a := d.Alias("numeric"); a != "decimal" {
		t.Errorf("Alias(numeric) = %q, 期望 decimal", a)
	}
	if a := d.Alias("varchar"); a != "varchar" {
		t.Errorf("Alias(varchar) 不应变化, got %q", a)
	}
	t.Log("✅ Alias: numeric → decimal")
}

// ── AutoIncrStr ──────────────────────────────────────────────

func TestAutoIncrStr(t *testing.T) {
	d := getDialect(t)
	if s := d.AutoIncrStr(); s != "IDENTITY" {
		t.Errorf("AutoIncrStr = %q, 期望 IDENTITY", s)
	}
	t.Log("✅ AutoIncrStr = IDENTITY")
}

// ── GenScanResult ────────────────────────────────────────────

func TestGenScanResult(t *testing.T) {
	drv := getDriver(t)

	tests := []struct {
		colType string
		expect  string
	}{
		{"VARCHAR", "*sql.NullString"},
		{"BIGINT", "*sql.NullInt64"},
		{"INT", "*sql.NullInt32"},
		{"SMALLINT", "*sql.NullInt32"},
		{"FLOAT", "*sql.NullFloat64"},
		{"DECIMAL", "*sql.NullString"},
		{"DATETIME", "*sql.NullTime"},
		{"BLOB", "*sql.RawBytes"},
	}

	for _, tt := range tests {
		result, err := drv.GenScanResult(tt.colType)
		if err != nil {
			t.Errorf("GenScanResult(%q) 错误: %v", tt.colType, err)
			continue
		}
		got := typeStr(result)
		if got != tt.expect {
			t.Errorf("GenScanResult(%q) = %s, 期望 %s", tt.colType, got, tt.expect)
		}
	}

	t.Log("✅ 8 种类型的 GenScanResult 映射通过")
}

// ── Filters / SetParams ──────────────────────────────────────

func TestFiltersAndSetParams(t *testing.T) {
	d := getDialect(t)
	filters := d.Filters()
	t.Logf("✅ Filters() 返回 %d 个 filter", len(filters))

	// SetParams 不应 panic
	d.SetParams(map[string]string{"rowFormat": "DYNAMIC"})
	d.SetParams(map[string]string{})
	d.SetParams(nil)
	t.Log("✅ SetParams 不 panic")
}

// ── AddColumnSQL / ModifyColumnSQL ───────────────────────────

func TestAddAndModifyColumnSQL(t *testing.T) {
	d := getDialect(t)

	col := &schemas.Column{
		Name:           "email",
		SQLType:        schemas.SQLType{Name: schemas.Varchar},
		Length:         255,
		Nullable:       true,
		DefaultIsEmpty: true,
	}

	addSQL := d.AddColumnSQL("users", col)
	if !strings.Contains(addSQL, "ALTER TABLE") || !strings.Contains(addSQL, "ADD") {
		t.Errorf("AddColumnSQL 异常: %s", addSQL)
	}
	t.Logf("✅ AddColumnSQL: %s", addSQL)

	col2 := &schemas.Column{
		Name:           "email",
		SQLType:        schemas.SQLType{Name: schemas.Varchar},
		Length:         512,
		Nullable:       false,
		DefaultIsEmpty: true,
	}
	modSQL := d.ModifyColumnSQL("users", col2)
	if !strings.Contains(modSQL, "ALTER TABLE") || !strings.Contains(modSQL, "MODIFY COLUMN") {
		t.Errorf("ModifyColumnSQL 异常: %s", modSQL)
	}
	t.Logf("✅ ModifyColumnSQL: %s", modSQL)
}

// ── SetQuotePolicy ───────────────────────────────────────────

func TestSetQuotePolicy(t *testing.T) {
	d := getDialect(t)

	d.SetQuotePolicy(dialects.QuotePolicyNone)
	q := d.Quoter()
	if q.IsReserved == nil || q.IsReserved("SELECT") {
		t.Error("QuotePolicyNone 下 SELECT 不应被保留")
	}

	d.SetQuotePolicy(dialects.QuotePolicyAlways)
	q = d.Quoter()
	if !q.IsReserved("SELECT") {
		t.Error("QuotePolicyAlways 下 SELECT 应为保留")
	}

	t.Log("✅ SetQuotePolicy 切换正常")
}

// ── 方言注册存在性 ───────────────────────────────────────────

func TestDialectAndDriverRegistered(t *testing.T) {
	if dialects.QueryDialect("xugusql") == nil {
		t.Fatal("dialect 'xugusql' 未注册")
	}
	if dialects.QueryDriver("xugu") == nil {
		t.Fatal("driver 'xugu' 未注册")
	}
	t.Log("✅ xugu driver + xugusql dialect 均已注册")
}
