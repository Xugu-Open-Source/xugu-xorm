package xugu_test

import (
	"strings"
	"testing"

	_ "github.com/Xugu-Open-Source/xugu-xorm"
	"github.com/go-xorm/core"
	"github.com/stretchr/testify/assert"
)

func newXuguDialect(t *testing.T) core.Dialect {
	t.Helper()
	d := core.QueryDialect("xugusql")
	assert.NotNil(t, d, "xugusql dialect should be registered")
	return d
}

func newXuguDriver(t *testing.T) core.Driver {
	t.Helper()
	driver := core.QueryDriver("xugu")
	assert.NotNil(t, driver, "xugu driver should be registered")
	return driver
}

func TestRegisterDriverAndDialect(t *testing.T) {
	assert.NotNil(t, newXuguDriver(t))
	assert.NotNil(t, newXuguDialect(t))
}

func TestParseXuguDSN(t *testing.T) {
	tests := []struct {
		in       string
		expected *core.Uri
	}{
		{
			in:       "ip=192.168.1.100;port=5138;db=mydb;user=admin;pwd=secret;char_set=utf8",
			expected: &core.Uri{DbType: "xugusql", Host: "192.168.1.100", Port: "5138", DbName: "mydb", User: "admin", Passwd: "secret", Charset: "utf8"},
		},
		{
			in:       "ip=127.0.0.1;port=5138;db=test",
			expected: &core.Uri{DbType: "xugusql", Host: "127.0.0.1", Port: "5138", DbName: "test"},
		},
		{
			in:       "IP=10.0.0.1;PORT=5138;DB=prod;USER=root;PWD=pass;CHAR_SET=gbk",
			expected: &core.Uri{DbType: "xugusql", Host: "10.0.0.1", Port: "5138", DbName: "prod", User: "root", Passwd: "pass", Charset: "gbk"},
		},
		{
			in:       "ip= 172.16.0.1 ; port= 5138 ; db= mydb ",
			expected: &core.Uri{DbType: "xugusql", Host: "172.16.0.1", Port: "5138", DbName: "mydb"},
		},
		{
			in:       "",
			expected: &core.Uri{DbType: "xugusql"},
		},
		{
			in:       "ip=localhost;port=5138;db='mydb'",
			expected: &core.Uri{DbType: "xugusql", Host: "localhost", Port: "5138", DbName: "mydb"},
		},
	}

	driver := newXuguDriver(t)
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			uri, err := driver.Parse("xugu", tt.in)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected.DbType, uri.DbType)
			assert.Equal(t, tt.expected.Host, uri.Host)
			assert.Equal(t, tt.expected.Port, uri.Port)
			assert.Equal(t, tt.expected.DbName, uri.DbName)
			assert.Equal(t, tt.expected.User, uri.User)
			assert.Equal(t, tt.expected.Passwd, uri.Passwd)
			assert.Equal(t, tt.expected.Charset, uri.Charset)
		})
	}
}

func TestSqlType(t *testing.T) {
	d := newXuguDialect(t)

	col := &core.Column{SQLType: core.SQLType{Name: core.Bool}}
	assert.Equal(t, "TINYINT(1)", d.SqlType(col))

	col = &core.Column{SQLType: core.SQLType{Name: core.Serial}}
	assert.Equal(t, "INT", d.SqlType(col))
	assert.True(t, col.IsAutoIncrement)
	assert.True(t, col.IsPrimaryKey)

	col = &core.Column{SQLType: core.SQLType{Name: core.Uuid}}
	assert.Equal(t, "VARCHAR(40)", d.SqlType(col))

	col = &core.Column{SQLType: core.SQLType{Name: core.Json}}
	assert.Equal(t, "TEXT", d.SqlType(col))

	col = &core.Column{SQLType: core.SQLType{Name: "NUMERIC"}, Length: 10, Length2: 2}
	assert.Equal(t, "DECIMAL(10,2)", d.SqlType(col))
}

func TestAutoIncrAndQuote(t *testing.T) {
	d := newXuguDialect(t)
	assert.Equal(t, "IDENTITY", d.AutoIncrStr())
	assert.Equal(t, "`", d.QuoteStr())
	assert.Equal(t, "`name`", d.Quote("name"))
	assert.True(t, d.IsReserved("SELECT"))
	assert.False(t, d.SupportEngine())
	assert.False(t, d.SupportCharset())
}

func TestCreateTableSql(t *testing.T) {
	d := newXuguDialect(t)
	table := core.NewEmptyTable()
	table.Name = "user"
	table.AddColumn(&core.Column{
		Name:            "id",
		SQLType:         core.SQLType{Name: core.Int},
		IsPrimaryKey:    true,
		IsAutoIncrement: true,
		Nullable:        false,
		DefaultIsEmpty:  true,
	})
	table.AddColumn(&core.Column{
		Name:           "name",
		SQLType:        core.SQLType{Name: core.Varchar},
		Length:         64,
		Nullable:       false,
		DefaultIsEmpty: true,
	})
	table.PrimaryKeys = []string{"id"}

	sql := d.CreateTableSql(table, "", "", "")
	assert.True(t, strings.Contains(sql, "CREATE TABLE `user`"))
	assert.True(t, strings.Contains(sql, "IDENTITY"), "got: %s", sql)
	assert.True(t, strings.Contains(sql, "CONSTRAINT PK_user PRIMARY KEY"), "got: %s", sql)
	assert.True(t, strings.Contains(sql, "`name` VARCHAR(64)"), "got: %s", sql)
}

func TestTableAndIndexCheckSql(t *testing.T) {
	d := newXuguDialect(t)
	sql, args := d.TableCheckSql("t1")
	assert.Contains(t, sql, "ALL_TABLES")
	assert.Equal(t, []interface{}{"t1"}, args)

	sql, args = d.IndexCheckSql("t1", "idx1")
	assert.Contains(t, sql, "ALL_INDEXES")
	assert.Equal(t, []interface{}{"t1", "idx1"}, args)
}
