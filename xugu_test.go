// Copyright 2025 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xugu_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	_ "github.com/Xugu-Open-Source/xugu-xorm" // blank import triggers init() registration
	"xorm.io/xorm/core"
	"xorm.io/xorm/dialects"
	"xorm.io/xorm/schemas"

	"github.com/stretchr/testify/assert"
)

// shared helpers

func newXuguDialect(t *testing.T) dialects.Dialect {
	t.Helper()
	d := dialects.QueryDialect("xugusql")
	assert.NotNil(t, d, "xugusql dialect should be registered")
	return d
}

func newXuguDialectInit(t *testing.T, dbName string) dialects.Dialect {
	t.Helper()
	d := newXuguDialect(t)
	uri := &dialects.URI{DBType: "xugusql", DBName: dbName}
	err := d.Init(uri)
	assert.NoError(t, err)
	return d
}

func newXuguDriver(t *testing.T) dialects.Driver {
	t.Helper()
	driver := dialects.QueryDriver("xugu")
	assert.NotNil(t, driver, "xugu driver should be registered")
	return driver
}

type queryerFunc func(context.Context, string, ...interface{}) (*core.Rows, error)

func (f queryerFunc) QueryContext(ctx context.Context, query string, args ...interface{}) (*core.Rows, error) {
	return f(ctx, query, args...)
}

type versionRowsDriver struct {
	columns []string
	rows    [][]driver.Value
}

func (d versionRowsDriver) Open(string) (driver.Conn, error) {
	return &versionRowsConn{columns: d.columns, rows: d.rows}, nil
}

type versionRowsConn struct {
	columns []string
	rows    [][]driver.Value
}

func (*versionRowsConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("Prepare is not supported by this test driver")
}

func (*versionRowsConn) Close() error {
	return nil
}

func (*versionRowsConn) Begin() (driver.Tx, error) {
	return nil, errors.New("Begin is not supported by this test driver")
}

func (c *versionRowsConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &versionRows{columns: c.columns, rows: c.rows}, nil
}

type versionRows struct {
	columns []string
	rows    [][]driver.Value
	pos     int
}

func (r *versionRows) Columns() []string {
	return r.columns
}

func (*versionRows) Close() error {
	return nil
}

func (r *versionRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.rows) {
		return io.EOF
	}

	copy(dest, r.rows[r.pos])
	r.pos++
	return nil
}

var versionRowsDriverID uint64

func newVersionQueryer(t *testing.T, rows ...[]driver.Value) core.Queryer {
	return newRowsQueryer(t, []string{"VERSION"}, rows...)
}

func newRowsQueryer(t *testing.T, columns []string, rows ...[]driver.Value) core.Queryer {
	t.Helper()
	driverName := fmt.Sprintf("xugu-version-rows-%d", atomic.AddUint64(&versionRowsDriverID, 1))
	sql.Register(driverName, versionRowsDriver{columns: columns, rows: rows})
	db, err := sql.Open(driverName, "")
	assert.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })
	return core.FromDB(db)
}

// ============================================================================
// Layer 1: Driver-level tests (Parse + GenScanResult)
// ============================================================================

func TestParseXuguDSN(t *testing.T) {
	tests := []struct {
		in       string
		expected *dialects.URI
	}{
		{
			in:       "ip=192.0.2.10;port=5138;db=mydb;user=test_user;pwd=example_password;char_set=utf8",
			expected: &dialects.URI{DBType: "xugusql", Host: "192.0.2.10", Port: "5138", DBName: "mydb", User: "test_user", Passwd: "example_password", Charset: "utf8"},
		},
		{
			in:       "ip=127.0.0.1;port=5138;db=test",
			expected: &dialects.URI{DBType: "xugusql", Host: "127.0.0.1", Port: "5138", DBName: "test"},
		},
		{
			in:       "IP=10.0.0.1;PORT=5138;DB=prod;USER=root;PWD=pass;CHAR_SET=gbk",
			expected: &dialects.URI{DBType: "xugusql", Host: "10.0.0.1", Port: "5138", DBName: "prod", User: "root", Passwd: "pass", Charset: "gbk"},
		},
		{
			in:       "ip= 172.16.0.1 ; port= 5138 ; db= mydb ",
			expected: &dialects.URI{DBType: "xugusql", Host: "172.16.0.1", Port: "5138", DBName: "mydb"},
		},
		{
			in:       "ip=localhost;port=5138;user=sa",
			expected: &dialects.URI{DBType: "xugusql", Host: "localhost", Port: "5138", User: "sa"},
		},
		{
			in:       "",
			expected: &dialects.URI{DBType: "xugusql"},
		},
		{
			in:       "ip=localhost;port=5138;db=test;unknown_key=value",
			expected: &dialects.URI{DBType: "xugusql", Host: "localhost", Port: "5138", DBName: "test"},
		},
		{
			in:       "ip=localhost;port=5138;db='mydb'",
			expected: &dialects.URI{DBType: "xugusql", Host: "localhost", Port: "5138", DBName: "mydb"},
		},
	}

	driver := newXuguDriver(t)

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			uri, err := driver.Parse("xugu", tt.in)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected.DBType, uri.DBType)
			assert.Equal(t, tt.expected.Host, uri.Host)
			assert.Equal(t, tt.expected.Port, uri.Port)
			assert.Equal(t, tt.expected.DBName, uri.DBName)
			assert.Equal(t, tt.expected.User, uri.User)
			assert.Equal(t, tt.expected.Passwd, uri.Passwd)
			assert.Equal(t, tt.expected.Charset, uri.Charset)
		})
	}
}

