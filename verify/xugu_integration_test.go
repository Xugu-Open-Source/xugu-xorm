package main

import (
	"strings"
	"testing"

	_ "github.com/Xugu-Open-Source/xugu-xorm"
	"github.com/go-xorm/core"
)

// 本目录用于 go-xorm v0.7.0 场景下的方言注册与 SQL 片段冒烟测试。
// 真实库联调见 realdb_test.go（需虚谷实例 + database/sql 驱动）。

func getDialect(t *testing.T) core.Dialect {
	t.Helper()
	d := core.QueryDialect("xugusql")
	if d == nil {
		t.Fatal("xugu dialect 未注册 — 请确认 blank import 生效")
	}
	return d
}

func getDriver(t *testing.T) core.Driver {
	t.Helper()
	drv := core.QueryDriver("xugu")
	if drv == nil {
		t.Fatal("xugu driver 未注册")
	}
	return drv
}

func TestDialectRegistered(t *testing.T) {
	getDialect(t)
	getDriver(t)
}

func TestParseDSN(t *testing.T) {
	uri, err := getDriver(t).Parse("xugu", "IP=127.0.0.1;PORT=5138;DB=SYSTEM;USER=SYSDBA;PWD=SYSDBA")
	if err != nil {
		t.Fatal(err)
	}
	if uri.DbType != "xugusql" || uri.Host != "127.0.0.1" || uri.DbName != "SYSTEM" {
		t.Fatalf("unexpected uri: %+v", uri)
	}
}

func TestCreateTableSqlSmoke(t *testing.T) {
	d := getDialect(t)
	table := core.NewEmptyTable()
	table.Name = "demo"
	table.AddColumn(&core.Column{
		Name:            "id",
		SQLType:         core.SQLType{Name: core.Int},
		IsPrimaryKey:    true,
		IsAutoIncrement: true,
		Nullable:        false,
		DefaultIsEmpty:  true,
	})
	table.PrimaryKeys = []string{"id"}

	sql := d.CreateTableSql(table, "", "", "")
	if !strings.Contains(sql, "IDENTITY") || !strings.Contains(sql, "CONSTRAINT PK_demo") {
		t.Fatalf("unexpected sql: %s", sql)
	}
}
