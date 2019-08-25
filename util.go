/*Package sqlgen 提供数据库操作便利工具，如SQL语句拼凑*/
package sqlgen

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// ArgFunc to generate argument placeholder in SQL statement
type ArgFunc func(idx int) string

// Builder to create sql
type Builder struct {
	args  []interface{}
	b     strings.Builder
	argFn ArgFunc
	dbTag string

	isWhereAdded bool
}

// DollarArgFunc ArgFunc for postgres
func DollarArgFunc(idx int) string {
	return "$" + strconv.Itoa(idx)
}

// QuestionArgFunc for MySQL, sqlite
func QuestionArgFunc(int) string {
	return "?"
}

// NewBuilder create builder
func NewBuilder(fn ArgFunc, dbTag string) *Builder {
	return &Builder{
		argFn: fn,
		dbTag: dbTag,
	}
}

// NewDefaultBuilder return default SQL Builder for postgres
func NewDefaultBuilder() *Builder {
	return NewBuilder(DollarArgFunc, "json")
}

// Raw simply add SQL
func (b *Builder) Raw(raw string) *Builder {
	b.b.WriteString(raw)
	return b
}

// Select start select statement
func (b *Builder) Select(columns ...string) *Builder {
	b.b.WriteString("SELECT ")
	b.b.WriteString(strings.Join(columns, ","))
	return b
}

// From add from statement
func (b *Builder) From(tables ...string) *Builder {
	b.b.WriteString(" FROM ")
	b.b.WriteString(strings.Join(tables, ","))
	return b
}

// Where add where statement and condition triples.
// e.g. WHERE a.b=1
// The condition above can be splitted into a condition triple ["a.b", "=", 1]
func (b *Builder) Where(condTriples ...interface{}) *Builder {
	// No conditions, skip
	if len(condTriples) == 0 {
		return b
	}
	if len(condTriples)%3 != 0 {
		panic(fmt.Sprintf("condition triples has incorrect length:%d", len(condTriples)))
	}
	if !b.isWhereAdded {
		b.b.WriteString(" WHERE ")
		b.isWhereAdded = true
	} else {
		b.b.WriteString(" AND ")
	}

	tripleNum := len(condTriples) / 3
	for i := 0; i < tripleNum; i++ {
		col, op := condTriples[i*3].(string), condTriples[i*3+1].(string)
		if col == "" || op == "" {
			panic(fmt.Sprintf("invalid column name or operator: %v%v", condTriples[i*3], condTriples[i*3+1]))
		}
		b.args = append(b.args, condTriples[i*3+2])
		b.b.WriteString(col)
		b.b.WriteString(op)
		b.b.WriteString(b.argFn(len(b.args)))
		if i+1 != tripleNum {
			b.b.WriteRune(' ')
		}
	}
	return b
}

// Update create update statement
func (b *Builder) Update(table string, sets ...interface{}) *Builder {
	// No conditions, skip
	if len(sets) == 0 {
		return b
	}
	if len(sets)%2 != 0 {
		panic(fmt.Sprintf("update sets has incorrect length:%d", len(sets)))
	}
	b.b.WriteString("UPDATE " + table + " SET ")
	setNum := len(sets) / 2
	for i := 0; i < setNum; i++ {
		col, ok := sets[i*2].(string)
		if !ok {
			panic(fmt.Sprintf("invalid column name: %v", sets[i*2]))
		}
		b.args = append(b.args, sets[i*2+1])

		b.b.WriteString(col)
		b.b.WriteString("=")
		b.b.WriteString(b.argFn(len(b.args)))
		if i != setNum-1 {
			b.b.WriteRune(',')
		}
	}
	return b
}

// Insert column/value pairs into table
func (b *Builder) Insert(table string, sets ...interface{}) *Builder {
	// No conditions, skip
	if len(sets) == 0 {
		return b
	}
	if len(sets)%2 != 0 {
		panic(fmt.Sprintf("insert sets has incorrect length:%d", len(sets)))
	}
	b.b.WriteString("INSERT INTO " + table)
	setNum := len(sets) / 2
	var cols []string
	var args []string
	for i := 0; i < setNum; i++ {
		col, ok := sets[i*2].(string)
		if !ok {
			panic(fmt.Sprintf("invalid column name: %v", sets[i*2]))
		}
		cols = append(cols, col)

		b.args = append(b.args, sets[i*2+1])
		args = append(args, b.argFn(len(b.args)))
	}
	b.b.WriteRune('(')
	b.b.WriteString(strings.Join(cols, ","))
	b.b.WriteRune(')')
	b.b.WriteString("VALUES(")
	b.b.WriteString(strings.Join(args, ","))
	b.b.WriteRune(')')
	return b
}