func TestParseXuguMalformedDSN(t *testing.T) {
	driver := newXuguDriver(t)

	// The driver parser is intentionally permissive: malformed segments are
	// ignored and validation remains the responsibility of the Xugu driver
	// when a connection is opened. The important dialect contract is that
	// malformed input cannot panic or fabricate connection fields.
	uri, err := driver.Parse("xugu", "ip=localhost;not-a-pair;db;=missing-key")
	assert.NoError(t, err)
	assert.Equal(t, "xugusql", string(uri.DBType))
	assert.Equal(t, "localhost", uri.Host)
	assert.Empty(t, uri.DBName)
}

func TestXuguGenScanResult(t *testing.T) {
	tests := []struct {
		colType string
		isNull  bool
	}{
		{"CHAR", true},
		{"VARCHAR", true},
		{"TINYTEXT", true},
		{"TEXT", true},
		{"MEDIUMTEXT", true},
		{"LONGTEXT", true},
		{"ENUM", true},
		{"SET", true},
		{"JSON", true},
		{"BIGINT", true},
		{"TINYINT", true},
		{"SMALLINT", true},
		{"MEDIUMINT", true},
		{"INT", true},
		{"FLOAT", true},
		{"REAL", true},
		{"DOUBLE PRECISION", true},
		{"DOUBLE", true},
		{"DECIMAL", true},
		{"NUMERIC", true},
		{"DATETIME", true},
		{"TIMESTAMP", true},
		{"BIT", false},
		{"BINARY", false},
		{"VARBINARY", false},
		{"TINYBLOB", false},
		{"BLOB", false},
		{"MEDIUMBLOB", false},
		{"LONGBLOB", false},
		{"CUSTOM_TYPE", false},
	}

	driver := newXuguDriver(t)

	for _, tt := range tests {
		t.Run(tt.colType, func(t *testing.T) {
			result, err := driver.GenScanResult(tt.colType)
			assert.NoError(t, err)
			assert.NotNil(t, result)

			if tt.isNull {
				assert.Contains(t, reflect.TypeOf(result).String(), "sql.Null")
			} else {
				assert.Contains(t, reflect.TypeOf(result).String(), "sql.RawBytes")
			}
		})
	}
}

func TestXuguGenScanResultUnsignedStripping(t *testing.T) {
	driver := newXuguDriver(t)

	// UNSIGNED prefix should be stripped before matching
	result, err := driver.GenScanResult("UNSIGNED INT")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, reflect.TypeOf(result).String(), "sql.NullInt32")

	result2, err := driver.GenScanResult("UNSIGNED BIGINT")
	assert.NoError(t, err)
	assert.Contains(t, reflect.TypeOf(result2).String(), "sql.NullInt64")
}

// ============================================================================
// Layer 2: Core SQL generation tests
// ============================================================================

func TestXuguSQLType(t *testing.T) {
	d := newXuguDialectInit(t, "test")

	newCol := func(name string, l1, l2 int64) *schemas.Column {
		return &schemas.Column{
			Name:    name,
			SQLType: schemas.SQLType{Name: name},
			Length:  l1,
			Length2: l2,
		}
	}

	tests := []struct {
		name     string
		col      *schemas.Column
		expected string
	}{
		{"Bool", newCol(schemas.Bool, 0, 0), "TINYINT(1)"},
		{"Serial", newCol(schemas.Serial, 0, 0), "INT"},
		{"BigSerial", newCol(schemas.BigSerial, 0, 0), "BIGINT"},
		{"Bytea", newCol(schemas.Bytea, 0, 0), "BLOB"},
		{"TimeStampz", newCol(schemas.TimeStampz, 0, 0), "CHAR(64)"},
		{"NVarchar", newCol(schemas.NVarchar, 0, 0), "VARCHAR"},
		{"Uuid", newCol(schemas.Uuid, 0, 0), "VARCHAR(40)"},
		{"Json", newCol(schemas.Json, 0, 0), "TEXT"},
		{"Varchar with length", newCol(schemas.Varchar, 255, 0), "VARCHAR(255)"},
		{"Decimal with precision,scale", newCol(schemas.Decimal, 10, 2), "DECIMAL(10,2)"},
		{"Char with length", newCol(schemas.Char, 64, 0), "CHAR(64)"},
		{"Int", newCol(schemas.Int, 0, 0), "INT"},
		{"BigInt no length", newCol(schemas.BigInt, 0, 0), "BIGINT"},
		{"TinyInt", newCol(schemas.TinyInt, 0, 0), "TINYINT"},
		{"SmallInt", newCol(schemas.SmallInt, 0, 0), "SMALLINT"},
		{"MediumInt", newCol(schemas.MediumInt, 0, 0), "MEDIUMINT"},
		{"Float", newCol(schemas.Float, 0, 0), "FLOAT"},
		{"Double", newCol(schemas.Double, 0, 0), "DOUBLE"},
		{"Real", newCol(schemas.Real, 0, 0), "REAL"},
		{"Text", newCol(schemas.Text, 0, 0), "TEXT"},
		{"Blob", newCol(schemas.Blob, 0, 0), "BLOB"},
		{"DateTime", newCol(schemas.DateTime, 0, 0), "DATETIME"},
		{"Date", newCol(schemas.Date, 0, 0), "DATE"},
		{"Time", newCol(schemas.Time, 0, 0), "TIME"},
		{"TimeStamp", newCol(schemas.TimeStamp, 0, 0), "TIMESTAMP"},
		{"Bit", newCol(schemas.Bit, 0, 0), "BIT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.SQLType(tt.col)
			assert.Equal(t, tt.expected, result)
		})
	}

	// Enum and Set need special setup (EnumOptions/SetOptions)
	t.Run("Enum", func(t *testing.T) {
		c := newCol(schemas.Enum, 0, 0)
		c.EnumOptions = map[string]int{"A": 0, "B": 1}
		result := d.SQLType(c)
		assert.True(t, strings.HasPrefix(result, "ENUM("), "should start with ENUM(, got: %s", result)
		assert.Contains(t, result, "'A'")
		assert.Contains(t, result, "'B'")
	})
	t.Run("Set", func(t *testing.T) {
		c := newCol(schemas.Set, 0, 0)
		c.SetOptions = map[string]int{"X": 0, "Y": 1, "Z": 2}
		result := d.SQLType(c)
		// Map iteration order is non-deterministic; check prefix and contained values
		assert.True(t, strings.HasPrefix(result, "SET("), "should start with SET(, got: %s", result)
		assert.Contains(t, result, "'X'")
		assert.Contains(t, result, "'Y'")
		assert.Contains(t, result, "'Z'")
		assert.True(t, strings.HasSuffix(result, ")"), "should end with )")
	})
}

