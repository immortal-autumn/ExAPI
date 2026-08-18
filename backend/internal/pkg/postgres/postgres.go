// Package postgres centralizes the PostgreSQL database/sql driver integration.
package postgres

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/stdlib"
)

// DriverName is the database/sql registration used by pgx's standard-library adapter.
const DriverName = "pgx"

// NewConnector parses both libpq-style keyword DSNs and PostgreSQL URLs.
func NewConnector(dsn string) (driver.Connector, error) {
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	return stdlib.GetConnector(*config), nil
}

// IsDriver reports whether driver is pgx's database/sql adapter.
func IsDriver(databaseDriver driver.Driver) bool {
	_, ok := databaseDriver.(*stdlib.Driver)
	return ok
}

// Array adapts a Go slice for both pgx and database/sql test drivers. pgx uses
// the pgtype.ArrayGetter methods, while generic drivers can use driver.Valuer.
// A pointer is treated as an array scan destination.
func Array(value any) any {
	reflected := reflect.ValueOf(value)
	if reflected.IsValid() && reflected.Kind() == reflect.Ptr {
		return pgtype.NewMap().SQLScanner(value)
	}
	return arrayValue{value: value}
}

type arrayValue struct {
	value any
}

func (a arrayValue) Dimensions() []pgtype.ArrayDimension {
	value := reflect.ValueOf(a.value)
	if !value.IsValid() || (value.Kind() == reflect.Slice && value.IsNil()) {
		return nil
	}
	return []pgtype.ArrayDimension{{Length: int32(value.Len()), LowerBound: 1}}
}

func (a arrayValue) Index(index int) any {
	return reflect.ValueOf(a.value).Index(index).Interface()
}

func (a arrayValue) IndexType() any {
	valueType := reflect.TypeOf(a.value)
	if valueType == nil || (valueType.Kind() != reflect.Slice && valueType.Kind() != reflect.Array) {
		return nil
	}
	return reflect.Zero(valueType.Elem()).Interface()
}

func (a arrayValue) Value() (driver.Value, error) {
	value := reflect.ValueOf(a.value)
	if !value.IsValid() || (value.Kind() == reflect.Slice && value.IsNil()) {
		return nil, nil
	}
	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		return nil, fmt.Errorf("postgres array requires a slice or array, got %T", a.value)
	}

	var builder strings.Builder
	_ = builder.WriteByte('{')
	for index := 0; index < value.Len(); index++ {
		if index > 0 {
			_ = builder.WriteByte(',')
		}
		element := value.Index(index)
		if (element.Kind() == reflect.Ptr || element.Kind() == reflect.Interface) && element.IsNil() {
			_, _ = builder.WriteString("NULL")
			continue
		}
		if element.Kind() == reflect.Interface || element.Kind() == reflect.Ptr {
			element = element.Elem()
		}
		if element.Kind() == reflect.String {
			_ = builder.WriteByte('"')
			_, _ = builder.WriteString(strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(element.String()))
			_ = builder.WriteByte('"')
			continue
		}
		_, _ = fmt.Fprint(&builder, element.Interface())
	}
	_ = builder.WriteByte('}')
	return builder.String(), nil
}

type sqlStateError interface {
	SQLState() string
}

// SQLState returns a PostgreSQL SQLSTATE from wrapped pgx errors.
func SQLState(err error) string {
	if err == nil {
		return ""
	}
	var stateErr sqlStateError
	if !errors.As(err, &stateErr) || stateErr == nil {
		return ""
	}
	value := reflect.ValueOf(stateErr)
	if value.Kind() == reflect.Ptr && value.IsNil() {
		return ""
	}
	return stateErr.SQLState()
}

// IsSQLState reports whether err contains the requested PostgreSQL SQLSTATE.
func IsSQLState(err error, state string) bool {
	return SQLState(err) == state
}

// QuoteIdentifier safely quotes a PostgreSQL identifier.
func QuoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

// QuoteLiteral safely quotes a PostgreSQL string literal with standard_conforming_strings enabled.
func QuoteLiteral(literal string) string {
	return `'` + strings.ReplaceAll(literal, `'`, `''`) + `'`
}
