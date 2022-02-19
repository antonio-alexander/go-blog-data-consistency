package internal_test

import (
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/antonio-alexander/go-blog-data-consistency/internal"

	"github.com/stretchr/testify/assert"

	_ "github.com/go-sql-driver/mysql"
)

var configuration *internal.Configuration

func init() {
	envs := make(map[string]string)
	for _, env := range os.Environ() {
		if s := strings.Split(env, "="); len(s) > 1 {
			envs[s[0]] = strings.Join(s[1:], "=")
		}
	}
	configuration = internal.ConfigFromEnv(envs)
}

func initDatabase() (*sql.DB, error) {
	return internal.Initialize(configuration)
}

func TestConcurrentCreate(t *testing.T) {
	db, err := initDatabase()
	assert.Nil(t, err)
	err = db.Ping()
	assert.Nil(t, err)
	employee := &internal.Employee{
		ID:           "",
		FirstName:    "Antonio",
		LastName:     "Alexander",
		EmailAddress: "antonio.alexander@mistersoftwaredeveloper.com",
		//KIM: version is effectively ignored/read-only
		// Version:      0,
	}
	firstUUID := internal.GenerateID()
	employee.ID = firstUUID
	//delete the employee
	err = internal.EmployeeDelete(db, employee)
	assert.Nil(t, err)
	//attempt to create employee
	employeeCreated, err := internal.EmployeeCreate(db, employee)
	assert.Nil(t, err)
	assert.Equal(t, 1, employeeCreated.Version)
	employee.Version = employeeCreated.Version
	assert.Equal(t, employee, employeeCreated)
	//attempt to create again, but with an alternate id
	employee.ID = internal.GenerateID()
	employeeCreated, err = internal.EmployeeCreate(db, employee)
	assert.Nil(t, err)
	assert.Equal(t, firstUUID, employeeCreated.ID)
	assert.Equal(t, 2, employeeCreated.Version)
	//clean-up
	err = db.Close()
	assert.Nil(t, err)
}

func TestConcurrentMutate(t *testing.T) {
	db, err := initDatabase()
	assert.Nil(t, err)
	err = db.Ping()
	assert.Nil(t, err)
	employee := &internal.Employee{
		FirstName:    "Antonio",
		LastName:     "Alexander",
		EmailAddress: "antonio.alexander@mistersoftwaredeveloper.com",
	}
	employee.ID = internal.GenerateID()
	//delete the employee
	err = internal.EmployeeDelete(db, employee)
	assert.Nil(t, err)
	//attempt to create employee
	employeeCreated, err := internal.EmployeeCreate(db, employee)
	assert.Nil(t, err)
	//mutate employee first time
	employeeMutated, err := internal.EmployeeWrite(db, &internal.Employee{
		ID:           employeeCreated.ID,
		FirstName:    "Tony",
		LastName:     employeeCreated.LastName,
		EmailAddress: employeeCreated.EmailAddress,
		Version:      employeeCreated.Version,
	})
	assert.Nil(t, err)
	assert.Equal(t, employeeCreated.Version+1, employeeMutated.Version)
	//mutate employee a second time
	employeeMutated, err = internal.EmployeeWrite(db, &internal.Employee{
		ID:           employeeCreated.ID,
		FirstName:    "Tony",
		LastName:     employeeCreated.LastName,
		EmailAddress: employeeCreated.EmailAddress,
		Version:      employeeCreated.Version,
	})
	assert.NotNil(t, err)
	assert.Nil(t, employeeMutated)
	//clean-up
	err = db.Close()
	assert.Nil(t, err)
}

func TestConsistencyBetweenTables(t *testing.T) {
	db, err := initDatabase()
	assert.Nil(t, err)
	err = db.Ping()
	assert.Nil(t, err)
	timer := &internal.Timer{
		ID:         internal.GenerateID(),
		Comment:    "This is a comment",
		Start:      time.Now().UnixNano(),
		EmployeeID: "",
	}
	//create timer with non-existing employee
	_, err = internal.TimerCreate(db, timer)
	assert.NotNil(t, err)
	//create employee
	employee, err := internal.EmployeeCreate(db, &internal.Employee{
		ID:           internal.GenerateID(),
		FirstName:    "Antonio",
		LastName:     "Alexander",
		EmailAddress: "antonio.alexander@mistersoftwaredeveloper.com",
	})
	assert.Nil(t, err)
	//create timer
	timer.EmployeeID = employee.ID
	timerCreated, err := internal.TimerCreate(db, timer)
	assert.Nil(t, err)
	//TODO: read timer
	timerRead, err := internal.TimerRead(db, timer.ID)
	assert.Nil(t, err)
	assert.Equal(t, timerCreated, timerRead)
}
