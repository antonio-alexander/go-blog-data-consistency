package internal

import (
	"database/sql"
	"fmt"

	"github.com/pkg/errors"
)

//Initialize can be used to create a database pointer
// with the provided configuration
func Initialize(config *Configuration) (*sql.DB, error) {
	dataSourceName := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=%t",
		config.Username, config.Password, config.Hostname, config.Port, config.Database, config.ParseTime)
	return sql.Open("mysql", dataSourceName)
}

//EmployeeCreate can be used to upsert an employee, if the employee exists
// via its candidate keys, it'll return that employee rather than
// create its own
func EmployeeCreate(db interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}, employee *Employee) (*Employee, error) {

	if employee == nil {
		return nil, errors.New("employee is nil")
	}
	query := fmt.Sprintf(`INSERT INTO %s (uuid, first_name, last_name, email_address) 
			VALUES (?, ?, ?, ?) 
		ON DUPLICATE KEY UPDATE 
			first_name=?, last_name=?, version=version+1
		RETURNING 
			uuid, first_name, last_name, email_address, version;`,
		tableEmployee)
	args := []interface{}{
		employee.ID, employee.FirstName, employee.LastName, employee.EmailAddress, employee.FirstName, employee.LastName,
	}
	row := db.QueryRow(query, args...)
	if row.Err() != nil {
		return nil, row.Err()
	}
	employee = &Employee{}
	if err := row.Scan(
		&employee.ID,
		&employee.FirstName,
		&employee.LastName,
		&employee.EmailAddress,
		&employee.Version,
	); err != nil {
		return nil, err
	}
	return employee, nil
}

//EmployeeDelete can be used to delete a specific employee or
// all employees
func EmployeeDelete(db interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}, employee *Employee) error {

	var args []interface{}
	var query string

	if employee == nil {
		query = fmt.Sprintf("DELETE from %s", tableEmployee)
	} else {
		query = fmt.Sprintf("DELETE from %s WHERE uuid=? OR email_address=?", tableEmployee)
		args = []interface{}{employee.ID, employee.EmailAddress}
	}
	if _, err := db.Exec(query, args...); err != nil {
		return err
	}
	return nil
}

