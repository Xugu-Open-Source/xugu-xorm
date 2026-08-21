// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xugu

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"xorm.io/xorm/core"
	"xorm.io/xorm/dialects"
	"xorm.io/xorm/schemas"
)

func init() {
	dialects.RegisterDriver("xugu", &xuguDriver{})
	dialects.RegisterDialect("xugusql", func() dialects.Dialect {
		return &xugu{}
	})
}

var (
	xuguVersionNumber = regexp.MustCompile(`(^|[^0-9A-Za-z])([0-9]+(?:\.[0-9]+)+(?:[-+][0-9A-Za-z][0-9A-Za-z.-]*)?)($|[^0-9A-Za-z])`)

	xuguReservedWords = map[string]bool{
		"ADD":               true,
		"ALL":               true,
		"ALTER":             true,
		"ANALYZE":           true,
		"AND":               true,
		"AS":                true,
		"ASC":               true,
		"ASENSITIVE":        true,
		"BEFORE":            true,
		"BETWEEN":           true,
		"BIGINT":            true,
		"BINARY":            true,
		"BLOB":              true,
		"BOTH":              true,
		"BY":                true,
		"CALL":              true,
		"CASCADE":           true,
		"CASE":              true,
		"CHAIN":             true,
		"CHANGE":            true,
		"CHAR":              true,
		"CHARACTER":         true,
		"CHECK":             true,
		"COLLATE":           true,
		"COLUMN":            true,
		"CONDITION":         true,
		"CONNECTION":        true,
		"CONSTRAINT":        true,
		"CONTINUE":          true,
		"CONVERT":           true,
		"CREATE":            true,
		"CROSS":             true,
		"CURRENT_DATE":      true,
		"CURRENT_TIME":      true,
		"CURRENT_TIMESTAMP": true,
		"CURRENT_USER":      true,
		"CURSOR":            true,
		"DATABASE":          true,
		"DATABASES":         true,
		"DAY_HOUR":          true,
		"DAY_MICROSECOND":   true,
		"DAY_MINUTE":        true,
		"DAY_SECOND":        true,
		"DEC":               true,
		"DECIMAL":           true,
		"DECLARE":           true,
		"DEFAULT":           true,
		"DELAYED":           true,
		"DELETE":            true,
		"DESC":              true,
		"DESCRIBE":          true,
		"DETERMINISTIC":     true,
		"DISTINCT":          true,
		"DISTINCTROW":       true,
		"DIV":               true,
		"DOUBLE":            true,
		"DROP":              true,
		"DUAL":              true,
		"EACH":              true,
		"ELSE":              true,
		"ELSEIF":            true,
		"ENCLOSED":          true,
		"ESCAPED":           true,
		"EXISTS":            true,
		"EXIT":              true,
		"EXPLAIN":           true,
		"FALSE":             true,
		"FETCH":             true,
		"FLOAT":             true,
		"FLOAT4":            true,
		"FLOAT8":            true,
		"FOR":               true,
		"FORCE":             true,
		"FOREIGN":           true,
		"FROM":              true,
		"FULLTEXT":          true,
		"GOTO":              true,
		"GRANT":             true,
		"GROUP":             true,
		"HAVING":            true,
		"HIGH_PRIORITY":     true,
		"HOUR_MICROSECOND":  true,
		"HOUR_MINUTE":       true,
		"HOUR_SECOND":       true,
		"IF":                true,
		"IGNORE":            true,
		"IN":                true, "INDEX": true,
		"INFILE": true, "INNER": true, "INOUT": true,
		"INSENSITIVE": true, "INSERT": true, "INT": true,
		"INT1": true, "INT2": true, "INT3": true,
		"INT4": true, "INT8": true, "INTEGER": true,
		"INTERVAL": true, "INTO": true, "IS": true,
		"ITERATE": true, "JOIN": true, "KEY": true,
		"KEYS": true, "KILL": true, "LABEL": true,
		"LEADING": true, "LEAVE": true, "LEFT": true,
		"LIKE": true, "LIMIT": true, "LINEAR": true,
		"LINES": true, "LOAD": true, "LOCALTIME": true,
		"LOCALTIMESTAMP": true, "LOCK": true, "LONG": true,
		"LONGBLOB": true, "LONGTEXT": true, "LOOP": true,
		"LOW_PRIORITY": true, "MATCH": true, "MEDIUMBLOB": true,
		"MEDIUMINT": true, "MEDIUMTEXT": true, "MIDDLEINT": true,
		"MINUTE_MICROSECOND": true, "MINUTE_SECOND": true, "MOD": true,
		"MODIFIES": true, "NATURAL": true, "NOT": true,
		"NO_WRITE_TO_BINLOG": true, "NULL": true, "NUMERIC": true,
		"ON": true, "OPTIMIZE": true, "OPTION": true,
		"OPTIONALLY": true, "OR": true, "ORDER": true,
		"OUT": true, "OUTER": true, "OUTFILE": true,
		"PRECISION": true, "PRIMARY": true, "PROCEDURE": true,
		"PURGE": true, "RAID0": true, "RANGE": true,
		"RANK": true,
		"READ": true, "READS": true, "REAL": true,
		"REFERENCES": true, "REGEXP": true, "RELEASE": true,
		"RENAME": true, "REPEAT": true, "REPLACE": true,
		"REQUIRE": true, "RESTRICT": true, "RETURN": true,
		"REVOKE": true, "RIGHT": true, "RLIKE": true,
		"SCHEMA": true, "SCHEMAS": true, "SECOND_MICROSECOND": true,
		"SELECT": true, "SENSITIVE": true, "SEPARATOR": true,
		"SET": true, "SHOW": true, "SMALLINT": true,
		"SPATIAL": true, "SPECIFIC": true, "SQL": true,
		"SQLEXCEPTION": true, "SQLSTATE": true, "SQLWARNING": true,
		"SQL_BIG_RESULT": true, "SQL_CALC_FOUND_ROWS": true, "SQL_SMALL_RESULT": true,
		"SSL": true, "STARTING": true, "STRAIGHT_JOIN": true,
		"TABLE": true, "TERMINATED": true, "THEN": true,
		"TINYBLOB": true, "TINYINT": true, "TINYTEXT": true,
		"TO": true, "TRAILING": true, "TRIGGER": true,
		"TRUE": true, "UNDO": true, "UNION": true,
		"UNIQUE": true, "UNLOCK": true, "UNSIGNED": true,
		"UPDATE": true, "USAGE": true, "USE": true,
		"USING": true, "UTC_DATE": true, "UTC_TIME": true,
		"UTC_TIMESTAMP": true, "VALUES": true, "VARBINARY": true,
		"VARCHAR":      true,
		"VARCHARACTER": true,
		"VARYING":      true,
		"WHEN":         true,
		"WHERE":        true,
		"WHILE":        true,
		"WITH":         true,
		"WRITE":        true,
		"X509":         true,
		"XOR":          true,
		"YEAR_MONTH":   true,
		"ZEROFILL":     true,
	}

	xuguQuoter = schemas.Quoter{
		Prefix:     '`',
		Suffix:     '`',
		IsReserved: schemas.AlwaysReserve,
	}
)

