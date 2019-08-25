# sqlgen

package for generating SQL statements in a chain style. The SQL statements can be used by Go ORM ```sqlx``` directly.

### Example

```Go
func insertRecord(db *sqlx.DB)
	record := struct {
		ID           int    `json:"id"`
		Username     string `json:"username"`
		Password     string `json:"password"`
		SessionToken string `json:"token,omitdb"`
		CreatedAt    int64  `json:"created_at"`
	}{
		1, "a", "password", "token", time.Now().Unix(),
    }
    // INSERT INTO user(id,username,password,created_at)VALUES(:id,:username,:password,:created_at
	query, args := NewBuilder(QuestionArgFunc, "db", "json").InsertStruct("user", &record).Query()
    db.NamedExec(query, args...)
}
```

### Usage

```New``` create a new sql builder. The first argument is a function that provides parameter placeholder in SQL statements:

* ```QuestionArgFunc``` is for sqlite and MySQL, which takes a single ? as parameter placeholder
* ```DollarArgFunc``` is for postgres, which will place $1, $2, ... as parameter placeholder
* For other databases, you may need to create a custom function, or modify this package on your own need.

#### Tips

If a structed is embeded, simply add a tag like ```json:","```,  all of its fields can be expanded into sql using ```InsertStruct``` or ```UpdateStruct```.

```go
type Info struct {
	Username string `json:"username"`
}

type Detail struct {
	Info `json:","
	Birthdate string `json:"birthdate"
}
```