//EmployeeWrite can be used to mutate an existing employee, it will return an error
// if the provided version for employee isn't the current version
func EmployeeWrite(db interface {
	Begin() (*sql.Tx, error)
}, employee *Employee) (*Employee, error) {

	if employee == nil {
		return nil, errors.New("employee is nil")
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	query := fmt.Sprintf(`UPDATE %s SET first_name=?, last_name=?, email_address=?, version=version+1 WHERE uuid=? and version=?`,
		tableEmployee)
	args := []interface{}{
		employee.FirstName, employee.LastName, employee.EmailAddress, employee.ID, employee.Version,
	}
	result, err := tx.Exec(query, args...)
	if err != nil {
		return nil, err
	}
	if err := RowsAffected(result, "no rows affected, version mismatch or non-existent employee"); err != nil {
		return nil, err
	}
	query = fmt.Sprintf("SELECT uuid, first_name, last_name, email_address, version FROM %s WHERE uuid=? AND version=?", tableEmployee)
	args = []interface{}{employee.ID, employee.Version + 1}
	row := tx.QueryRow(query, args...)
	employee = &Employee{}
	if err := row.Scan(
		&employee.ID,
		&employee.FirstName,
		&employee.LastName,
		&employee.EmailAddress,
		&employee.Version,
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return employee, nil
}

func EmployeeRead(db interface {
	Begin() (*sql.Tx, error)
}, employeeUUID string) (*Employee, error) {

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	query := fmt.Sprintf(`SELECT uuid, first_name, last_name, email_address, version
		FROM %s WHERE uuid=?`, tableEmployee)
	row := tx.QueryRow(query, employeeUUID)
	employee := &Employee{}
	if err = row.Scan(
		&employee.ID,
		&employee.FirstName,
		&employee.LastName,
		&employee.EmailAddress,
		&employee.Version,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.Errorf("employee with id, \"%s\", not found locally", employeeUUID)
		}
		return nil, err
	}
	if err = tx.Rollback(); err != nil {
		return nil, err
	}
	return employee, nil
}

//TimerCreate can be used to create a timer, if the timer already exists
// it'll return that timer and update that timer
func TimerCreate(db interface {
	Begin() (*sql.Tx, error)
}, timer *Timer) (*Timer, error) {

	var employeeID int

	//REVIEW: it's a bit neater to do this with subqueries, but the
	// interaction between parameters and sub-queries is a bit strange
	// and causes mysql to crash...
	if timer == nil {
		return nil, errors.New("timer is nil")
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	query := fmt.Sprintf("SELECT id from %s WHERE uuid=?", tableEmployee)
	args := []interface{}{timer.EmployeeID}
	row := tx.QueryRow(query, args...)
	if err := row.Scan(&employeeID); err != nil {
		return nil, err
	}
	query = fmt.Sprintf(`INSERT INTO %s (uuid, start, comment, employee_id)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			comment=?, employee_id=?, version=version+1
		RETURNING
			uuid, start, comment, employee_id, version;`,
		tableTimer)
	args = []interface{}{
		timer.ID, timer.Start, timer.Comment, employeeID, timer.Comment, employeeID,
	}
	row = tx.QueryRow(query, args...)
	timer = &Timer{}
	if err := row.Scan(
		&timer.ID,
		&timer.Start,
		&timer.Comment,
		&timer.EmployeeID,
		&timer.Version,
	); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return timer, nil
}

//TimerRead can be used to read a given timer
func TimerRead(db interface {
	Begin() (*sql.Tx, error)
}, timerUUID string) (*Timer, error) {

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	query := fmt.Sprintf(`SELECT uuid, start, finish, comment, completed, employee_id, version
		FROM %s WHERE uuid=?`, tableTimer)
	row := tx.QueryRow(query, timerUUID)
	timer := &Timer{}
	if err = row.Scan(
		&timer.ID,
		&timer.Start,
		&timer.Finish,
		&timer.Comment,
		&timer.Completed,
		&timer.EmployeeID,
		&timer.Version,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.Errorf("timer with id, \"%s\", not found locally", timerUUID)
		}
		return nil, err
	}
	if err = tx.Rollback(); err != nil {
		return nil, err
	}
	return timer, nil
}

//TimerWrite can be used to mutate an existing timer
func TimerWrite(db interface {
	Begin() (*sql.Tx, error)
}, timer *Timer) (*Timer, error) {

	if timer == nil {
		return nil, errors.New("timer is nil")
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	query := fmt.Sprintf(`UPDATE %s SET comment=?, version=?
		WHERE uuid=? AND version=?`, tableTimer)
	args := []interface{}{
		timer.Comment, timer.Version + 1, timer.ID, timer.Version,
	}
	result, err := tx.Exec(query, args)
	if err != nil {
		return nil, err
	}
	if err := RowsAffected(result, "no rows affected, version mismatch or non-existent timer"); err != nil {
		return nil, err
	}
	query = fmt.Sprintf(`SELECT uuid, timer_start, timer_finish, timer_comment, timer_completed
		FROM %s WHERE uuid = ?`,
		tableTimer)
	row := tx.QueryRow(query, timer.ID)
	timer = &Timer{}
	if err = row.Scan(
		&timer.ID,
		&timer.Start,
		&timer.Finish,
		&timer.Comment,
		&timer.Completed,
	); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return timer, nil
}

//TimerDelete can be used to delete one or all timers
func TimerDelete(db interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}, timerID string) error {

	var args []interface{}
	var query string

	if timerID == "" {
		query = fmt.Sprintf("DELETE from %s", tableTimer)
	} else {
		query = fmt.Sprintf("DELETE from %s WHERE uuid=?", tableTimer)
		args = []interface{}{timerID}
	}
	if _, err := db.Exec(query, args...); err != nil {
		return err
	}
	return nil
}
