// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"strconv"
	"strings"

	"xorm.io/xorm/core"
	"xorm.io/xorm/schemas"
)

func init() {
	RegisterDriver("xugu", &xuguDriver{})
	RegisterDialect("xugusql", func() Dialect {
		return &xugu{}
	})
}

var (
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
		"ON	OPTIMIZE": true, "OPTION": true,
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

type xugu struct {
	Base
	rowFormat string
}

func (db *xugu) Init(uri *URI) error {
	db.quoter = xuguQuoter
	return db.Base.Init(db, uri)
}

var xuguColAliases = map[string]string{
	"numeric": "decimal",
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
	rows, err := queryer.QueryContext(ctx, "SELECT VERSION;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var version string
	if !rows.Next() {
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		return nil, errors.New("unknow version")
	}

	if err := rows.Scan(&version); err != nil {
		return nil, err
	}

	fields := strings.Split(version, " ")

	var edition string
	if len(fields) == 2 {
		edition = fields[0]
	}

	return &schemas.Version{
		Number:  fields[1],
		Edition: edition,
	}, nil
}

func (db *xugu) Features() *DialectFeatures {
	return &DialectFeatures{
		AutoincrMode: IncrAutoincrMode,
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
		isUnsigned = true
	default:
		res = t
	}

	hasLen1 := c.Length > 0
	hasLen2 := c.Length2 > 0

	if res == schemas.BigInt && !hasLen1 && !hasLen2 {
		//c.Length = 20
		hasLen1 = false
	}

	if hasLen2 {
		res += "(" + strconv.Itoa(c.Length) + "," + strconv.Itoa(c.Length2) + ")"
	} else if hasLen1 {
		res += "(" + strconv.Itoa(c.Length) + ")"
	}

	if isUnsigned {
		//res += " UNSIGNED"
	}

	return res
}

func (db *xugu) ColumnTypeKind(t string) int {
	switch strings.ToUpper(t) {
	case "DATETIME":
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
	args := []interface{}{db.uri.DBName, tableName, idxName}
	sql := `SELECT  INDEX_NAME FROM ALL_INDEXES i  
		JOIN ALL_TABLEs t ON i.TABLE_ID = t.TABLE_ID
		WHERE t.table_name = ? AND index_name = ?;`
	return sql, args
}

func (db *xugu) IsTableExist(queryer core.Queryer, ctx context.Context, tableName string) (bool, error) {
	//sql := "SELECT `TABLE_NAME` from `INFORMATION_SCHEMA`.`TABLES` WHERE `TABLE_SCHEMA`=? and `TABLE_NAME`=?"
	sql := `
	SELECT TABLE_NAME,* from all_tables t 
	JOIN all_schemas s ON t.schema_id = s.schema_id
	WHERE  s.SCHEMA_NAME = ? and t.TABLE_NAME = ?;
	`
	return db.HasRecords(queryer, ctx, sql, db.uri.DBName, tableName)
}

func (db *xugu) AddColumnSQL(tableName string, col *schemas.Column) string {
	quoter := db.dialect.Quoter()
	s, _ := ColumnString(db, col, true)
	sql := fmt.Sprintf("ALTER TABLE %v ADD %v", quoter.Quote(tableName), s)
	if len(col.Comment) > 0 {
		sql += " COMMENT '" + col.Comment + "'"
	}
	return sql
}

func (db *xugu) GetColumns(queryer core.Queryer, ctx context.Context, tableName string) ([]string, map[string]*schemas.Column, error) {
	args := []interface{}{db.uri.DBName, tableName}
	s := `
SELECT   c1.COL_NAME '字段名', c1.NOT_NULL '是否空', c1.TYPE_NAME '字段类型', c1.IS_SERIAL '是否为序列值', c1.COMMENTS '注释',c1.SCALE '数据尺寸' ,con1.DEFINE '约束定义', con1.CONS_TYPE'约束'  
FROM all_tables t1 
JOIN all_columns c1   ON  c1.TABLE_ID = t1.TABLE_ID 
LEFT JOIN   all_constraints con1 ON con1.table_id =  t1.TABLE_ID AND  con1.define  like '%"'||c1.col_name||'"%' 
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
		var colNaame, typeName, comment, define, cons_type string
		var notNUll, isSerial bool

		err = rows.Scan(&colNaame, &notNUll, &typeName, &isSerial, &comment, &scale, &define, &cons_type)
		if err != nil {
			return nil, nil, err
		}
		col.Name = colNaame
		col.Comment = comment
		if notNUll {
			col.Nullable = true
		}

		col.SQLType = schemas.SQLType{Name: typeName, DefaultLength: int(scale), DefaultLength2: 0}
		switch cons_type {
		case "P":
			col.IsPrimaryKey = true

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
	args := []interface{}{db.uri.DBName}
	sql := "SELECT `TABLE_NAME`, `ENGINE`, `AUTO_INCREMENT`, `TABLE_COMMENT`, `TABLE_COLLATION` from " +
		"`INFORMATION_SCHEMA`.`TABLES` WHERE `TABLE_SCHEMA`=? AND (`ENGINE`='MyISAM' OR `ENGINE` = 'InnoDB' OR `ENGINE` = 'TokuDB')"
	fmt.Println("GetTables db.uri.DBName", db.uri.DBName)
	sql = `
		SELECT TABLE_NAME FROM ALL_TABLES;
	`
	rows, err := queryer.QueryContext(ctx, sql, args...)
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

func (db *xugu) SetQuotePolicy(quotePolicy QuotePolicy) {
	switch quotePolicy {
	case QuotePolicyNone:
		q := xuguQuoter
		q.IsReserved = schemas.AlwaysNoReserve
		db.quoter = q
	case QuotePolicyReserved:
		q := xuguQuoter
		q.IsReserved = db.IsReserved
		db.quoter = q
	case QuotePolicyAlways:
		fallthrough
	default:
		db.quoter = xuguQuoter
	}
}

func (db *xugu) GetIndexes(queryer core.Queryer, ctx context.Context, tableName string) (map[string]*schemas.Index, error) {
	//	args := []interface{}{tableName}
	fmt.Println("GetIndexes tableName", tableName)
	//	s := "SELECT `INDEX_NAME`, `NON_UNIQUE`, `COLUMN_NAME` FROM `INFORMATION_SCHEMA`.`STATISTICS` WHERE `TABLE_SCHEMA` = ? AND `TABLE_NAME` = ? ORDER BY `SEQ_IN_INDEX`"

	// 	s := `
	// SELECT  i1.index_name,IS_UNIQUE,* FROM ALL_TABLEs t1
	// LEFT JOIN ALL_INDEXES i1 ON t1.TABLE_ID = i1.TABLE_ID
	// WHERE t1.TABLE_NAME = ?;
	// `
	// 	rows, err := queryer.QueryContext(ctx, s, args...)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	defer rows.Close()

	// 	indexes := make(map[string]*schemas.Index)
	// 	for rows.Next() {
	// 		var indexType int
	// 		var indexName, colName string
	// 		var nonUnique bool
	// 		err = rows.Scan(&indexName, &nonUnique, colName)
	// 		if err != nil {
	// 			return nil, err
	// 		}

	// 		if nonUnique {
	// 			indexType = schemas.UniqueType
	// 		} else {
	// 			indexType = schemas.IndexType
	// 		}
	// 		colNames := strings.Split(colName, ",")
	// 		var index *schemas.Index
	// 		var ok bool
	// 		if index, ok = indexes[indexName]; !ok {
	// 			index = new(schemas.Index)
	// 			index.IsRegular = isRegular
	// 			index.Type = indexType
	// 			index.Name = indexName
	// 			indexes[indexName] = index
	// 		}
	// 		index.AddColumn(colName)
	// 		// colName = strings.Trim(colName, "` ")
	// 		// var isRegular bool
	// 		// if strings.HasPrefix(indexName, "IDX_"+tableName) || strings.HasPrefix(indexName, "UQE_"+tableName) {
	// 		// 	indexName = indexName[5+len(tableName):]
	// 		// 	isRegular = true
	// 		// }

	// 		var index *schemas.Index
	// 		// var ok bool
	// 		// if index, ok = indexes[indexName]; !ok {
	// 		// 	index = new(schemas.Index)
	// 		// 	index.IsRegular = isRegular
	// 		index.Type = indexType
	// 		// 	index.Name = indexName
	// 		// 	indexes[indexName] = index
	// 		// }
	// 		// index.AddColumn(colName)
	// 	}
	// 	if rows.Err() != nil {
	// 		return nil, rows.Err()
	// 	}
	indexes := make(map[string]*schemas.Index)
	return indexes, nil
}

func (db *xugu) ColumnString(dialect Dialect, col *schemas.Column, includePrimaryKey bool) (string, error) {
	bd := strings.Builder{}

	if err := dialect.Quoter().QuoteTo(&bd, col.Name); err != nil {
		return "", err
	}

	if err := bd.WriteByte(' '); err != nil {
		return "", err
	}

	if _, err := bd.WriteString(dialect.SQLType(col)); err != nil {
		return "", err
	}

	if includePrimaryKey && col.IsPrimaryKey {
		if col.IsAutoIncrement {
			if err := bd.WriteByte(' '); err != nil {
				return "", err
			}
			if _, err := bd.WriteString(dialect.AutoIncrStr()); err != nil {
				return "", err
			}
		}
		// if _, err := bd.WriteString(" PRIMARY KEY"); err != nil {
		// 	return "", err
		// }

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
		s, _ := db.ColumnString(db.dialect, col, col.IsPrimaryKey && len(table.PrimaryKeys) == 1)
		// s, _ := ColumnString(db, col, false)
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

func (db *xugu) Filters() []Filter {
	return []Filter{}
}

type xuguDriver struct {
	baseDriver
}

func (p *xuguDriver) Features() *DriverFeatures {
	return &DriverFeatures{
		SupportReturnInsertedID: true,
	}
}

func (p *xuguDriver) Parse(driverName, dataSourceName string) (*URI, error) {
	uri := &URI{DBType: "xugusql"}
	pairs := strings.Split(dataSourceName, ";")
	// Iterate over the pairs and map them to the struct fields
	for _, pair := range pairs {
		// Split each pair by the equals sign
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, value := strings.TrimSpace(kv[0]), strings.Trim(strings.TrimSpace(kv[1]), "'")
		keyL := strings.ToLower(key)

		// Map the key to the appropriate struct field
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
