package session

import "testing"

type User struct {
	Name string `geeorm:"PRIMARY KEY"`
	Age  int
}

func TestCrateTable(t *testing.T) {

	s := NewSession().Model(&User{})
	/*err := s.CreateTable()
	if err != nil {
		t.Fatal("create table User error")
	} */
	err := s.DropTable()
	if err != nil {
		t.Fatal("drop table User error")
	}
	err = s.CreateTable()
	if err != nil {
		t.Fatal("create table User error")
	}
	if !s.HasTable() {
		t.Fatal("Failed to create table User")
	}
}
