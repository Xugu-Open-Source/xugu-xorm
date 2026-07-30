// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xugu

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-xorm/core"
)

func init() {
	core.RegisterDriver("xugu", &xuguDriver{})
	core.RegisterDialect("xugusql", func() core.Dialect {
		return &xugu{}
	})
}

var xuguReservedWords = map[string]bool{
	"ADD": true, "ALL": true, "ALTER": true, "ANALYZE": true, "AND": true,
	"AS": true, "ASC": true, "ASENSITIVE": true, "BEFORE": true, "BETWEEN": true,
	"BIGINT": true, "BINARY": true, "BLOB": true, "BOTH": true, "BY": true,
	"CALL": true, "CASCADE": true, "CASE": true, "CHAIN": true, "CHANGE": true,
	"CHAR": true, "CHARACTER": true, "CHECK": true, "COLLATE": true, "COLUMN": true,
	"CONDITION": true, "CONNECTION": true, "CONSTRAINT": true, "CONTINUE": true,
	"CONVERT": true, "CREATE": true, "CROSS": true, "CURRENT_DATE": true,
	"CURRENT_TIME": true, "CURRENT_TIMESTAMP": true, "CURRENT_USER": true,
	"CURSOR": true, "DATABASE": true, "DATABASES": true, "DAY_HOUR": true,
	"DAY_MICROSECOND": true, "DAY_MINUTE": true, "DAY_SECOND": true, "DEC": true,
	"DECIMAL": true, "DECLARE": true, "DEFAULT": true, "DELAYED": true,
	"DELETE": true, "DESC": true, "DESCRIBE": true, "DETERMINISTIC": true,
	"DISTINCT": true, "DISTINCTROW": true, "DIV": true, "DOUBLE": true,
	"DROP": true, "DUAL": true, "EACH": true, "ELSE": true, "ELSEIF": true,
	"ENCLOSED": true, "ESCAPED": true, "EXISTS": true, "EXIT": true,
	"EXPLAIN": true, "FALSE": true, "FETCH": true, "FLOAT": true, "FLOAT4": true,
	"FLOAT8": true, "FOR": true, "FORCE": true, "FOREIGN": true, "FROM": true,
	"FULLTEXT": true, "GOTO": true, "GRANT": true, "GROUP": true, "HAVING": true,
	"HIGH_PRIORITY": true, "HOUR_MICROSECOND": true, "HOUR_MINUTE": true,
	"HOUR_SECOND": true, "IF": true, "IGNORE": true, "IN": true, "INDEX": true,
	"INFILE": true, "INNER": true, "INOUT": true, "INSENSITIVE": true,
	"INSERT": true, "INT": true, "INT1": true, "INT2": true, "INT3": true,
	"INT4": true, "INT8": true, "INTEGER": true, "INTERVAL": true, "INTO": true,
	"IS": true, "ITERATE": true, "JOIN": true, "KEY": true, "KEYS": true,
	"KILL": true, "LABEL": true, "LEADING": true, "LEAVE": true, "LEFT": true,
	"LIKE": true, "LIMIT": true, "LINEAR": true, "LINES": true, "LOAD": true,
	"LOCALTIME": true, "LOCALTIMESTAMP": true, "LOCK": true, "LONG": true,
	"LONGBLOB": true, "LONGTEXT": true, "LOOP": true, "LOW_PRIORITY": true,
	"MATCH": true, "MEDIUMBLOB": true, "MEDIUMINT": true, "MEDIUMTEXT": true,
	"MIDDLEINT": true, "MINUTE_MICROSECOND": true, "MINUTE_SECOND": true,
	"MOD": true, "MODIFIES": true, "NATURAL": true, "NOT": true,
	"NO_WRITE_TO_BINLOG": true, "NULL": true, "NUMERIC": true, "ON": true,
	"OPTIMIZE": true, "OPTION": true, "OPTIONALLY": true, "OR": true,
	"ORDER": true, "OUT": true, "OUTER": true, "OUTFILE": true, "PRECISION": true,
	"PRIMARY": true, "PROCEDURE": true, "PURGE": true, "RAID0": true, "RANGE": true,
	"RANK": true, "READ": true, "READS": true, "REAL": true, "REFERENCES": true,
	"REGEXP": true, "RELEASE": true, "RENAME": true, "REPEAT": true,
	"REPLACE": true, "REQUIRE": true, "RESTRICT": true, "RETURN": true,
	"REVOKE": true, "RIGHT": true, "RLIKE": true, "SCHEMA": true, "SCHEMAS": true,
	"SECOND_MICROSECOND": true, "SELECT": true, "SENSITIVE": true,
	"SEPARATOR": true, "SET": true, "SHOW": true, "SMALLINT": true,
	"SPATIAL": true, "SPECIFIC": true, "SQL": true, "SQLEXCEPTION": true,
	"SQLSTATE": true, "SQLWARNING": true, "SQL_BIG_RESULT": true,
	"SQL_CALC_FOUND_ROWS": true, "SQL_SMALL_RESULT": true, "SSL": true,
	"STARTING": true, "STRAIGHT_JOIN": true, "TABLE": true, "TERMINATED": true,
	"THEN": true, "TINYBLOB": true, "TINYINT": true, "TINYTEXT": true, "TO": true,
	"TRAILING": true, "TRIGGER": true, "TRUE": true, "UNDO": true, "UNION": true,
	"UNIQUE": true, "UNLOCK": true, "UNSIGNED": true, "UPDATE": true,
	"USAGE": true, "USE": true, "USING": true, "UTC_DATE": true, "UTC_TIME": true,
	"UTC_TIMESTAMP": true, "VALUES": true, "VARBINARY": true, "VARCHAR": true,
	"VARCHARACTER": true, "VARYING": true, "WHEN": true, "WHERE": true,
	"WHILE": true, "WITH": true, "WRITE": true, "X509": true, "XOR": true,
	"YEAR_MONTH": true, "ZEROFILL": true,
}

