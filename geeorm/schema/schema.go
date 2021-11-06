package schema

import (
	"geektutu/geeorm/dialect"
	"geektutu/geeorm/log"
	"reflect"
)

type Field struct {
	Name string
	Type string
	Tag  string
}

// Schema表示一个表对象，原作与go对象Model关联是不恰当的
// 实际上应该是与go对象类型关联，类型唯一的对应了一个Schema或者说数据库表。
// -- 虽然schema“应该”与go对象类型关联，但session引用了schema并将其视为当前处理的表或者对象
// -- 所以schema中存放了具体的go对象，从而方便操作。
// -- 什么情况下session的具体操作需要了解当前的值呢？select? update? or insert?
// -- 似乎都是通过参数提供的，因此不需要。
// -- 那么hook呢？
// 最重要的，session只与表关联，而与具体的记录无关，因此Model不应该是具体的go对象！
// 具体的记录是执行sql语句时，实施提供（Insert，或者update）或者生成得到的（Find）。
type Schema struct {
	//Model  interface{}
	Model  reflect.Type
	Name   string
	Fields []*Field
	// 只是为了方便而增加的字段
	FieldNames []string
	fieldMap   map[string]*Field
}

func (schema *Schema) GetField(name string) *Field {
	return schema.fieldMap[name]
}

// obj是一个结构体对象或者一个指向结构体对象的指针
// 我们根据当前的dialect来将这个结构体对象转换为一个本地的Schema对象，方便后续构建sql操作。
func Parse(obj interface{}, dialect dialect.Dialect) *Schema {
	// 表名等于结构体名称
	value := reflect.Indirect(reflect.ValueOf(obj))
	if value.Kind() != reflect.Struct {
		log.Error("Parse error: the object is not a structi\n")
		return nil
	}
	typ := value.Type()
	schema := &Schema{
		//Model:    obj,
		Model:    typ,
		Name:     typ.Name(),
		fieldMap: make(map[string]*Field), // 初始化
	}
	// 准备其余的字段
	for i := 0; i < typ.NumField(); i++ {
		reflectField := typ.Field(i)
		if !reflectField.Anonymous && reflectField.PkgPath == "" {
			field := &Field{
				Name: reflectField.Name,
				Type: dialect.DataTypeOf(reflectField.Type),
			}
			if v, ok := reflectField.Tag.Lookup("geeorm"); ok {
				field.Tag = v
			}
			schema.Fields = append(schema.Fields, field)
			schema.FieldNames = append(schema.FieldNames, field.Name)
			schema.fieldMap[field.Name] = field
		}
	}

	return schema
}

// 将一个对象先转换成[fieldname, ]的slice形式，
// 然后strings.Join([]string, ",")拼接，并fmt.Sprintf("(%s)",string)  // 无需如此！格式化是(?, ), (?, )，通过clause.Set(VALUE)即可得到
func (s *Schema) RecordValues(obj interface{}) []interface{} {
	objValue := reflect.Indirect(reflect.ValueOf(obj))

	var fieldValues []interface{}

	for _, field := range s.Fields {
		fieldValues = append(fieldValues, objValue.FieldByName(field.Name).Interface())
	}

	return fieldValues
}
