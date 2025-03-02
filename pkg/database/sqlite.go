package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/Dominik-Friedrich/tables-to-go/v2/pkg/settings"
)

// SQLite implements the Database interface with help of GeneralDatabase.
type SQLite struct {
	*GeneralDatabase
}

// NewSQLite creates a new SQLite database.
func NewSQLite(s *settings.Settings) *SQLite {
	return &SQLite{
		GeneralDatabase: &GeneralDatabase{
			Settings: s,
			driver:   dbTypeToDriverMap[s.DbType],
		},
	}
}

// Connect connects to the database by the given data source name (dsn) of the
// concrete database.
func (s *SQLite) Connect() (err error) {
	return s.GeneralDatabase.Connect(s.DSN())
}

// DSN creates the DSN String to connect to this database.
func (s *SQLite) DSN() string {
	if s.Settings.User == "" && s.Settings.Pswd == "" {
		return s.Settings.DbName
	}

	u, err := url.Parse(s.DbName)
	if err != nil {
		return s.Settings.DbName
	}

	query := u.Query()
	query.Set("_auth_user", s.Settings.User)
	query.Set("_auth_pass", s.Settings.Pswd)
	u.RawQuery = query.Encode()

	// SQLite driver expects a empty `_auth` request param
	return strings.ReplaceAll(u.RequestURI(), "_auth=&", "_auth&")
}

func (s *SQLite) GetTables(tables ...string) ([]*Table, error) {

	var args []any
	in := s.andInClause("name", tables, &args)

	var dbTables []*Table
	err := s.Select(&dbTables, `
		SELECT name AS table_name
		FROM sqlite_master
		WHERE type = 'table'
		AND name NOT LIKE 'sqlite?_%' ESCAPE '?'
		`+in+`
	`, args...)

	if s.Verbose {
		if err != nil {
			fmt.Println("> Error at GetTables()")
			fmt.Printf("> database: %q\r\n", s.DbName)
		}
	}

	return dbTables, err
}

func (s *SQLite) PrepareGetColumnsOfTableStmt() (err error) {
	return nil
}

func (s *SQLite) GetColumnsOfTable(table *Table) (err error) {

	rows, err := s.Queryx(`
		SELECT * 
		FROM PRAGMA_TABLE_INFO('` + table.Name + `')
	`)
	if err != nil {
		if s.Verbose {
			fmt.Printf("> Error at GetColumnsOfTable(%v)\r\n", table.Name)
			fmt.Printf("> database: %q\r\n", s.DbName)
		}
		return err
	}

	type column struct {
		CID          int            `db:"cid"`
		Name         string         `db:"name"`
		DataType     string         `db:"type"`
		NotNull      int            `db:"notnull"`
		DefaultValue sql.NullString `db:"dflt_value"`
		PrimaryKey   int            `db:"pk"`
	}

	for rows.Next() {
		var col column
		err = rows.StructScan(&col)
		if err != nil {
			return err
		}

		isNullable := "YES"
		if col.NotNull == 1 {
			isNullable = "NO"
		}

		isPrimaryKey := ""
		if col.PrimaryKey == 1 {
			isPrimaryKey = "PK"
		}

		table.Columns = append(table.Columns, Column{
			OrdinalPosition:        col.CID,
			Name:                   col.Name,
			DataType:               col.DataType,
			DefaultValue:           col.DefaultValue,
			IsNullable:             isNullable,
			CharacterMaximumLength: sql.NullInt64{},
			NumericPrecision:       sql.NullInt64{},
			// reuse mysql column_key as primary key indicator
			ColumnKey:      isPrimaryKey,
			Extra:          "",
			ConstraintName: sql.NullString{},
			ConstraintType: sql.NullString{},
		})
	}

	return nil
}

func (s *SQLite) IsPrimaryKey(column Column) bool {
	return column.ColumnKey == "PK"
}

func (s *SQLite) IsAutoIncrement(column Column) bool {
	return column.ColumnKey == "PK"
}

func (s *SQLite) GetStringDatatypes() []string {
	return []string{
		"text",
	}
}

func (s *SQLite) IsString(column Column) bool {
	return isStringInSlice(column.DataType, s.GetStringDatatypes())
}

func (s *SQLite) GetTextDatatypes() []string {
	return []string{
		"text",
	}
}

func (s *SQLite) IsText(column Column) bool {
	return isStringInSlice(column.DataType, s.GetTextDatatypes())
}

func (s *SQLite) GetIntegerDatatypes() []string {
	return []string{
		"integer",
	}
}

func (s *SQLite) IsInteger(column Column) bool {
	return isStringInSlice(column.DataType, s.GetIntegerDatatypes())
}

func (s *SQLite) GetFloatDatatypes() []string {
	return []string{
		"real",
		"numeric",
	}
}

func (s *SQLite) IsFloat(column Column) bool {
	return isStringInSlice(column.DataType, s.GetFloatDatatypes())
}

func (s *SQLite) GetTemporalDatatypes() []string {
	return []string{}
}

func (s *SQLite) IsTemporal(_ Column) bool {
	return false
}