const xuguCurrentScopeSQL = `WITH CURRENT_SCOPE AS (
	SELECT s.DB_ID, s.SCHEMA_ID
	FROM ALL_SCHEMAS s
	WHERE s.DB_ID = (
		SELECT d.DB_ID FROM ALL_DATABASES d
		WHERE d.DB_NAME = DATABASE() LIMIT 1
	)
	AND s.SCHEMA_NAME = CURRENT_SCHEMA()
)`

type xugu struct {
	dialects.Base
	rowFormat string
	quoter    schemas.Quoter // shadows Base.quoter; used via Quoter() override
}

func (db *xugu) Init(uri *dialects.URI) error {
	db.quoter = xuguQuoter
	return db.Base.Init(db, uri)
}

// Quoter overrides Base.Quoter to return the xugu quoter.
func (db *xugu) Quoter() schemas.Quoter {
	return db.quoter
}

var xuguColAliases = map[string]string{
	"numeric": "decimal",
	"char":    "varchar",
	"integer": "int",
}

// Alias returns a alias of column
func (db *xugu) Alias(col string) string {
	v, ok := xuguColAliases[strings.ToLower(col)]
	if ok {
		return v
	}
	return col
}

func (db *xugu) Version(ctx context.Context, queryer core.Queryer) (*schemas.Version, error) {
	rows, err := queryer.QueryContext(ctx, "SELECT VERSION()")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var version string
	if !rows.Next() {
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		return nil, errors.New("unknown version")
	}

	if err := rows.Scan(&version); err != nil {
		return nil, err
	}

	version = strings.TrimSpace(version)
	if version == "" {
		return nil, errors.New("unknown version")
	}

	match := xuguVersionNumber.FindStringSubmatchIndex(version)
	if match == nil {
		return nil, fmt.Errorf("unrecognized Xugu version response %q", version)
	}
	number := version[match[4]:match[5]]
	edition := strings.TrimSpace(version[:match[4]])

	return &schemas.Version{
		Number:  number,
		Edition: edition,
	}, nil
}

