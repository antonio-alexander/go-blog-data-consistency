package internal

import "database/sql"

//KIM: these objects were copied from the project github.com/antonio-alexander/go-bludgeon
// they have certainly been modified

const (
	tableTimer    string = "timer"
	tableEmployee string = "employee"
)

// These variables are populated at build time
// REFERENCE: https://www.digitalocean.com/community/tutorials/using-ldflags-to-set-version-information-for-go-applications
// to find where the variables are...
//  go tool nm ./app | grep app
var (
	Version   string
	GitCommit string
	GitBranch string
)

//Employee models the information that describes an employee
type Employee struct {
	ID           string `json:"id"`
	FirstName    string `json:"first_name,omitempty"`
	LastName     string `json:"last_name,omitempty"`
	EmailAddress string `json:"email_address"`
	Version      int    `json:"version"`
	LastUpdated  int64  `json:"last_updated"`
}

//Timer models a given timer with a start/stop time, its specifically used
// to show the relationship between timers and employees
type Timer struct {
	ID          string `json:"id"`
	Comment     string `json:"comment"`
	Start       int64  `json:"start"`
	Finish      int64  `json:"finish"`
	ElapsedTime int64  `json:"elapsed_time"`
	Completed   bool   `json:"completed"`
	Version     int    `json:"version"`
	EmployeeID  string `json:"employee_id"`
}

//DB provides an interface that implements all functions required
// by the DB
type DB interface {
	Begin() (*sql.Tx, error)
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Query(query string, args ...interface{}) (*sql.Rows, error)
}