type xugu struct {
	core.Base
	rowFormat string
}

func (db *xugu) Init(d *core.DB, uri *core.Uri, drivername, dataSourceName string) error {
	return db.Base.Init(d, db, uri, drivername, dataSourceName)
}

func (db *xugu) SetParams(params map[string]string) {
	rowFormat, ok := params["rowFormat"]
	if ok {
		t := strings.ToUpper(rowFormat)
		switch t {
		case "COMPACT", "REDUNDANT", "DYNAMIC", "COMPRESSED":
			db.rowFormat = t
		}
	}
}

func (db *xugu) SqlType(c *core.Column) string {
	var res string
	switch t := c.SQLType.Name; t {
	case core.Bool:
		res = core.TinyInt
		c.Length = 1
	case core.Serial:
		c.IsAutoIncrement = true
		c.IsPrimaryKey = true
		c.Nullable = false
		res = core.Int
	case core.BigSerial:
		c.IsAutoIncrement = true
		c.IsPrimaryKey = true
		c.Nullable = false
		res = core.BigInt
	case core.Bytea:
		res = core.Blob
	case core.TimeStampz:
		res = core.Char
		c.Length = 64
	case core.Enum:
		res = core.Enum
		res += "("
		opts := ""
		for v := range c.EnumOptions {
			opts += fmt.Sprintf(",'%v'", v)
		}
		res += strings.TrimLeft(opts, ",")
		res += ")"
	case core.Set:
		res = core.Set
		res += "("
		opts := ""
		for v := range c.SetOptions {
			opts += fmt.Sprintf(",'%v'", v)
		}
		res += strings.TrimLeft(opts, ",")
		res += ")"
	case core.NVarchar:
		res = core.Varchar
	case core.Uuid:
		res = core.Varchar
		c.Length = 40
	case core.Json:
		res = core.Text
	case core.Float:
		res = core.Float
	case "NUMERIC":
		res = core.Decimal
	default:
		res = t
	}

	hasLen1 := c.Length > 0
	hasLen2 := c.Length2 > 0
	if hasLen2 {
		res += "(" + strconv.Itoa(c.Length) + "," + strconv.Itoa(c.Length2) + ")"
	} else if hasLen1 {
		res += "(" + strconv.Itoa(c.Length) + ")"
	}
	return res
}

func (db *xugu) SupportInsertMany() bool {
	return true
}

func (db *xugu) IsReserved(name string) bool {
	_, ok := xuguReservedWords[strings.ToUpper(name)]
	return ok
}