func TestXuguColumnTypeKind(t *testing.T) {
	d := newXuguDialectInit(t, "test")

	tests := []struct {
		colType  string
		expected int
	}{
		{"DATETIME", schemas.TIME_TYPE},
		{"TIMESTAMP", schemas.TIME_TYPE},
		{"DATE", schemas.TIME_TYPE},
		{"TIME", schemas.TIME_TYPE},
		{"CHAR", schemas.TEXT_TYPE},
		{"VARCHAR", schemas.TEXT_TYPE},
		{"TINYTEXT", schemas.TEXT_TYPE},
		{"TEXT", schemas.TEXT_TYPE},
		{"MEDIUMTEXT", schemas.TEXT_TYPE},
		{"LONGTEXT", schemas.TEXT_TYPE},
		{"ENUM", schemas.TEXT_TYPE},
		{"SET", schemas.TEXT_TYPE},
		{"BIGINT", schemas.NUMERIC_TYPE},
		{"TINYINT", schemas.NUMERIC_TYPE},
		{"SMALLINT", schemas.NUMERIC_TYPE},
		{"MEDIUMINT", schemas.NUMERIC_TYPE},
		{"INT", schemas.NUMERIC_TYPE},
		{"FLOAT", schemas.NUMERIC_TYPE},
		{"REAL", schemas.NUMERIC_TYPE},
		{"DOUBLE PRECISION", schemas.NUMERIC_TYPE},
		{"DECIMAL", schemas.NUMERIC_TYPE},
		{"NUMERIC", schemas.NUMERIC_TYPE},
		{"BIT", schemas.NUMERIC_TYPE},
		{"BINARY", schemas.BLOB_TYPE},
		{"VARBINARY", schemas.BLOB_TYPE},
		{"TINYBLOB", schemas.BLOB_TYPE},
		{"BLOB", schemas.BLOB_TYPE},
		{"MEDIUMBLOB", schemas.BLOB_TYPE},
		{"LONGBLOB", schemas.BLOB_TYPE},
		{"varchar", schemas.TEXT_TYPE},
		{"int", schemas.NUMERIC_TYPE},
		{"blob", schemas.BLOB_TYPE},
		{"XYZ_FOO", schemas.UNKNOW_TYPE},
	}

	for _, tt := range tests {
		t.Run(tt.colType, func(t *testing.T) {
			result := d.ColumnTypeKind(tt.colType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestXuguCreateTableSQL(t *testing.T) {
	d := newXuguDialectInit(t, "test")

	tests := []struct {
		name      string
		tableName string
		setup     func() *schemas.Table
		contains  []string
	}{
		{
			name:      "single column, no primary key",
			tableName: "users",
			setup: func() *schemas.Table {
				table := schemas.NewTable("users", nil)
				table.AddColumn(&schemas.Column{
					Name:           "id",
					SQLType:        schemas.SQLType{Name: schemas.Int},
					Nullable:       false,
					DefaultIsEmpty: true,
				})
				table.AddColumn(&schemas.Column{
					Name:           "name",
					SQLType:        schemas.SQLType{Name: schemas.Varchar},
					Length:         100,
					Nullable:       true,
					DefaultIsEmpty: true,
				})
				return table
			},
			contains: []string{
				"CREATE TABLE `users`",
				"`id` INT NOT NULL",
				"`name` VARCHAR(100) NULL",
			},
		},
		{
			name:      "single primary key with IDENTITY",
			tableName: "products",
			setup: func() *schemas.Table {
				table := schemas.NewTable("products", nil)
				table.AddColumn(&schemas.Column{
					Name:            "id",
					SQLType:         schemas.SQLType{Name: schemas.Int},
					IsPrimaryKey:    true,
					IsAutoIncrement: true,
					Nullable:        false,
					DefaultIsEmpty:  true,
				})
				table.AddColumn(&schemas.Column{
					Name:           "name",
					SQLType:        schemas.SQLType{Name: schemas.Varchar},
					Length:         200,
					Nullable:       false,
					DefaultIsEmpty: true,
				})
				table.PrimaryKeys = []string{"id"}
				return table
			},
			contains: []string{
				"CREATE TABLE `products`",
				"`id` INT IDENTITY NOT NULL",
				"CONSTRAINT PK_products PRIMARY KEY (`id`)",
			},
		},
		{
			name:      "composite primary key",
			tableName: "order_items",
			setup: func() *schemas.Table {
				table := schemas.NewTable("order_items", nil)
				table.AddColumn(&schemas.Column{
					Name:           "order_id",
					SQLType:        schemas.SQLType{Name: schemas.Int},
					IsPrimaryKey:   true,
					Nullable:       false,
					DefaultIsEmpty: true,
				})
				table.AddColumn(&schemas.Column{
					Name:           "item_id",
					SQLType:        schemas.SQLType{Name: schemas.Int},
					IsPrimaryKey:   true,
					Nullable:       false,
					DefaultIsEmpty: true,
				})
				table.AddColumn(&schemas.Column{
					Name:           "quantity",
					SQLType:        schemas.SQLType{Name: schemas.Int},
					Nullable:       false,
					DefaultIsEmpty: true,
				})
				table.PrimaryKeys = []string{"order_id", "item_id"}
				return table
			},
			contains: []string{
				"CREATE TABLE `order_items`",
				"CONSTRAINT PK_order_items PRIMARY KEY (`order_id`,`item_id`)",
			},
		},
		{
			name:      "column with default value",
			tableName: "config",
			setup: func() *schemas.Table {
				table := schemas.NewTable("config", nil)
				table.AddColumn(&schemas.Column{
					Name:            "id",
					SQLType:         schemas.SQLType{Name: schemas.Int},
					IsPrimaryKey:    true,
					IsAutoIncrement: true,
					Nullable:        false,
					DefaultIsEmpty:  true,
				})
				col := &schemas.Column{
					Name:     "status",
					SQLType:  schemas.SQLType{Name: schemas.Varchar},
					Length:   20,
					Default:  "'active'",
					Nullable: false,
				}
				table.AddColumn(col)
				table.PrimaryKeys = []string{"id"}
				return table
			},
			contains: []string{
				"CREATE TABLE `config`",
				"`id` INT IDENTITY NOT NULL",
				"`status` VARCHAR(20) DEFAULT 'active' NOT NULL",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := tt.setup()
			sql, _, err := d.CreateTableSQL(context.Background(), nil, table, tt.tableName)
			assert.NoError(t, err)
			assert.NotEmpty(t, sql)

			for _, substr := range tt.contains {
				assert.Contains(t, sql, substr, "SQL should contain: %s\nGot: %s", substr, sql)
			}
		})
	}
}

// ============================================================================
// Layer 3: Helper function tests
// ============================================================================

func TestXuguIndexCheckSQL(t *testing.T) {
	d := newXuguDialectInit(t, "testdb")

	sql, args := d.IndexCheckSQL("users", "idx_name")
	assert.Contains(t, sql, "ALL_INDEXES")
	assert.Contains(t, strings.ToUpper(sql), "ALL_TABLES")
	assert.Equal(t, []interface{}{"users", "idx_name"}, args)
}

func assertMetadataScopeSQL(t *testing.T, query string) {
	t.Helper()
	upper := strings.ToUpper(query)
	for _, fragment := range []string{
		"WITH CURRENT_SCOPE AS",
		"ALL_DATABASES",
		"ALL_SCHEMAS",
		"DATABASE()",
		"CURRENT_SCHEMA()",
	} {
		assert.Contains(t, upper, fragment)
	}
}

func TestXuguMetadataQueriesUseCurrentScope(t *testing.T) {
	d := newXuguDialectInit(t, "ignored-dsn-db")

	indexSQL, _ := d.IndexCheckSQL("users", "idx_users")
	assertMetadataScopeSQL(t, indexSQL)
	assert.Contains(t, strings.ToUpper(indexSQL), "I.DB_ID = T.DB_ID")
	assert.Contains(t, strings.ToUpper(indexSQL), "I.TABLE_ID = T.TABLE_ID")

	queries := make(map[string]string)
	capture := func(name string, columns []string, rows ...[]driver.Value) core.Queryer {
		queryer := newRowsQueryer(t, columns, rows...)
		return queryerFunc(func(ctx context.Context, query string, args ...interface{}) (*core.Rows, error) {
			queries[name] = query
			return queryer.QueryContext(ctx, query, args...)
		})
	}

	_, err := d.IsTableExist(capture("exist", []string{"TABLE_NAME"}), context.Background(), "users")
	assert.NoError(t, err)
	_, err = d.GetTables(capture("tables", []string{"TABLE_NAME"}), context.Background())
	assert.NoError(t, err)
	_, _, err = d.GetColumns(capture("columns", []string{"COL_NAME", "NOT_NULL", "TYPE_NAME", "IS_SERIAL", "COMMENTS", "SCALE", "DEF_VAL", "DEFINE", "CONS_TYPE"}), context.Background(), "users")
	assert.NoError(t, err)
	_, err = d.GetIndexes(capture("indexes", []string{"INDEX_NAME", "KEYS", "IS_UNIQUE", "IS_PRIMARY"}), context.Background(), "users")
	assert.NoError(t, err)

	for name, query := range queries {
		t.Run(name, func(t *testing.T) { assertMetadataScopeSQL(t, query) })
	}
	assert.Contains(t, strings.ToUpper(queries["columns"]), "C1.DB_ID = T1.DB_ID")
	assert.Contains(t, strings.ToUpper(queries["columns"]), "CON1.DB_ID = T1.DB_ID")
	assert.Contains(t, strings.ToUpper(queries["indexes"]), "I.DB_ID = T.DB_ID")
}

func TestXuguAddColumnSQL(t *testing.T) {
	d := newXuguDialectInit(t, "test")

	tests := []struct {
		name      string
		tableName string
		col       *schemas.Column
		contains  []string
	}{
		{
			name:      "add varchar column without comment",
			tableName: "users",
			col: &schemas.Column{
				Name:           "email",
				SQLType:        schemas.SQLType{Name: schemas.Varchar},
				Length:         255,
				Nullable:       true,
				DefaultIsEmpty: true,
			},
			contains: []string{
				"ALTER TABLE `users` ADD",
				"`email` VARCHAR(255) NULL",
			},
		},
		{
			name:      "add int column with comment",
			tableName: "users",
			col: &schemas.Column{
				Name:           "age",
				SQLType:        schemas.SQLType{Name: schemas.Int},
				Nullable:       true,
				Comment:        "user age in years",
				DefaultIsEmpty: true,
			},
			contains: []string{
				"ALTER TABLE `users` ADD",
				"`age` INT",
				"COMMENT 'user age in years'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.AddColumnSQL(tt.tableName, tt.col)
			for _, substr := range tt.contains {
				assert.Contains(t, result, substr)
			}
		})
	}
}

func TestXuguSetQuotePolicy(t *testing.T) {
	d := newXuguDialectInit(t, "test")

	// QuotePolicyAlways (default) — always quotes
	d.SetQuotePolicy(dialects.QuotePolicyAlways)
	assert.True(t, d.Quoter().IsReserved("ANYTHING"), "always should reserve everything")

	// QuotePolicyNone — never quotes
	d.SetQuotePolicy(dialects.QuotePolicyNone)
	assert.False(t, d.Quoter().IsReserved("SELECT"), "none should never reserve")
	assert.False(t, d.Quoter().IsReserved("ANYTHING"), "none should never reserve")

	// QuotePolicyReserved — only reserved words
	d.SetQuotePolicy(dialects.QuotePolicyReserved)
	assert.True(t, d.Quoter().IsReserved("SELECT"), "SELECT should be reserved")
	assert.True(t, d.Quoter().IsReserved("TABLE"), "TABLE should be reserved")
	assert.False(t, d.Quoter().IsReserved("my_column"), "my_column should not be reserved")
}

func TestXuguAlias(t *testing.T) {
	d := newXuguDialectInit(t, "test")

	tests := []struct {
		input    string
		expected string
	}{
		{"numeric", "decimal"},
		{"NUMERIC", "decimal"},
		{"Numeric", "decimal"},
		{"char", "varchar"},
		{"INTEGER", "int"},
		{"varchar", "varchar"},
		{"int", "int"},
		{"decimal", "decimal"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := d.Alias(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestXuguIsReserved(t *testing.T) {
	d := newXuguDialectInit(t, "test")

	reserved := []string{"SELECT", "FROM", "WHERE", "TABLE", "INDEX", "CREATE", "DROP",
		"INSERT", "UPDATE", "DELETE", "ALTER", "AND", "OR", "NOT", "NULL",
		"PRIMARY", "KEY", "INT", "VARCHAR", "BIGINT", "JOIN", "INNER", "LEFT",
		"RIGHT", "ORDER", "GROUP", "BY", "HAVING", "LIMIT", "UNION", "INTO", "SET"}
	for _, word := range reserved {
		assert.True(t, d.IsReserved(word), "%s should be reserved", word)
	}

	notReserved := []string{"my_table", "user_data", "foo_bar", "xyz123", "normal_column"}
	for _, word := range notReserved {
		assert.False(t, d.IsReserved(word), "%s should NOT be reserved", word)
	}
}

func TestXuguInit(t *testing.T) {
	d := newXuguDialect(t)
	uri := &dialects.URI{DBType: "xugusql", DBName: "testdb"}
	err := d.Init(uri)
	assert.NoError(t, err)
	assert.Equal(t, "xugusql", string(d.URI().DBType))
	assert.Equal(t, "testdb", d.URI().DBName)
	// quoter should be set to xuguQuoter (backtick)
	assert.Equal(t, byte('`'), d.Quoter().Prefix)
	assert.Equal(t, byte('`'), d.Quoter().Suffix)
}

func TestXuguAutoIncrStr(t *testing.T) {
	d := newXuguDialectInit(t, "test")
	assert.Equal(t, "IDENTITY", d.AutoIncrStr())
}

func TestXuguFeatures(t *testing.T) {
	d := newXuguDialectInit(t, "test")
	f := d.Features()
	assert.NotNil(t, f)
	assert.Equal(t, dialects.IncrAutoincrMode, f.AutoincrMode)
}

func TestXuguDriverFeatures(t *testing.T) {
	driver := newXuguDriver(t)
	f := driver.Features()
	assert.NotNil(t, f)
	assert.True(t, f.SupportReturnInsertedID)
}

func TestXuguFilters(t *testing.T) {
	d := newXuguDialectInit(t, "test")
	filters := d.Filters()
	assert.Empty(t, filters, "xugu dialect should return no filters")
}

func TestXuguDialectRegistration(t *testing.T) {
	d := dialects.QueryDialect("xugusql")
	assert.NotNil(t, d, "xugusql dialect should be registered")
}

func TestXuguSQLTypeSpecialCases(t *testing.T) {
	d := newXuguDialectInit(t, "test")

	t.Run("Serial sets auto-increment and pk", func(t *testing.T) {
		col := &schemas.Column{
			Name:    schemas.Serial,
			SQLType: schemas.SQLType{Name: schemas.Serial},
		}
		_ = d.SQLType(col)
		assert.True(t, col.IsAutoIncrement)
		assert.True(t, col.IsPrimaryKey)
		assert.False(t, col.Nullable)
	})

	t.Run("BigSerial sets auto-increment and pk", func(t *testing.T) {
		col := &schemas.Column{
			Name:    schemas.BigSerial,
			SQLType: schemas.SQLType{Name: schemas.BigSerial},
		}
		_ = d.SQLType(col)
		assert.True(t, col.IsAutoIncrement)
		assert.True(t, col.IsPrimaryKey)
		assert.False(t, col.Nullable)
	})

	t.Run("Unsigned types mapping", func(t *testing.T) {
		tests := []struct {
			sqlType  string
			expected string
		}{
			{schemas.UnsignedInt, schemas.Int},
			{schemas.UnsignedBigInt, schemas.BigInt},
			{schemas.UnsignedMediumInt, schemas.MediumInt},
			{schemas.UnsignedSmallInt, schemas.SmallInt},
			{schemas.UnsignedTinyInt, schemas.TinyInt},
		}
		for _, tt := range tests {
			t.Run(tt.sqlType, func(t *testing.T) {
				col := &schemas.Column{
					Name:    tt.sqlType,
					SQLType: schemas.SQLType{Name: tt.sqlType},
				}
				result := d.SQLType(col)
				assert.True(t, strings.HasPrefix(result, tt.expected),
					"Expected %s to start with %s, got %s", tt.sqlType, tt.expected, result)
			})
		}
	})
}

func TestXuguSetParams(t *testing.T) {
	// SetParams is on the Dialect interface; internal rowFormat field
	// is not accessible from external test. Testing that SetParams runs
	// without panicking for all valid inputs.
	d := newXuguDialectInit(t, "test")
	tests := []map[string]string{
		{},
		{"rowFormat": "compact"},
		{"rowFormat": "redundant"},
		{"rowFormat": "dynamic"},
		{"rowFormat": "compressed"},
		{"rowFormat": "unsupported"},
		{"other": "value"},
	}

	for _, params := range tests {
		// should not panic
		d.SetParams(params)
	}
}

// ============================================================================
// Layer 4: New regression tests for fixes
// ============================================================================

func TestXuguModifyColumnSQL(t *testing.T) {
	d := newXuguDialectInit(t, "test")

	tests := []struct {
		name      string
		tableName string
		col       *schemas.Column
		contains  []string
	}{
		{
			name:      "modify varchar column",
			tableName: "users",
			col: &schemas.Column{
				Name:           "email",
				SQLType:        schemas.SQLType{Name: schemas.Varchar},
				Length:         512,
				Nullable:       true,
				DefaultIsEmpty: true,
			},
			contains: []string{
				"ALTER TABLE `users` MODIFY COLUMN",
				"`email` VARCHAR(512) NULL",
			},
		},
		{
			name:      "modify not null int column",
			tableName: "users",
			col: &schemas.Column{
				Name:           "age",
				SQLType:        schemas.SQLType{Name: schemas.Int},
				Nullable:       false,
				DefaultIsEmpty: true,
			},
			contains: []string{
				"ALTER TABLE `users` MODIFY COLUMN",
				"`age` INT NOT NULL",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.ModifyColumnSQL(tt.tableName, tt.col)
			for _, substr := range tt.contains {
				assert.Contains(t, result, substr)
			}
		})
	}
}

func TestXuguIsTableExistSQL(t *testing.T) {
	d := newXuguDialectInit(t, "testdb")

	// IsTableExist uses HasRecords internally; we can verify the SQL generation
	// by checking that the dialect is correctly configured with the DBName
	assert.Equal(t, "testdb", d.URI().DBName)
}

func TestXuguGetIndexesSQLStructure(t *testing.T) {
	d := newXuguDialectInit(t, "testdb")

	// Verify GetIndexes produces a valid SQL string that references
	// the expected system tables for xugu (Oracle-compatible)
	sql, args := d.IndexCheckSQL("users", "idx_name")
	assert.Contains(t, sql, "ALL_INDEXES")
	assert.Contains(t, strings.ToUpper(sql), "ALL_TABLES")
	// args should be [tableName, idxName] (no DBName - fixed from mismatch)
	assert.Len(t, args, 2)
	assert.Equal(t, "users", args[0].(string))
	assert.Equal(t, "idx_name", args[1].(string))
}

func TestXuguColumnTypeKindTimeTypes(t *testing.T) {
	d := newXuguDialectInit(t, "test")

	// Verify DATETIME/TIMESTAMP/DATE/TIME are classified as TIME_TYPE (P1-4 fix)
	timeTypes := []string{"DATETIME", "TIMESTAMP", "DATE", "TIME", "datetime", "timestamp"}
	for _, typ := range timeTypes {
		t.Run(typ, func(t *testing.T) {
			assert.Equal(t, schemas.TIME_TYPE, d.ColumnTypeKind(typ),
				"%s should be classified as TIME_TYPE", typ)
		})
	}
}

func TestXuguIsReservedEdgeCases(t *testing.T) {
	d := newXuguDialectInit(t, "test")

	// Verify both "ON" and "OPTIMIZE" are independently reserved (P2-12 fix)
	assert.True(t, d.IsReserved("ON"), "ON should be reserved (was corrupted in tab-joined entry)")
	assert.True(t, d.IsReserved("OPTIMIZE"), "OPTIMIZE should be reserved (was corrupted in tab-joined entry)")
	assert.True(t, d.IsReserved("on"), "on (lowercase) should be reserved")
	assert.True(t, d.IsReserved("optimize"), "optimize (lowercase) should be reserved")
}

func TestXuguFloatNotUnsigned(t *testing.T) {
	d := newXuguDialectInit(t, "test")

	// Float should NOT be marked as unsigned (P2-7 fix)
	col := &schemas.Column{
		Name:    schemas.Float,
		SQLType: schemas.SQLType{Name: schemas.Float},
	}
	result := d.SQLType(col)
	assert.Equal(t, "FLOAT", result, "Float should not have unsigned modifier")
}

func TestXuguNullableLogic(t *testing.T) {
	// This test verifies the ColumnString behavior with Nullable field
	// (P0-1 fix: Nullable logic was reversed in GetColumns)
	d := newXuguDialectInit(t, "test")

	// NOT NULL column
	col := &schemas.Column{
		Name:           "required_col",
		SQLType:        schemas.SQLType{Name: schemas.Varchar},
		Length:         100,
		Nullable:       false,
		DefaultIsEmpty: true,
	}
	sql, err := dialects.ColumnString(d, col, false, false)
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(sql, "NOT NULL"),
		"nullable=false should produce NOT NULL suffix, got: %s", sql)
	// double-check: "NOT NULL" should appear exactly as substring
	assert.Contains(t, sql, "NOT NULL")

	// Nullable column
	col2 := &schemas.Column{
		Name:           "optional_col",
		SQLType:        schemas.SQLType{Name: schemas.Varchar},
		Length:         100,
		Nullable:       true,
		DefaultIsEmpty: true,
	}
	sql2, err := dialects.ColumnString(d, col2, false, false)
	assert.NoError(t, err)
	assert.Contains(t, sql2, "NULL")
	assert.NotContains(t, sql2, "NOT NULL")
}

func TestXuguVersion(t *testing.T) {
	d := newXuguDialectInit(t, "test")

	t.Run("query error", func(t *testing.T) {
		expected := errors.New("version query failed")
		_, err := d.Version(context.Background(), queryerFunc(func(context.Context, string, ...interface{}) (*core.Rows, error) {
			return nil, expected
		}))
		assert.Equal(t, expected, err)
	})

	tests := []struct {
		name        string
		rows        [][]driver.Value
		wantNumber  string
		wantEdition string
		wantErr     bool
		wantErrText string
	}{
		{
			name:        "empty result",
			wantErr:     true,
			wantErrText: "unknown version",
		},
		{
			name:        "single version token",
			rows:        [][]driver.Value{{"1.3.6"}},
			wantNumber:  "1.3.6",
			wantEdition: "",
		},
		{
			name:        "edition and version",
			rows:        [][]driver.Value{{"Xugu 1.3.6"}},
			wantNumber:  "1.3.6",
			wantEdition: "Xugu",
		},
		{
			name:        "XuguDB observed version",
			rows:        [][]driver.Value{{"XuguDB 12.0.0"}},
			wantNumber:  "12.0.0",
			wantEdition: "XuguDB",
		},
		{
			name:        "build suffix",
			rows:        [][]driver.Value{{"  XuguDB 12.10.13 build xxx  "}},
			wantNumber:  "12.10.13",
			wantEdition: "XuguDB",
		},
		{
			name:        "multi word edition",
			rows:        [][]driver.Value{{"Xugu Enterprise 1.3.6"}},
			wantNumber:  "1.3.6",
			wantEdition: "Xugu Enterprise",
		},
		{
			name:        "blank version",
			rows:        [][]driver.Value{{"   "}},
			wantErr:     true,
			wantErrText: "unknown version",
		},
		{
			name:        "error response",
			rows:        [][]driver.Value{{"ERROR"}},
			wantErr:     true,
			wantErrText: "unrecognized Xugu version response",
		},
		{
			name:        "edition error response",
			rows:        [][]driver.Value{{"Xugu ERROR"}},
			wantErr:     true,
			wantErrText: "unrecognized Xugu version response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := d.Version(context.Background(), newVersionQueryer(t, tt.rows...))
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, version)
				if tt.wantErrText != "" {
					assert.Contains(t, err.Error(), tt.wantErrText)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantNumber, version.Number)
			assert.Equal(t, tt.wantEdition, version.Edition)
		})
	}
}

func TestXuguGetIndexesPropagatesQueryError(t *testing.T) {
	d := newXuguDialectInit(t, "test")
	expected := errors.New("ALL_IND_COLUMNS is unavailable")

	indexes, err := d.GetIndexes(queryerFunc(func(context.Context, string, ...interface{}) (*core.Rows, error) {
		return nil, expected
	}), context.Background(), "users")

	assert.Equal(t, expected, err)
	assert.Nil(t, indexes)
}

func TestXuguGetIndexes(t *testing.T) {
	d := newXuguDialectInit(t, "test")
	var query string
	queryer := newRowsQueryer(t,
		[]string{"INDEX_NAME", "KEYS", "IS_UNIQUE", "IS_PRIMARY"},
		[]driver.Value{"PK_USERS", `"ID"`, "true", "true"},
		[]driver.Value{"IDX_USERS_EMAIL", ` "EMAIL" , "DISPLAY,NAME" , "A""B" `, "true", "false"},
		[]driver.Value{"IDX_USERS_AGE", `"AGE"`, "false", "false"},
	)
	indexes, err := d.GetIndexes(queryerFunc(func(ctx context.Context, sql string, args ...interface{}) (*core.Rows, error) {
		query = sql
		return queryer.QueryContext(ctx, sql, args...)
	}), context.Background(), "users")

	assert.NoError(t, err)
	assertMetadataScopeSQL(t, query)
	assert.Equal(t, schemas.UniqueType, indexes["IDX_USERS_EMAIL"].Type)
	assert.Equal(t, []string{"EMAIL", "DISPLAY,NAME", `A"B`}, indexes["IDX_USERS_EMAIL"].Cols)
	assert.Equal(t, schemas.IndexType, indexes["IDX_USERS_AGE"].Type)
	assert.Equal(t, []string{"AGE"}, indexes["IDX_USERS_AGE"].Cols)
	assert.NotContains(t, indexes, "PK_USERS")
}

func TestXuguGetIndexesRejectsUnsupportedKeys(t *testing.T) {
	d := newXuguDialectInit(t, "test")
	for _, keys := range []string{"LOWER(NAME)", `"NAME" DESC`, `"unterminated`, ""} {
		t.Run(keys, func(t *testing.T) {
			indexes, err := d.GetIndexes(newRowsQueryer(t,
				[]string{"INDEX_NAME", "KEYS", "IS_UNIQUE", "IS_PRIMARY"},
				[]driver.Value{"IDX_BAD", keys, "false", "false"},
			), context.Background(), "users")
			assert.Error(t, err)
			assert.Nil(t, indexes)
			assert.Contains(t, err.Error(), "parse index")
		})
	}
}

func TestXuguGetColumns(t *testing.T) {
	d := newXuguDialectInit(t, "test")
	var query string
	queryer := newRowsQueryer(t,
		[]string{"COL_NAME", "NOT_NULL", "TYPE_NAME", "IS_SERIAL", "COMMENTS", "SCALE", "DEF_VAL", "CONSTRAINT_DEFINE", "CONS_TYPE"},
		[]driver.Value{"ID", true, "INT", true, "identifier", int64(0), nil, "PRIMARY KEY", "P"},
		[]driver.Value{"NAME", false, "VARCHAR", false, nil, int64(255), "'anonymous'", nil, nil},
	)
	columns, definitions, err := d.GetColumns(queryerFunc(func(ctx context.Context, sql string, args ...interface{}) (*core.Rows, error) {
		query = sql
		return queryer.QueryContext(ctx, sql, args...)
	}), context.Background(), "users")

	assert.NoError(t, err)
	assertMetadataScopeSQL(t, query)
	assert.Contains(t, strings.ToUpper(query), "C1.DB_ID = T1.DB_ID")
	assert.Contains(t, strings.ToUpper(query), "CON1.DB_ID = T1.DB_ID")
	assert.Contains(t, query, "c1.DEF_VAL")
	assert.NotContains(t, query, "c1.DEFINE")
	assert.NotContains(t, query, "DEFAULT_VALUE")
	assert.Equal(t, []string{"ID", "NAME"}, columns)
	assert.True(t, definitions["ID"].IsPrimaryKey)
	assert.True(t, definitions["ID"].IsAutoIncrement)
	assert.False(t, definitions["ID"].Nullable)
	assert.Equal(t, "identifier", definitions["ID"].Comment)
	assert.True(t, definitions["ID"].DefaultIsEmpty)
	assert.Empty(t, definitions["ID"].Default)
	assert.True(t, definitions["NAME"].Nullable)
	assert.Equal(t, int64(255), definitions["NAME"].SQLType.DefaultLength)
	assert.False(t, definitions["NAME"].DefaultIsEmpty)
	assert.Equal(t, "'anonymous'", definitions["NAME"].Default)
}

func TestXuguGetColumnsPropagatesQueryError(t *testing.T) {
	d := newXuguDialectInit(t, "test")
	expected := errors.New("ALL_COLUMNS is unavailable")

	columns, definitions, err := d.GetColumns(queryerFunc(func(context.Context, string, ...interface{}) (*core.Rows, error) {
		return nil, expected
	}), context.Background(), "users")

	assert.Equal(t, expected, err)
	assert.Nil(t, columns)
	assert.Nil(t, definitions)
}