// InsertStruct create insert query with named args, leveraging sqlx NamedExec
// If dbTag are given, it will look for tags one by one, and take the first tag value found as column name for this field. If it's "-",
// the field will be ignored in INSERT query.
// If dbTag are empty, it will assume the field name is exactly the column name - and of couse it shall be ugly.
func (b *Builder) InsertStruct(table string, v interface{}) *Builder {
	columns, args := reflectColumnValues(v, b.dbTag)
	argPlaceholder := make([]string, len(args))
	for i, arg := range args {
		b.args = append(b.args, arg)
		argPlaceholder[i] = b.argFn(len(b.args))
	}

	b.b.WriteString("INSERT INTO " + table + "(")
	b.b.WriteString(strings.Join(columns, ","))
	b.b.WriteRune(')')
	b.b.WriteString("VALUES(")
	b.b.WriteString(strings.Join(argPlaceholder, ","))
	b.b.WriteRune(')')
	return b
}

// UpdateStruct create update query for every field in the struct, except for the specified ones
func (b *Builder) UpdateStruct(table string, v interface{}, except ...string) *Builder {
	exceptMap := make(map[string]bool)
	for _, e := range except {
		exceptMap[e] = true
	}

	cols, args := reflectColumnValues(v, b.dbTag)
	var updates []string
	for i, col := range cols {
		if exceptMap[col] {
			continue
		}
		b.args = append(b.args, args[i])
		updates = append(updates, col+"="+b.argFn(len(b.args)))
	}

	b.b.WriteString("UPDATE " + table + " SET ")
	b.b.WriteString(strings.Join(updates, ","))
	return b
}

func reflectColumnValues(v interface{}, dbTag string) (columns []string, args []interface{}) {
	tv := reflect.TypeOf(v)
	rv := reflect.ValueOf(v)
	if tv.Kind() == reflect.Ptr {
		tv = tv.Elem()
		rv = rv.Elem()
	}

fieldLoop:
	for i := 0; i < tv.NumField(); i++ {
		f := tv.Field(i)
		fv := rv.Field(i)
		if !fv.CanInterface() {
			panic("cannot interface " + f.Name)
		}
		argValue := fv.Interface()
		var dbTv string
		if dbTag == "" {
			dbTv = f.Name
		} else {
			var ok bool
			if dbTv, ok = f.Tag.Lookup(dbTag); ok {
				if dbTv == "," {
					embedCols, embedArgs := reflectColumnValues(argValue, dbTag)
					columns = append(columns, embedCols...)
					args = append(args, embedArgs...)
					continue
				}
				values := strings.Split(dbTv, ",")
				dbTv = values[0]
				if len(values) > 1 {
					for _, v := range values[1:] {
						if v == "omitdb" {
							continue fieldLoop
						}
					}
				}

			}
		}

		args = append(args, argValue)
		columns = append(columns, dbTv)
	}
	return
}

// Delete start DELETE, forget adding where will be VERY DANGEROUS
func (b *Builder) Delete(table string) *Builder {
	b.b.WriteString("DELETE FROM ")
	b.b.WriteString(table)
	return b
}

// OrderBy order by something, each statement is a string like "created_at DESC"
func (b *Builder) OrderBy(statements ...string) *Builder {
	if len(statements) == 0 {
		return b
	}
	b.b.WriteString(" ORDER BY ")
	b.b.WriteString(strings.Join(statements, ","))
	return b
}

// GroupBy columns separated by commas
func (b *Builder) GroupBy(columns ...string) *Builder {
	if len(columns) == 0 {
		return b
	}
	b.b.WriteString(" GROUP BY ")
	b.b.WriteString(strings.Join(columns, ","))
	return b
}

// Limit add limit
func (b *Builder) Limit(n int) *Builder {
	b.b.WriteString(" LIMIT ")
	b.b.WriteString(strconv.Itoa(n))
	return b
}

// Offset add offset
func (b *Builder) Offset(o int) *Builder {
	b.b.WriteString(" OFFSET ")
	b.b.WriteString(strconv.Itoa(o))
	return b
}

// Query return final SQL
func (b *Builder) Query() (string, []interface{}) {
	return b.b.String(), b.args
}