func (db *xugu) Quote(name string) string {
	return "`" + name + "`"
}

func (db *xugu) QuoteStr() string {
	return "`"
}

func (db *xugu) SupportEngine() bool {
	return false
}

func (db *xugu) SupportCharset() bool {
	return false
}

func (db *xugu) SupportDropIfExists() bool {
	return true
}

func (db *xugu) IndexOnTable() bool {
	return true
}

func (db *xugu) AutoIncrStr() string {
	return "IDENTITY"
}

func (db *xugu) IndexCheckSql(tableName, idxName string) (string, []interface{}) {
	args := []interface{}{tableName, idxName}
	sql := `SELECT INDEX_NAME FROM ALL_INDEXES i
		JOIN ALL_TABLES t ON i.TABLE_ID = t.TABLE_ID
		WHERE t.table_name = ? AND index_name = ?`
	return sql, args
}

func (db *xugu) TableCheckSql(tableName string) (string, []interface{}) {
	args := []interface{}{tableName}
	return `SELECT TABLE_NAME FROM ALL_TABLES WHERE TABLE_NAME = ?`, args
}

func (db *xugu) IsColumnExist(tableName, colName string) (bool, error) {
	query := `SELECT c.COL_NAME FROM ALL_COLUMNS c
		JOIN ALL_TABLES t ON c.TABLE_ID = t.TABLE_ID
		WHERE t.TABLE_NAME = ? AND c.COL_NAME = ?`
	return db.HasRecords(query, tableName, colName)
}

func (db *xugu) ModifyColumnSql(tableName string, col *core.Column) string {
	return fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s", db.Quote(tableName), col.StringNoPk(db))
}

func (db *xugu) GetColumns(tableName string) ([]string, map[string]*core.Column, error) {
	args := []interface{}{tableName}
	s := `
SELECT c1.COL_NAME, c1.NOT_NULL, c1.TYPE_NAME, c1.IS_SERIAL, c1.COMMENTS, c1.SCALE, con1.DEFINE, con1.CONS_TYPE
FROM all_tables t1
JOIN all_columns c1 ON c1.TABLE_ID = t1.TABLE_ID
LEFT JOIN all_constraints con1 ON con1.table_id = t1.TABLE_ID AND con1.define like '%"'||c1.col_name||'"%'
WHERE t1.TABLE_NAME = ?`

	db.LogSQL(s, args)
	rows, err := db.DB().Query(s, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	cols := make(map[string]*core.Column)
	colSeq := make([]string, 0)

	for rows.Next() {
		col := new(core.Column)
		col.Indexes = make(map[string]int)

		var scale int
		var colName, typeName string
		var comment, define, consType sql.NullString
		var notNull, isSerial bool

		err = rows.Scan(&colName, &notNull, &typeName, &isSerial, &comment, &scale, &define, &consType)
		if err != nil {
			return nil, nil, err
		}
		col.Name = colName
		col.Comment = comment.String
		col.Nullable = !notNull
		col.IsAutoIncrement = isSerial
		col.Length = scale
		col.SQLType = core.SQLType{Name: typeName, DefaultLength: scale, DefaultLength2: 0}
		if consType.Valid && consType.String == "P" {
			col.IsPrimaryKey = true
		}
		cols[col.Name] = col
		colSeq = append(colSeq, col.Name)
	}
	return colSeq, cols, nil
}

func (db *xugu) GetTables() ([]*core.Table, error) {
	s := `SELECT TABLE_NAME FROM ALL_TABLES`
	db.LogSQL(s, nil)
	rows, err := db.DB().Query(s)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]*core.Table, 0)
	for rows.Next() {
		table := core.NewEmptyTable()
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, err
		}
		table.Name = name
		tables = append(tables, table)
	}
	return tables, nil
}

