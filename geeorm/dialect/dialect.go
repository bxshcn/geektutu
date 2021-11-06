package dialect

import "reflect"

// 初始化一个全局变量，保存Dialects
var DialectsMap = map[string]Dialect{}

type Dialect interface {
	// 根据go中的值对象，映射为数据库中的类型（string表示）
	// 不是用固定的map表来表示，而是用函数来表示这种映射关系
	// 显然，用函数来表示映射更为合理：想想数学中的函数概念！
	// [CUSTOM] 将参数reflect.Value改成reflect.Type
	DataTypeOf(fieldType reflect.Type) string
	// 判断某个表是否存在的sql语句
	// 实话说我并不直到不同数据库在判断表是否存在上有不同的sql语句
	// 我们拭目以待
	TableExistSQL(tableName string) (string, []interface{})
}

func RegisterDialect(name string, dialect Dialect) {
	DialectsMap[name] = dialect
}

func GetDialect(name string) (dialect Dialect, ok bool) {
	dialect, ok = DialectsMap[name]
	// 必须有return，否则函数只是自然结束，而不会返回相关出参。
	return
}
