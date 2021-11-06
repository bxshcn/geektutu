package dialect

import (
	"fmt"
	"reflect"
	"time"
)

// sqlite3本质上只是实现了Dialect接口的实例
// 从目前来看，它并不需要自己的内在属性来支撑上述接口的实现
type sqlite3 struct{}

func init() {
	RegisterDialect("sqlite3", &sqlite3{})
}

// sqlite3的数据类型
// 参考：https://sqlite.org/lang_createtable.html & https://www.sqlite.org/datatype3.html
// 核心概念：
// 1. Storage Classes and Datatypes: NULL, INTEGER, REAL, TEXT, BLOB
// 2. type affinities: TEXT, NUMERIC, INTEGER, REAL, BLOB
// 3. Determination Of Column Affinity
// 简单说，就是根据columns declared type来决定对应列的type affinity，然后按照对应的type affinities
// 规则，以及提供的data value来决定具体的storage classes。
// Unlike most SQL databases, SQLite does not restrict the type of data that may
// be inserted into a column based on the columns declared type. Instead,
// SQLite uses dynamic typing. The declared type of a column is used to determine
// the affinity of the column only.
//
// 下面返回的string，即为columns declared type，which is compatible with sqlite3!
func (sqlite *sqlite3) DataTypeOf(dataType reflect.Type) string {
	switch dataType.Kind() {
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uintptr:
		return "integer"
	case reflect.Int64, reflect.Uint64:
		return "bigint"
	case reflect.Float32, reflect.Float64:
		return "real"
	case reflect.String:
		return "text"
	case reflect.Array, reflect.Slice:
		return "blob"
	case reflect.Struct:
		if _, ok := reflect.Indirect(reflect.New(dataType)).Interface().(time.Time); ok {
			return "datetime"
		}
	}
	panic(fmt.Sprintf("invalid sql type %s (%s)", dataType.Name(), dataType.Kind()))
}

func (sqlite *sqlite3) TableExistSQL(tableName string) (string, []interface{}) {
	args := []interface{}{tableName}
	return "select name FROM sqlite_master WHERE type='table' and name = ?", args
}