func (db *xugu) GetIndexes(tableName string) (map[string]*core.Index, error) {
	// ALL_IND_COLUMNS 不是所有虚谷版本都存在；查询失败时返回空 map，避免中断 DBMetas。
	args := []interface{}{tableName}
	s := `SELECT i.INDEX_NAME, i.UNIQUENESS, c.COLUMN_NAME
		FROM ALL_INDEXES i
		LEFT JOIN ALL_IND_COLUMNS c ON i.INDEX_NAME = c.INDEX_NAME
		JOIN ALL_TABLES t ON i.TABLE_ID = t.TABLE_ID
		WHERE t.TABLE_NAME = ? AND i.INDEX_NAME NOT LIKE 'SYS%'
		ORDER BY c.COLUMN_POSITION`

	db.LogSQL(s, args)
	rows, err := db.DB().Query(s, args...)
	if err != nil {
		return make(map[string]*core.Index), nil
	}
	defer rows.Close()

	indexes := make(map[string]*core.Index)
	for rows.Next() {
		var indexName, uniqueness, colName string
		if err := rows.Scan(&indexName, &uniqueness, &colName); err != nil {
			return nil, err
		}
		indexName = strings.TrimSpace(indexName)
		indexType := core.IndexType
		if strings.ToUpper(strings.TrimSpace(uniqueness)) == "UNIQUE" {
			indexType = core.UniqueType
		}
		idx, ok := indexes[indexName]
		if !ok {
			idx = new(core.Index)
			idx.Name = indexName
			idx.Type = indexType
			indexes[indexName] = idx
		}
		if colName != "" {
			idx.AddColumn(colName)
		}
	}
	return indexes, nil
}

func (db *xugu) columnString(col *core.Column, includeAutoIncr bool) string {
	var b strings.Builder
	b.WriteString(db.Quote(col.Name))
	b.WriteByte(' ')
	b.WriteString(db.SqlType(col))

	if includeAutoIncr && col.IsPrimaryKey && col.IsAutoIncrement {
		b.WriteByte(' ')
		b.WriteString(db.AutoIncrStr())
	}

	// go-xorm 0.7 映射后未设默认值时常见 Default=="" && DefaultIsEmpty==false；
	// 虚谷对 IDENTITY/数值列不允许 DEFAULT ''。
	if col.Default != "" {
		b.WriteString(" DEFAULT ")
		b.WriteString(col.Default)
	}

	if col.Nullable {
		b.WriteString(" NULL")
	} else {
		b.WriteString(" NOT NULL")
	}
	return b.String()
}

func (db *xugu) CreateTableSql(table *core.Table, tableName, storeEngine, charset string) string {
	if tableName == "" {
		tableName = table.Name
	}

	sql := "CREATE TABLE " + db.Quote(tableName) + " ("
	pkList := table.PrimaryKeys
	cols := table.ColumnsSeq()

	for i, colName := range cols {
		col := table.GetColumn(colName)
		if (col.SQLType.Name == core.Bool || col.SQLType.Name == core.Boolean) && !col.DefaultIsEmpty {
			if col.Default == "true" {
				col.Default = "1"
			} else if col.Default == "false" {
				col.Default = "0"
			}
		}
		sql += db.columnString(col, col.IsPrimaryKey && len(pkList) == 1)
		if len(col.Comment) > 0 {
			sql += " COMMENT '" + col.Comment + "'"
		}
		if i != len(cols)-1 {
			sql += ", "
		}
	}

	if len(pkList) > 0 {
		if len(cols) > 0 {
			sql += ", "
		}
		sql += "CONSTRAINT PK_" + tableName + " PRIMARY KEY ("
		sql += db.Quote(strings.Join(pkList, db.Quote(",")))
		sql += ")"
	}
	sql += ")"

	if db.rowFormat != "" {
		sql += " ROW_FORMAT=" + db.rowFormat
	}
	_ = storeEngine
	_ = charset
	return sql
}

func (db *xugu) Filters() []core.Filter {
	return []core.Filter{}
}

type xuguDriver struct{}

func (p *xuguDriver) Parse(driverName, dataSourceName string) (*core.Uri, error) {
	uri := &core.Uri{DbType: "xugusql"}
	pairs := strings.Split(dataSourceName, ";")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, value := strings.TrimSpace(kv[0]), strings.Trim(strings.TrimSpace(kv[1]), "'")
		switch strings.ToLower(key) {
		case "ip":
			uri.Host = value
		case "port":
			uri.Port = value
		case "db":
			uri.DbName = value
		case "user":
			uri.User = value
		case "pwd":
			uri.Passwd = value
		case "char_set":
			uri.Charset = value
		}
	}
	return uri, nil
}
