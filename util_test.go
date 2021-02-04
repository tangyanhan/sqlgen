package sqlgen

import "testing"
import "time"

func assertBuilder(t *testing.T, b *Builder, argNum int, query string) {
	gotQuery, args := b.Query()
	if len(args) != argNum {
		t.Fatalf("Query=%s argNum=%d args=%v", query, argNum, args)
	}
	if gotQuery != query {
		t.Fatalf("Expect query=%s\n Got=\n%s", query, gotQuery)
	}
}

// Info info
type Info struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type person struct {
	Info   `json:","`
	Labels []string `json:"labels,json"`
}

func TestSelect(t *testing.T) {
	assertBuilder(t, NewDefaultBuilder().Select("*").From("dummy").
		Where("id", "=", 14, "x", "=", "y").
		OrderBy("created_at DESC"),
		2, "SELECT * FROM dummy WHERE `id`=$1 AND `x`=$2 ORDER BY created_at DESC")
	assertBuilder(t, NewDefaultBuilder().Select("*").From("user").GroupBy("area").Limit(10).Offset(1),
		2, "SELECT * FROM user GROUP BY area LIMIT $1 OFFSET $2")
	assertBuilder(t, NewBuilder(QuestionArgFunc, "").Update("user",
		"password", "abc",
		"nickname", "Ethan",
		"modified_at", time.Now().Unix()).Where("id", "=", 1),
		4, "UPDATE user SET `password`=?,`nickname`=?,`modified_at`=? WHERE `id`=?")
	assertBuilder(t, NewBuilder(QuestionArgFunc, "").Insert("user",
		"id", 1,
		"username", "a",
		"password", "password"),
		3, "INSERT INTO user(`id`,`username`,`password`)VALUES(?,?,?)")
	record := struct {
		ID           int    `json:"id"`
		Username     string `json:"username"`
		Password     string `json:"password"`
		SessionToken string `json:"token,omitdb"`
		CreatedAt    int64  `json:"created_at"`
	}{
		1, "a", "password", "token", time.Now().Unix(),
	}
	assertBuilder(t, NewBuilder(QuestionArgFunc, "json").InsertStruct("user", &record),
		4, "INSERT INTO user(`id`,`username`,`password`,`created_at`)VALUES(?,?,?,?)")
	assertBuilder(t, NewBuilder(QuestionArgFunc, "").InsertStruct("user", &record),
		5, "INSERT INTO user(`ID`,`Username`,`Password`,`SessionToken`,`CreatedAt`)VALUES(?,?,?,?,?)")
	assertBuilder(t, NewBuilder(QuestionArgFunc, "json").Select("*").From("user").Where("id", "=", 1).Where("password", "=", 2),
		2, "SELECT * FROM user WHERE `id`=? AND `password`=?")
	assertBuilder(t, NewBuilder(QuestionArgFunc, "json").Delete("users").Where("id", "=", 1),
		1, "DELETE FROM users WHERE `id`=?")
	assertBuilder(t, NewBuilder(QuestionArgFunc, "json").Delete("users").In("username", "a", "b"),
		2, "DELETE FROM users WHERE `username` IN(?,?)")

	embed := person{
		Info: Info{
			Name: "ethan",
			Age:  19,
		},
		Labels: []string{"a", "b", "c"},
	}

	assertBuilder(t, NewBuilder(QuestionArgFunc, "json").InsertStruct("user", &embed), 3, "INSERT INTO user(`name`,`age`,`labels`)VALUES(?,?,?)")
}
