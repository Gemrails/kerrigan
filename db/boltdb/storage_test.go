package boltdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"github.com/asdine/storm"
	"time"
	"fmt"
)

// CreateTempFile creates a temporary file and returns its path
func CreateTempFile(t *testing.T) string {
	file, err := ioutil.TempFile("", "")
	assert.Nil(t, err)
	return file.Name()
}

// CleanupTempFile removes the given temp file
func CleanupTempFile(t *testing.T, fileName string) {
	err := os.Remove(fileName)
	if err != nil {
		t.Logf("Could not remove temp file: %v. Err: %v\n", fileName, err)
	}
}
type User struct{
	ID 			int
	Group 		string	`storm:"index"`
	Email		string  `storm:"unique"`
	Name 		string
	Age 		int		`storm:"index"`
	CreatedAt	time.Time	`storm:"index"`
}

type Product struct {
	Pk                  int `storm:"id,increment"` // primary key with auto increment
	Name                string
	IntegerField        uint64 `storm:"increment"`
	IndexedIntegerField uint32 `storm:"index,increment"`
	UniqueIntegerField  int16  `storm:"unique,increment=100"` // the starting value can be set
}

// CreateDB creates a new boltdb instance with a temp file and returns the underlying file and database
func TestCreateDB(t *testing.T)  {
	path := CreateTempFile(t)
	db, err := storm.Open(path)
	assert.Nil(t, err)
	defer db.Close()


//////////////////////////////////////////test for auto increment integer values ,通过解析结构体定义中的json定义实现的自增
//	p := Product{Name: "Vaccum Cleaner"}
//	fmt.Println("   ---------------------------------")
//	fmt.Println(p.Pk)
//	fmt.Println(p.IntegerField)
//	fmt.Println(p.IndexedIntegerField)
//	fmt.Println(p.UniqueIntegerField)
//	fmt.Println(p.Name)//Vaccum Cleaner
//	// 0
//	// 0
//	// 0
//	// 0
//	fmt.Println("--------------middle-------------------")
//	_ = db.Save(&p)
//
//	fmt.Println(p.Pk)
//	fmt.Println(p.IntegerField)
//	fmt.Println(p.IndexedIntegerField)
//	fmt.Println(p.UniqueIntegerField)
//	fmt.Println(p.Name)//Vaccum Cleaner
//	fmt.Println("   ---------------------------------")
	// 1
	// 1
	// 1
	// 100
///////////////////////////////////////////test for auto increment integer values


	///////test for save object保存一个object到数据库
	user := User{
		ID:			10,
		Group:  	"staff",
		Email: 		"jack@provider.com",
		Name: 		"jack",
		Age: 		25,
		CreatedAt:   time.Now(),
	}
	err = db.Save(&user)

	user1 := User{
		ID:			11,
		Group:  	"staff",
		Email: 		"jack1@provider.com",
		Name: 		"jack1",
		Age: 		26,
		CreatedAt:   time.Now(),
	}
	err = db.Save(&user1)

	user2 := User{
		ID:			12,
		Group:  	"staff1",
		Email: 		"jack2@provider.com",
		Name: 		"jack2",
		Age: 		26,
		CreatedAt:   time.Now(),
	}
	err = db.Save(&user2)
	if err != nil{
		fmt.Println("add another group error")
		fmt.Println(err)
	}
	//user.ID++
	//err = db.Save(&user)

	///////test for save object


	//////test for fetch one object 获取一个object的方式，查找一个object的方式
	var usr User
	err = db.One("Email","jack@provider.com",&usr)

	if err != nil{
		//fmt.Println("not found!!!")
		fmt.Println(err)
	}else{
		fmt.Println("found it !!!")
		fmt.Println(usr.Name)
	}
	var usr1 User
	err = db.One("Name","jack",&usr1)
	if err != nil{
		//fmt.Println("not found!!!")
		fmt.Println(err)
	}else{
		fmt.Println("found it!!!")
		fmt.Println(usr1.Email)
		fmt.Println("ID:")
		fmt.Println(usr1.ID)
	}
	var usr2 User
	err = db.One("Name","jhon",&usr2)
	if err != nil{
		//fmt.Println("not found!!!")
		fmt.Println(err)
	}else{
		fmt.Println("found it!!!")
		fmt.Println(usr2.Email)

	}
	//////test for fetch one object 获取一个object的方式，查找一个object的方式

	//////fetch multiple objects 获取多个object的方式
	var users []User
	err = db.Find("Group","staff",&users)
	if err != nil{
		fmt.Println(err)
	}
	fmt.Println(cap(users))

	fmt.Println(users[0].Group)
	fmt.Println(users[0].Name)
	fmt.Println(users[0].Email)

	fmt.Println(users[1].Group)
	fmt.Println(users[1].Name)
	fmt.Println(users[1].Email)

	//////fetch multiple objects 获取多个object的方式


	//////fetch all objects 获取所有object
	fmt.Println("------------------------fetch all objects")
	var users1 []User
	err = db.All(&users1)
	if err != nil{
		fmt.Println(err)
	}
	fmt.Println(cap(users1))

	fmt.Println(users1[0].Group)
	fmt.Println(users1[0].Name)
	fmt.Println(users1[0].Email)

	fmt.Println(users1[1].Group)
	fmt.Println(users1[1].Name)
	fmt.Println(users1[1].Email)

	fmt.Println(users1[2].Group)
	fmt.Println(users1[2].Name)
	fmt.Println(users1[2].Email)
	//fmt.Println("4th:")
	//fmt.Println(users1[3].Name)
	fmt.Println("------------------------fetch all objects")
	//////fetch all objects 获取所有object

	//////fetch all objects sorted by index
	fmt.Println("------------------------fetch all objects by index")
	var users2 []User
	err = db.AllByIndex("CreatedAt",&users2)
	if err!=nil{
		fmt.Println(err)
	}
	fmt.Println(cap(users1))

	fmt.Println(users2[0].Group)
	fmt.Println(users2[0].Name)
	fmt.Println(users2[0].Email)

	fmt.Println(users2[1].Group)
	fmt.Println(users2[1].Name)
	fmt.Println(users2[1].Email)

	fmt.Println(users2[2].Group)
	fmt.Println(users2[2].Name)
	fmt.Println(users2[2].Email)
	fmt.Println("------------------------fetch all objects by index")
	//////fetch all objects sorted by index
}

// CleanupDB removes the provided file after stopping the given db
func TestCleanupDB(t *testing.T) {
	path := CreateTempFile(t)
	db, err := storm.Open(path)
	assert.Nil(t, err)

	err = db.Close()
	if err != nil {
		t.Logf("Could not close db %v\n", err)
	}
	CleanupTempFile(t, path)
}