func (db *xugu) Features() *dialects.DialectFeatures {
	return &dialects.DialectFeatures{
		AutoincrMode: dialects.IncrAutoincrMode,
	}
}

func (db *xugu) SetParams(params map[string]string) {
	rowFormat, ok := params["rowFormat"]
	if ok {
		t := strings.ToUpper(rowFormat)
		switch t {
		case "COMPACT":
			fallthrough
		case "REDUNDANT":
			fallthrough
		case "DYNAMIC":
			fallthrough
		case "COMPRESSED":
			db.rowFormat = t
		}
	}
}

func (db *xugu) SQLType(c *schemas.Column) string {
	var res string
	var isUnsigned bool
	switch t := c.SQLType.Name; t {
	case schemas.Bool:
		res = schemas.TinyInt
		c.Length = 1
	case schemas.Serial:
		c.IsAutoIncrement = true
		c.IsPrimaryKey = true
		c.Nullable = false
		res = schemas.Int
	case schemas.BigSerial:
		c.IsAutoIncrement = true
		c.IsPrimaryKey = true
		c.Nullable = false
		res = schemas.BigInt
	case schemas.Bytea:
		res = schemas.Blob
	case schemas.TimeStampz:
		res = schemas.Char
		c.Length = 64
	case schemas.Enum: // xugu enum
		res = schemas.Enum
		res += "("
		opts := ""
		for v := range c.EnumOptions {
			opts += fmt.Sprintf(",'%v'", v)
		}
		res += strings.TrimLeft(opts, ",")
		res += ")"
	case schemas.Set: // xugu set
		res = schemas.Set
		res += "("
		opts := ""
		for v := range c.SetOptions {
			opts += fmt.Sprintf(",'%v'", v)
		}
		res += strings.TrimLeft(opts, ",")
		res += ")"
	case schemas.NVarchar:
		res = schemas.Varchar
	case schemas.Uuid:
		res = schemas.Varchar
		c.Length = 40
	case schemas.Json:
		res = schemas.Text
	case schemas.UnsignedInt:
		res = schemas.Int
		isUnsigned = true
	case schemas.UnsignedBigInt:
		res = schemas.BigInt
		isUnsigned = true
	case schemas.UnsignedMediumInt:
		res = schemas.MediumInt
		isUnsigned = true
	case schemas.UnsignedSmallInt:
		res = schemas.SmallInt
		isUnsigned = true
	case schemas.UnsignedTinyInt:
		res = schemas.TinyInt
		isUnsigned = true
	case schemas.Float:
		res = schemas.Float
	default:
		res = t
	}

	hasLen1 := c.Length > 0
	hasLen2 := c.Length2 > 0

	if hasLen2 {
		res += "(" + strconv.FormatInt(c.Length, 10) + "," + strconv.FormatInt(c.Length2, 10) + ")"
	} else if hasLen1 {
		res += "(" + strconv.FormatInt(c.Length, 10) + ")"
	}

	if isUnsigned {
		//res += " UNSIGNED"
	}

	return res
}

