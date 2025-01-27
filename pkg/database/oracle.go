package database

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Dominik-Friedrich/tables-to-go/v2/pkg/settings"
	_ "github.com/godror/godror"
	go_ora "github.com/sijms/go-ora/v2"
)

// Oracle implements the Database interface with help of GeneralDatabase.
type Oracle struct {
	*GeneralDatabase

	defaultUserName string
}

// NewOracle creates a new Oracle database handler.
func NewOracle(s *settings.Settings) *Oracle {
	return &Oracle{
		GeneralDatabase: &GeneralDatabase{
			Settings: s,
			driver:   dbTypeToDriverMap[s.DbType],
		},
		defaultUserName: "system",
	}
}

// DSN creates the DSN string to connect to Oracle.
// Example format: user/password@host:port/servicename
// Adjust as needed for your driver or TNS usage.
func (o *Oracle) DSN() string {
	user := o.defaultUserName
	if o.Settings.User != "" {
		user = o.Settings.User
	}

	urlOptions := make(map[string]string)

	port, err := strconv.Atoi(o.Settings.Port)
	if err != nil {
		panic(err)
	}

	connectionString := go_ora.BuildUrl(o.Settings.Host, port, o.Settings.DbName, user, o.Settings.Pswd, urlOptions)

	return connectionString
}

// Connect connects to the database using the DSN generated above.
func (o *Oracle) Connect() error {
	return o.GeneralDatabase.Connect(o.DSN())
}

// Close closes the database connection.
func (o *Oracle) Close() error {
	return o.DB.Close()
}

// GetTables retrieves all tables for the current (or specified) schema.
// If `tables...` is provided, it filters by those table names.
func (o *Oracle) GetTables(tables ...string) ([]*Table, error) {
	// If the user didn't supply a specific schema, assume the connected user
	owner := o.Settings.Schema
	if owner == "" {
		owner = o.Settings.User
	}
	owner = strings.ToUpper(owner)

	args := []any{owner}
	inClause := ""
	if len(tables) > 0 {
		placeholders := make([]string, 0, len(tables))
		for i, tbl := range tables {
			placeholders = append(placeholders, ":v"+strconv.Itoa(i))
			args = append(args, strings.ToUpper(tbl))
		}
		inClause = "AND OBJECT_NAME IN (" + strings.Join(placeholders, ",") + ")"
	}

	query := fmt.Sprintf(`
SELECT DISTINCT OBJECT_NAME as "table_name"
FROM ALL_OBJECTS
WHERE OBJECT_TYPE = 'TABLE'
AND OWNER = :owner
%s
ORDER BY OBJECT_NAME
	`, inClause)

	var dbTables []*Table
	err := o.Select(&dbTables, query, args...)
	if err != nil && o.Settings.Verbose {
		fmt.Println("> Error at GetTables()")
		fmt.Printf("> owner: %q\n", owner)
	}
	return dbTables, err
}

// PrepareGetColumnsOfTableStmt prepares a statement to retrieve columns
// (including information about primary keys) for a specific table.
func (o *Oracle) PrepareGetColumnsOfTableStmt() error {
	// We use a LEFT JOIN to label columns in the primary key as "PRI" in column_key.
	query := `
SELECT
    c.column_id AS "ordinal_position",
    c.column_name AS "column_name",
    c.data_type AS "data_type",
    c.data_default AS "column_default",
    c.nullable AS "is_nullable",
    c.data_length AS "character_maximum_length",
    c.data_precision AS "numeric_precision"
FROM USER_TAB_COLUMNS c
WHERE table_name = :name
`
	var err error
	o.GetColumnsOfTableStmt, err = o.Preparex(query)
	return err
}

// GetColumnsOfTable executes the prepared statement to retrieve column metadata.
func (o *Oracle) GetColumnsOfTable(table *Table) error {
	owner := o.Settings.Schema
	if owner == "" {
		owner = o.Settings.User
	}

	err := o.GetColumnsOfTableStmt.Select(
		&table.Columns,
		table.Name,
	)
	if err != nil && o.Settings.Verbose {
		fmt.Printf("> Error at GetColumnsOfTable(%v)\n", table.Name)
		fmt.Printf("> owner: %q\n", owner)
		fmt.Printf("> dbName: %q\n", o.DbName)
	}
	return err
}

// IsPrimaryKey checks if a column belongs to the primary key.
func (o *Oracle) IsPrimaryKey(column Column) bool {
	return strings.Contains(column.ColumnKey, "PRI")
}

// IsAutoIncrement checks if a column is auto-increment. Oracle does not have
// the same concept; typically sequences & triggers are used, so this is false.
func (o *Oracle) IsAutoIncrement(column Column) bool {
	return false
}

// IsNullable returns whether the column is nullable ('Y' for Oracle).
func (o *Oracle) IsNullable(column Column) bool {
	return column.IsNullable == "Y"
}

// GetStringDatatypes returns which datatypes Oracle generally treats as "string".
func (o *Oracle) GetStringDatatypes() []string {
	return []string{
		"CHAR",
		"VARCHAR2",
		"NCHAR",
		"NVARCHAR2",
	}
}

// IsString checks if a column is treated as a "string" type in Oracle.
func (o *Oracle) IsString(column Column) bool {
	return isStringInSlice(strings.ToUpper(column.DataType), o.GetStringDatatypes())
}

// GetTextDatatypes returns which datatypes Oracle generally treats as "text/clob".
func (o *Oracle) GetTextDatatypes() []string {
	return []string{
		"CLOB",
		"NCLOB",
	}
}

// IsText checks if a column is treated as a "text/clob" type in Oracle.
func (o *Oracle) IsText(column Column) bool {
	return isStringInSlice(strings.ToUpper(column.DataType), o.GetTextDatatypes())
}

// GetIntegerDatatypes returns which datatypes Oracle generally treats as "integer".
func (o *Oracle) GetIntegerDatatypes() []string {
	return []string{
		"NUMBER",   // often an integer if scale=0, but naive check here
		"INTEGER",  // Oracle synonym
		"SMALLINT", // Oracle synonym
	}
}

// IsInteger checks if a column is treated as an integer type in Oracle.
func (o *Oracle) IsInteger(column Column) bool {
	return isStringInSlice(strings.ToUpper(column.DataType), o.GetIntegerDatatypes())
}

// GetFloatDatatypes returns which datatypes Oracle generally treats as "floating".
func (o *Oracle) GetFloatDatatypes() []string {
	return []string{
		"FLOAT",
		"BINARY_FLOAT",
		"BINARY_DOUBLE",
		"DECIMAL",
		"NUMBER",
		"REAL",
		"DOUBLE PRECISION",
	}
}

// IsFloat checks if a column is treated as a floating-point type in Oracle.
func (o *Oracle) IsFloat(column Column) bool {
	return isStringInSlice(strings.ToUpper(column.DataType), o.GetFloatDatatypes())
}

// GetTemporalDatatypes returns which datatypes Oracle generally treats as "temporal".
func (o *Oracle) GetTemporalDatatypes() []string {
	return []string{
		"DATE",
		"TIMESTAMP",
		"TIMESTAMP WITH TIME ZONE",
		"TIMESTAMP WITH LOCAL TIME ZONE",
	}
}

// IsTemporal checks if a column is treated as a temporal/date/time type in Oracle.
func (o *Oracle) IsTemporal(column Column) bool {
	return isStringInSlice(strings.ToUpper(column.DataType), o.GetTemporalDatatypes())
}
