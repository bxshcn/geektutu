package schema

import (
	"geektutu/geeorm/dialect"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	type User struct {
		Name         string `geeorm:"PRIMARY KEY"`
		Age          int
		privateField string
	}

	testCases := []struct {
		desc    string
		typ     reflect.Type
		dialect string
	}{
		{
			desc:    "first test case: User",
			typ:     reflect.ValueOf(User{}).Type(),
			dialect: "sqlite3",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			var schema *Schema
			if dialect, ok := dialect.GetDialect(tC.dialect); ok {
				schema = Parse(reflect.New(tC.typ).Interface(), dialect)
			}
			if tC.typ.Name() == "User" {
				if schema.Name != "User" || len(schema.Fields) != 2 {
					t.Fatal("failed to parse User struct")
				}
				if schema.GetField("Name").Tag != "PRIMARY KEY" {
					t.Fatal("failed to parse primary key")
				}
			}
			//t.Error(*schema)
		})
	}
}