func (db *xugu) ColumnTypeKind(t string) int {
	switch strings.ToUpper(t) {
	case "DATETIME", "TIMESTAMP", "DATE", "TIME":
		return schemas.TIME_TYPE
	case "CHAR", "VARCHAR", "TINYTEXT", "TEXT", "MEDIUMTEXT", "LONGTEXT", "ENUM", "SET":
		return schemas.TEXT_TYPE
	case "BIGINT", "TINYINT", "SMALLINT", "MEDIUMINT", "INT", "FLOAT", "REAL", "DOUBLE PRECISION", "DECIMAL", "NUMERIC", "BIT":
		return schemas.NUMERIC_TYPE
	case "BINARY", "VARBINARY", "TINYBLOB", "BLOB", "MEDIUMBLOB", "LONGBLOB":
		return schemas.BLOB_TYPE
	default:
		return schemas.UNKNOW_TYPE
	}
}

func (db *xugu) IsReserved(name string) bool {
	_, ok := xuguReservedWords[strings.ToUpper(name)]
	return ok
}

func (db *xugu) AutoIncrStr() string {
	return "IDENTITY"
}

func (db *xugu) IndexCheckSQL(tableName, idxName string) (string, []interface{}) {
	args := []interface{}{tableName, idxName}
	sql := xuguCurrentScopeSQL + `
		SELECT i.INDEX_NAME FROM ALL_INDEXES i
		JOIN ALL_TABLES t ON i.DB_ID = t.DB_ID AND i.TABLE_ID = t.TABLE_ID
		JOIN CURRENT_SCOPE cs ON t.DB_ID = cs.DB_ID AND t.SCHEMA_ID = cs.SCHEMA_ID
		WHERE t.TABLE_NAME = ? AND i.INDEX_NAME = ?`
	return sql, args
}

func (db *xugu) IsTableExist(queryer core.Queryer, ctx context.Context, tableName string) (bool, error) {
	sql := xuguCurrentScopeSQL + `
		SELECT t.TABLE_NAME FROM ALL_TABLES t
		JOIN CURRENT_SCOPE cs ON t.DB_ID = cs.DB_ID AND t.SCHEMA_ID = cs.SCHEMA_ID
		WHERE t.TABLE_NAME = ?`
	return db.HasRecords(queryer, ctx, sql, tableName)
}

func (db *xugu) AddColumnSQL(tableName string, col *schemas.Column) string {
	quoter := db.Quoter()
	s, _ := dialects.ColumnString(db, col, true, false)
	sql := fmt.Sprintf("ALTER TABLE %v ADD %v", quoter.Quote(tableName), s)
	if len(col.Comment) > 0 {
		sql += " COMMENT '" + col.Comment + "'"
	}
	return sql
}

// ModifyColumnSQL overrides the base implementation to use our Quoter()
// instead of the unexported Base.quoter field.
func (db *xugu) ModifyColumnSQL(tableName string, col *schemas.Column) string {
	s, _ := dialects.ColumnString(db, col, false, false)
	return fmt.Sprintf("ALTER TABLE %v MODIFY COLUMN %v", db.Quoter().Quote(tableName), s)
}

