package database

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Dominik-Friedrich/tables-to-go/v2/pkg/settings"

	// postgres database driver
	_ "github.com/lib/pq"
)

// Postgresql implements the Database interface with help of GeneralDatabase.
type Postgresql struct {
	*GeneralDatabase

	defaultUserName string
}

// NewPostgresql creates a new Postgresql database.
func NewPostgresql(s *settings.Settings) *Postgresql {
	return &Postgresql{
		GeneralDatabase: &GeneralDatabase{
			Settings: s,
			driver:   dbTypeToDriverMap[s.DbType],
		},
		defaultUserName: "postgres",
	}
}

// Connect connects to the database by the given data source name (dsn) of the
// concrete database.
func (pg *Postgresql) Connect() error {
	return pg.GeneralDatabase.Connect(pg.DSN())
}

// DSN creates the DSN String to connect to this database.
func (pg *Postgresql) DSN() string {
	user := pg.defaultUserName
	if pg.Settings.User != "" {
		user = pg.Settings.User
	}
	if pg.Settings.Socket != "" {
		return fmt.Sprintf("postgres://%s:%s@?%s&%s&sslmode=%s",
			user, pg.Settings.Pswd, pg.Settings.Socket, pg.Settings.Port, pg.Settings.SSLMode)
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, pg.Settings.Pswd, pg.Settings.Host, pg.Settings.Port, pg.Settings.DbName, pg.Settings.SSLMode)
}

// GetTables gets all tables for a given schema by name.
func (pg *Postgresql) GetTables(tables ...string) ([]*Table, error) {

	args := []any{pg.Schema}
	in := pg.andInClause("LOWER(table_name)", tables, &args)

	var dbTables []*Table
	err := pg.Select(&dbTables, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_type = 'BASE TABLE'
		AND table_schema = $1
		`+in+`
		ORDER BY table_name
	`, args...)

	if pg.Verbose {
		if err != nil {
			fmt.Println("> Error at GetTables()")
			fmt.Printf("> schema: %q\r\n", pg.Schema)
		}
	}

	return dbTables, err
}

// PrepareGetColumnsOfTableStmt prepares the statement for retrieving the
// columns of a specific table for a given database.
func (pg *Postgresql) PrepareGetColumnsOfTableStmt() (err error) {

	pg.GetColumnsOfTableStmt, err = pg.Preparex(`
		SELECT
			ic.ordinal_position,
			ic.column_name,
			ic.data_type,
			ic.column_default,
			ic.is_nullable,
			ic.character_maximum_length,
			ic.numeric_precision,
			itc.constraint_name,
			itc.constraint_type
		FROM information_schema.columns AS ic
			LEFT JOIN information_schema.key_column_usage AS ikcu ON ic.table_name = ikcu.table_name
			AND ic.table_schema = ikcu.table_schema
			AND ic.column_name = ikcu.column_name
			LEFT JOIN information_schema.table_constraints AS itc ON ic.table_name = itc.table_name
			AND ic.table_schema = itc.table_schema
			AND ikcu.constraint_name = itc.constraint_name
		WHERE ic.table_name = $1
		AND ic.table_schema = $2
		ORDER BY ic.ordinal_position
	`)

	return err
}

// GetColumnsOfTable executes the statement for retrieving the columns of a
// specific table in a given schema.
func (pg *Postgresql) GetColumnsOfTable(table *Table) (err error) {

	err = pg.GetColumnsOfTableStmt.Select(&table.Columns, table.Name, pg.Schema)

	if pg.Verbose {
		if err != nil {
			fmt.Printf("> Error at GetColumnsOfTable(%v)\r\n", table.Name)
			fmt.Printf("> schema: %q\r\n", pg.Schema)
		}
	}

	return err
}

// IsPrimaryKey checks if the column belongs to the primary key.
func (pg *Postgresql) IsPrimaryKey(column Column) bool {
	return strings.Contains(column.ConstraintType.String, "PRIMARY KEY")
}

// IsAutoIncrement checks if the column is an auto_increment column.
func (pg *Postgresql) IsAutoIncrement(column Column) bool {
	return strings.Contains(column.DefaultValue.String, "nextval")
}

// GetStringDatatypes returns the string datatypes for the Postgresql database.
func (pg *Postgresql) GetStringDatatypes() []string {
	return []string{
		"character varying",
		"varchar",
		"character",
		"char",
		"uuid",
	}
}

// IsString returns true if colum is of type string for the Postgresql database.
func (pg *Postgresql) IsString(column Column) bool {
	return isStringInSlice(column.DataType, pg.GetStringDatatypes())
}

// GetTextDatatypes returns the text datatypes for the Postgresql database.
func (pg *Postgresql) GetTextDatatypes() []string {
	return []string{
		"text",
	}
}

// IsText returns true if colum is of type text for the Postgresql database.
func (pg *Postgresql) IsText(column Column) bool {
	return isStringInSlice(column.DataType, pg.GetTextDatatypes())
}

// GetIntegerDatatypes returns the integer datatypes for the Postgresql database.
func (pg *Postgresql) GetIntegerDatatypes() []string {
	return []string{
		"smallint",
		"integer",
		"bigint",
		"smallserial",
		"serial",
		"bigserial",
	}
}

// IsInteger returns true if colum is of type integer for the Postgresql database.
func (pg *Postgresql) IsInteger(column Column) bool {
	return isStringInSlice(column.DataType, pg.GetIntegerDatatypes())
}

// GetFloatDatatypes returns the float datatypes for the Postgresql database.
func (pg *Postgresql) GetFloatDatatypes() []string {
	return []string{
		"numeric",
		"decimal",
		"real",
		"double precision",
	}
}

// IsFloat returns true if colum is of type float for the Postgresql database.
func (pg *Postgresql) IsFloat(column Column) bool {
	return isStringInSlice(column.DataType, pg.GetFloatDatatypes())
}

// GetTemporalDatatypes returns the temporal datatypes for the Postgresql database.
func (pg *Postgresql) GetTemporalDatatypes() []string {
	return []string{
		"time",
		"timestamp",
		"time with time zone",
		"timestamp with time zone",
		"time without time zone",
		"timestamp without time zone",
		"date",
	}
}

// IsTemporal returns true if colum is of type temporal for the Postgresql database.
func (pg *Postgresql) IsTemporal(column Column) bool {
	return isStringInSlice(column.DataType, pg.GetTemporalDatatypes())
}

func (*Postgresql) andInClause(field string, params []string, args *[]any) string {
	if field == "" || len(params) == 0 {
		return ""
	}
	narg := len(*args)
	nparam := len(params)
	last := nparam - 1

	var sb strings.Builder
	sb.Grow(nparam*3 - 1) // 3 ==> len("$n,") == 3 and -1 for the skipped last comma
	for i := 1; i <= nparam; i++ {
		n := strconv.Itoa(narg + i)
		sb.WriteString("$" + n)
		if i <= last {
			sb.WriteString(",")
		}
	}

	*args = slices.Grow(*args, nparam)
	for i := range params {
		*args = append(*args, strings.ToLower(params[i]))
	}

	return "AND " + field + " IN (" + sb.String() + ")"
}
