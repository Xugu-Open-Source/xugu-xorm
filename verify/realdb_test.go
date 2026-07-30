//go:build realdb
// +build realdb

package main

import (
	"testing"
	"time"

	_ "gitee.com/XuguDB/go-xugu-driver"
	_ "github.com/Xugu-Open-Source/xugu-xorm"
	"github.com/go-xorm/core"
	"github.com/go-xorm/xorm"
)

// 虚谷 Go 驱动的 DSN key 通常大小写敏感：IP / Port / DB / User / PWD
const testDSN = "IP=192.168.2.216;Port=5138;DB=SYSTEM;User=SYSDBA;PWD=SYSDBA"

type TestUser struct {
	Id        int64     `xorm:"pk autoincr"`
	Name      string    `xorm:"varchar(100) notnull"`
	Age       int       `xorm:"int"`
	Email     string    `xorm:"varchar(255)"`
	Score     float64   `xorm:"decimal(10,2)"`
	CreatedAt time.Time `xorm:"created"`
}

func (TestUser) TableName() string {
	return "xugu_xorm_integration_test"
}

func newEngine(t *testing.T) *xorm.Engine {
	t.Helper()
	engine, err := xorm.NewEngine("xugu", testDSN)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	engine.ShowSQL(false)
	engine.SetMapper(core.LintGonicMapper)
	if err := engine.Ping(); err != nil {
		engine.Close()
		t.Fatalf("ping failed: %v", err)
	}
	return engine
}

func TestRealDBSyncAndCRUD(t *testing.T) {
	engine := newEngine(t)
	defer engine.Close()

	_ = engine.DropTables(new(TestUser))
	_ = engine.DropTables("test_user") // 清理先前试跑残留

	if err := engine.Sync2(new(TestUser)); err != nil {
		t.Fatalf("Sync2 failed: %v", err)
	}

	user := &TestUser{Name: "alice", Age: 20, Email: "a@example.com", Score: 99.5}
	if _, err := engine.Insert(user); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	if user.Id == 0 {
		t.Fatal("expected autoincr id")
	}

	var got TestUser
	has, err := engine.ID(user.Id).Get(&got)
	if err != nil || !has {
		t.Fatalf("Get failed: has=%v err=%v", has, err)
	}
	if got.Name != "alice" {
		t.Fatalf("unexpected name: %s", got.Name)
	}

	n, err := engine.ID(user.Id).Update(&TestUser{Age: 21})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row updated, got %d", n)
	}

	n, err = engine.ID(user.Id).Delete(new(TestUser))
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row deleted, got %d", n)
	}

	_ = engine.DropTables(new(TestUser))
}