func (db *xugu) GetColumns(queryer core.Queryer, ctx context.Context, tableName string) ([]string, map[string]*schemas.Column, error) {
	args := []interface{}{tableName}
	s := xuguCurrentScopeSQL + `
	SELECT   c1.COL_NAME '字段名', c1.NOT_NULL '是否空', c1.TYPE_NAME '字段类型', c1.IS_SERIAL '是否为序列值', c1.COMMENTS '注释',c1.SCALE '数据尺寸', c1.DEF_VAL '默认值', con1.DEFINE '约束定义', con1.CONS_TYPE'约束'
	FROM all_tables t1
	JOIN CURRENT_SCOPE cs ON t1.DB_ID = cs.DB_ID AND t1.SCHEMA_ID = cs.SCHEMA_ID
	JOIN all_columns c1 ON c1.DB_ID = t1.DB_ID AND c1.TABLE_ID = t1.TABLE_ID
	LEFT JOIN all_constraints con1 ON con1.DB_ID = t1.DB_ID AND con1.TABLE_ID = t1.TABLE_ID AND con1.define like '%"'||c1.col_name||'"%'
	WHERE t1.TABLE_NAME = ?;
`

	rows, err := queryer.QueryContext(ctx, s, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	cols := make(map[string]*schemas.Column)
	colSeq := make([]string, 0)

	for rows.Next() {
		col := new(schemas.Column)
		col.Indexes = make(map[string]int)

		var scale int64
		var colNaame, typeName string
		// XuguDB 12 exposes the column default expression as ALL_COLUMNS.DEF_VAL.
		// ALL_COLUMNS.DEFAULT_VALUE and ALL_COLUMNS.DEFINE do not exist on that server version.
		var comment, defaultValue, define, cons_type sql.NullString
		var notNUll, isSerial bool

		err = rows.Scan(&colNaame, &notNUll, &typeName, &isSerial, &comment, &scale, &defaultValue, &define, &cons_type)
		if err != nil {
			return nil, nil, err
		}
		col.Name = colNaame
		col.Comment = comment.String
		col.Nullable = !notNUll
		col.IsAutoIncrement = isSerial
		col.Default = strings.TrimSpace(defaultValue.String)
		col.DefaultIsEmpty = !defaultValue.Valid

		col.SQLType = schemas.SQLType{Name: db.Alias(typeName), DefaultLength: int64(scale), DefaultLength2: 0}
		col.Length = scale
		if cons_type.Valid {
			switch cons_type.String {
			case "P":
				col.IsPrimaryKey = true
			}
		}
		cols[col.Name] = col
		colSeq = append(colSeq, col.Name)
	}

	if rows.Err() != nil {
		return nil, nil, rows.Err()
	}
	return colSeq, cols, nil
}

func (db *xugu) GetTables(queryer core.Queryer, ctx context.Context) ([]*schemas.Table, error) {
	sql := xuguCurrentScopeSQL + `
		SELECT t.TABLE_NAME FROM ALL_TABLES t
		JOIN CURRENT_SCOPE cs ON t.DB_ID = cs.DB_ID AND t.SCHEMA_ID = cs.SCHEMA_ID`
	rows, err := queryer.QueryContext(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]*schemas.Table, 0)
	for rows.Next() {
		table := schemas.NewEmptyTable()
		var name string

		err = rows.Scan(&name)
		if err != nil {
			return nil, err
		}

		table.Name = name

		tables = append(tables, table)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return tables, nil
}

func (db *xugu) SetQuotePolicy(quotePolicy dialects.QuotePolicy) {
	switch quotePolicy {
	case dialects.QuotePolicyNone:
		q := xuguQuoter
		q.IsReserved = schemas.AlwaysNoReserve
		db.quoter = q
	case dialects.QuotePolicyReserved:
		q := xuguQuoter
		q.IsReserved = db.IsReserved
		db.quoter = q
	case dialects.QuotePolicyAlways:
		fallthrough
	default:
		db.quoter = xuguQuoter
	}
}

func (db *xugu) GetIndexes(queryer core.Queryer, ctx context.Context, tableName string) (map[string]*schemas.Index, error) {
	// XuguDB 12 stores an index's column list in ALL_INDEXES.KEYS; the
	// Oracle-compatible ALL_IND_COLUMNS view is not present in this version.
	args := []interface{}{tableName}
	s := xuguCurrentScopeSQL + `
		SELECT i.INDEX_NAME, i.KEYS, i.IS_UNIQUE, i.IS_PRIMARY
			FROM ALL_INDEXES i
			JOIN ALL_TABLES t ON i.DB_ID = t.DB_ID AND i.TABLE_ID = t.TABLE_ID
			JOIN CURRENT_SCOPE cs ON t.DB_ID = cs.DB_ID AND t.SCHEMA_ID = cs.SCHEMA_ID
			WHERE t.TABLE_NAME = ? AND i.INDEX_NAME NOT LIKE 'SYS%'
			ORDER BY i.INDEX_NAME`

	rows, err := queryer.QueryContext(ctx, s, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := make(map[string]*schemas.Index)
	for rows.Next() {
		var indexName, keys, uniqueness, primary string
		if err := rows.Scan(&indexName, &keys, &uniqueness, &primary); err != nil {
			return nil, err
		}
		switch strings.ToUpper(strings.TrimSpace(primary)) {
		case "TRUE", "YES", "1":
			continue
		}

		indexName = strings.TrimSpace(indexName)
		var indexType int
		switch strings.ToUpper(strings.TrimSpace(uniqueness)) {
		case "UNIQUE", "TRUE", "YES", "1":
			indexType = schemas.UniqueType
		default:
			indexType = schemas.IndexType
		}
		idx, ok := indexes[indexName]
		if !ok {
			idx = new(schemas.Index)
			idx.Name = indexName
			idx.Type = indexType
			indexes[indexName] = idx
		}
		columns, err := parseXuguIndexKeys(keys)
		if err != nil {
			return nil, fmt.Errorf("parse index %q keys: %w", indexName, err)
		}
		for _, column := range columns {
			idx.AddColumn(column)
		}
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return indexes, nil
}

func parseXuguIndexKeys(keys string) ([]string, error) {
	var columns []string
	for pos := 0; ; {
		for pos < len(keys) && strings.ContainsRune(" \t\r\n", rune(keys[pos])) {
			pos++
		}
		if pos == len(keys) {
			if len(columns) == 0 {
				return nil, errors.New("empty KEYS value")
			}
			return columns, nil
		}
		if keys[pos] != '"' {
			return nil, fmt.Errorf("unsupported KEYS encoding at byte %d", pos)
		}
		pos++
		var column strings.Builder
		closed := false
		for pos < len(keys) {
			if keys[pos] != '"' {
				column.WriteByte(keys[pos])
				pos++
				continue
			}
			if pos+1 < len(keys) && keys[pos+1] == '"' {
				column.WriteByte('"')
				pos += 2
				continue
			}
			pos++
			closed = true
			break
		}
		if !closed || column.Len() == 0 {
			return nil, errors.New("malformed quoted identifier in KEYS")
		}
		columns = append(columns, column.String())
		for pos < len(keys) && strings.ContainsRune(" \t\r\n", rune(keys[pos])) {
			pos++
		}
		if pos == len(keys) {
			return columns, nil
		}
		if keys[pos] != ',' {
			return nil, fmt.Errorf("unsupported KEYS suffix at byte %d", pos)
		}
		pos++
	}
}

func (db *xugu) ColumnString(d dialects.Dialect, col *schemas.Column, includePrimaryKey bool) (string, error) {
	bd := strings.Builder{}

	if err := d.Quoter().QuoteTo(&bd, col.Name); err != nil {
		return "", err
	}

	if err := bd.WriteByte(' '); err != nil {
		return "", err
	}

	if _, err := bd.WriteString(d.SQLType(col)); err != nil {
		return "", err
	}

	if includePrimaryKey && col.IsPrimaryKey {
		if col.IsAutoIncrement {
			if err := bd.WriteByte(' '); err != nil {
				return "", err
			}
			if _, err := bd.WriteString(d.AutoIncrStr()); err != nil {
				return "", err
			}
		}
	}

	if !col.DefaultIsEmpty {
		if _, err := bd.WriteString(" DEFAULT "); err != nil {
			return "", err
		}
		if col.Default == "" {
			if _, err := bd.WriteString("''"); err != nil {
				return "", err
			}
		} else {
			if _, err := bd.WriteString(col.Default); err != nil {
				return "", err
			}
		}
	}

	if col.Nullable {
		if _, err := bd.WriteString(" NULL"); err != nil {
			return "", err
		}
	} else {
		if _, err := bd.WriteString(" NOT NULL"); err != nil {
			return "", err
		}
	}

	return bd.String(), nil
}

func (db *xugu) CreateTableSQL(ctx context.Context, queryer core.Queryer, table *schemas.Table, tableName string) (string, bool, error) {
	if tableName == "" {
		tableName = table.Name
	}

	quoter := db.Quoter()
	var b strings.Builder
	if _, err := b.WriteString("CREATE TABLE "); err != nil {
		return "", false, err
	}
	if err := quoter.QuoteTo(&b, tableName); err != nil {
		return "", false, err
	}
	if _, err := b.WriteString(" ("); err != nil {
		return "", false, err
	}

	pkList := table.PrimaryKeys

	for i, colName := range table.ColumnsSeq() {
		col := table.GetColumn(colName)
		if col.SQLType.IsBool() && !col.DefaultIsEmpty {
			if col.Default == "true" {
				col.Default = "1"
			} else if col.Default == "false" {
				col.Default = "0"
			}
		}
		s, err := db.ColumnString(db, col, col.IsPrimaryKey && len(table.PrimaryKeys) == 1)
		if err != nil {
			return "", false, err
		}
		if _, err := b.WriteString(s); err != nil {
			return "", false, err
		}
		if i != len(table.ColumnsSeq())-1 {
			if _, err := b.WriteString(", "); err != nil {
				return "", false, err
			}
		}
	}

	if len(pkList) > 0 {
		if len(table.ColumnsSeq()) > 0 {
			if _, err := b.WriteString(", "); err != nil {
				return "", false, err
			}
		}
		if _, err := b.WriteString("CONSTRAINT PK_"); err != nil {
			return "", false, err
		}
		if _, err := b.WriteString(tableName); err != nil {
			return "", false, err
		}
		if _, err := b.WriteString(" PRIMARY KEY ("); err != nil {
			return "", false, err
		}
		if err := quoter.JoinWrite(&b, pkList, ","); err != nil {
			return "", false, err
		}
		if _, err := b.WriteString(")"); err != nil {
			return "", false, err
		}
	}
	if _, err := b.WriteString(")"); err != nil {
		return "", false, err
	}

	return b.String(), false, nil
}

func (db *xugu) Filters() []dialects.Filter {
	return []dialects.Filter{}
}

type xuguDriver struct{}

func (p *xuguDriver) Scan(ctx *dialects.ScanContext, rows *core.Rows, types []*sql.ColumnType, v ...interface{}) error {
	return rows.Scan(v...)
}

func (p *xuguDriver) Features() *dialects.DriverFeatures {
	return &dialects.DriverFeatures{
		SupportReturnInsertedID: true,
	}
}

func (p *xuguDriver) Parse(driverName, dataSourceName string) (*dialects.URI, error) {
	uri := &dialects.URI{DBType: "xugusql"}
	pairs := strings.Split(dataSourceName, ";")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, value := strings.TrimSpace(kv[0]), strings.Trim(strings.TrimSpace(kv[1]), "'")
		keyL := strings.ToLower(key)

		switch keyL {
		case "ip":
			uri.Host = value
		case "port":
			uri.Port = value
		case "db":
			uri.DBName = value
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

func (p *xuguDriver) GenScanResult(colType string) (interface{}, error) {
	colType = strings.Replace(colType, "UNSIGNED ", "", -1)
	switch colType {
	case "CHAR", "VARCHAR", "TINYTEXT", "TEXT", "MEDIUMTEXT", "LONGTEXT", "ENUM", "SET", "JSON":
		var s sql.NullString
		return &s, nil
	case "BIGINT":
		var s sql.NullInt64
		return &s, nil
	case "TINYINT", "SMALLINT", "MEDIUMINT", "INT":
		var s sql.NullInt32
		return &s, nil
	case "FLOAT", "REAL", "DOUBLE PRECISION", "DOUBLE":
		var s sql.NullFloat64
		return &s, nil
	case "DECIMAL", "NUMERIC":
		var s sql.NullString
		return &s, nil
	case "DATETIME", "TIMESTAMP":
		var s sql.NullTime
		return &s, nil
	case "BIT":
		var s sql.RawBytes
		return &s, nil
	case "BINARY", "VARBINARY", "TINYBLOB", "BLOB", "MEDIUMBLOB", "LONGBLOB":
		var r sql.RawBytes
		return &r, nil
	default:
		var r sql.RawBytes
		return &r, nil
	}
}
