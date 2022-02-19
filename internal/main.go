package internal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func initialize(envs map[string]string) (*sql.DB, error) {
	fmt.Println("Attempting to initialize and ping the database")
	config := ConfigFromEnv(envs)
	db, err := Initialize(config)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func employeeConcurrentCreate(db *sql.DB) error {
	const (
		firstName    string = "Antonio"
		lastName     string = "Alexander"
		emailAddress string = "antonio.alexander@mistersoftwaredeveloper.com"
	)

	fmt.Println("============================================")
	fmt.Println("--Testing Concurrent Create with Employees--")
	fmt.Println("============================================")
	//attempt to create employee
	employee := &Employee{
		ID:           GenerateID(),
		FirstName:    firstName,
		LastName:     lastName,
		EmailAddress: emailAddress,
		//KIM: version is effectively ignored/read-only
		// Version:      0,
	}
	fmt.Println("  Attempting to delete all current employees/timers")
	if err := TimerDelete(db, ""); err != nil {
		return err
	}
	if err := EmployeeDelete(db, nil); err != nil {
		return err
	}
	employee, err := EmployeeCreate(db, employee)
	if err != nil {
		return err
	}
	bytes, _ := json.MarshalIndent(employee, "  ", " ")
	fmt.Printf("  Created employee: \n\n  %s\n\n", string(bytes))
	employee = &Employee{
		ID:           GenerateID(),
		FirstName:    firstName,
		LastName:     lastName,
		EmailAddress: emailAddress,
		//KIM: version is effectively ignored/read-only
		// Version:      0,
	}
	bytes, _ = json.MarshalIndent(employee, "  ", " ")
	fmt.Printf("  Attempting to create the same employee, but with a different ID: \n\n  %s\n\n", string(bytes))
	employee, err = EmployeeCreate(db, employee)
	if err != nil {
		return err
	}
	bytes, _ = json.MarshalIndent(employee, "  ", " ")
	fmt.Printf("  Notice that although there was no error, the id remains the same,\n   the names were mutated and the version was incremented:  \n\n  %s\n", string(bytes))
	return nil
}

func employeeConcurrentWrite(db *sql.DB) error {
	const (
		firstName    string = "Teddy"
		lastName     string = "Perkins"
		emailAddress string = "teddy.perkins@atlanta.com"
	)

	fmt.Println("\n============================================")
	fmt.Println("--------Testing Concurrent Mutations--------")
	fmt.Println("============================================")
	//attempt to create employee
	employee := &Employee{
		ID:           GenerateID(),
		FirstName:    firstName,
		LastName:     lastName,
		EmailAddress: emailAddress,
		//KIM: version is effectively ignored/read-only
		// Version:      0,
	}
	if err := EmployeeDelete(db, nil); err != nil {
		return err
	}
	employee, err := EmployeeCreate(db, employee)
	if err != nil {
		return err
	}
	fmt.Printf("  Attempt to mutate the employee by maintaining the latest version of %d\n", employee.Version)
	mutatedEmployee, err := EmployeeWrite(db, &Employee{
		ID:        employee.ID,
		FirstName: "Theodore",
		LastName:  "Perkins",
		Version:   employee.Version,
	})
	if err != nil {
		return err
	}
	bytes, _ := json.MarshalIndent(mutatedEmployee, "  ", " ")
	fmt.Printf("  Notice that this employee mutation was successful:  \n\n  %s\n\n", string(bytes))
	fmt.Printf("  Attempt to mutate the employee again, but use the older version %d rather than the new version %d\n", employee.Version, mutatedEmployee.Version)
	_, err = EmployeeWrite(db, &Employee{
		ID:        employee.ID,
		FirstName: "Theodore",
		LastName:  "Perkins",
		Version:   employee.Version,
	})
	if err == nil {
		fmt.Println("\n!! an error was expected but didn't occur")
		return nil
	}
	*employee = *mutatedEmployee
	fmt.Printf("  Notice that the mutation failed with the error: \n   \"%s\"\n because the version wasn't as expected\n", err.Error())
	return nil
}

func employeeConcurrentMutations(db *sql.DB) error {
	const (
		firstName    string = "Antonio"
		lastName     string = "Alexander"
		emailAddress string = "antonio.alexander@mistersoftwaredeveloper.com"
	)

	fmt.Println("=====================================================")
	fmt.Println("-Testing Concurrent Mutations with Different Timings-")
	fmt.Println("=====================================================")
	//attempt to create employee
	employee := &Employee{
		ID:           GenerateID(),
		FirstName:    firstName,
		LastName:     lastName,
		EmailAddress: emailAddress,
		//KIM: version is effectively ignored/read-only
		// Version:      0,
	}
	employee, err := EmployeeCreate(db, employee)
	if err != nil {
		return err
	}
	bytes, _ := json.MarshalIndent(employee, "  ", " ")
	fmt.Printf("  Created employee: \n\n  %s\n\n", string(bytes))
	fmt.Println("  We're going to start to go routines running at different rates")
	fmt.Println("   and record the number of times a mutation failure occurs within 10s")
	for _, v := range []struct {
		rate   time.Duration
		offset time.Duration
	}{
		{
			rate:   2 * time.Second,
			offset: time.Second,
		},
		{
			rate:   time.Second,
			offset: time.Second,
		},
	} {
		wg := sync.WaitGroup{}
		fmt.Printf("\n  Attempting at a rate of %v and an offset of %v\n", v.rate, v.offset)
		stopper := make(chan struct{})
		start := make(chan struct{})
		wg.Add(3)
		go func() {
			defer wg.Done()

			<-start
			<-time.After(10 * time.Second)
			close(stopper)
		}()
		for i := 0; i < 2; i++ {
			go func(n int, employeeID string) {
				defer wg.Done()

				var writeFailures int

				<-start
				if t := time.Duration(n) * v.offset; t > 0 {
					<-time.After(t)
				}
				tCheck := time.NewTicker(v.rate)
				defer tCheck.Stop()
				for {
					select {
					case <-stopper:
						fmt.Printf("  >Routine %d, experienced %d failures\n", n, writeFailures)
						return
					case <-tCheck.C:
						employee, err := EmployeeRead(db, employeeID)
						if err != nil {
							fmt.Printf("  >Routine %d, experienced error reading: %s\n", n, err.Error())
							continue
						}
						_, err = EmployeeWrite(db, employee)
						if err != nil {
							writeFailures++
						}
					}
				}
			}(i, employee.ID)
		}
		close(start)
		wg.Wait()
	}

	return nil
}

func concurrencyTables(db *sql.DB) error {
	const (
		firstName    string = "Antonio"
		lastName     string = "Alexander"
		emailAddress string = "antonio.alexander@mistersoftwaredeveloper.com"
	)

	fmt.Println("\n============================================")
	fmt.Println("-----Testing Concurrency Between Tables-----")
	fmt.Println("============================================")
	//TODO: create employee

	employee, err := EmployeeCreate(db, &Employee{
		ID:           GenerateID(),
		FirstName:    firstName,
		LastName:     lastName,
		EmailAddress: emailAddress,
		//KIM: version is effectively ignored/read-only
		// Version:      0,
	})
	if err != nil {
		return err
	}
	timer := &Timer{
		ID:         GenerateID(),
		Comment:    "This is a comment",
		Start:      time.Now().UnixNano(),
		Finish:     0,
		EmployeeID: "",
	}
	bytes, _ := json.MarshalIndent(timer, "  ", " ")
	fmt.Printf("  Attempting to create a timer with a non-existent employee id:  \n\n  %s\n\n", string(bytes))
	_, err = TimerCreate(db, timer)
	if err == nil {
		fmt.Println("\n!! an error was expected but didn't occur")
		return nil
	}
	fmt.Printf("  This create failed with the following error: \n   \"%s\"\n   because of the foreign key constraint\n", err.Error())
	fmt.Printf("  If we update the timer with a valid employee id, we can now be successful\n")
	timer.EmployeeID = employee.ID
	timer, err = TimerCreate(db, timer)
	if err != nil {
		return err
	}
	bytes, _ = json.MarshalIndent(timer, "  ", " ")
	fmt.Printf("  We've created the following timer:  \n\n  %s\n\n", string(bytes))
	return nil
}

func Main(pwd string, args []string, envs map[string]string, osSignal chan os.Signal) error {
	db, err := initialize(envs)
	if err != nil {
		return err
	}
	if err := employeeConcurrentCreate(db); err != nil {
		return err
	}
	if err := employeeConcurrentWrite(db); err != nil {
		return err
	}
	if err := employeeConcurrentMutations(db); err != nil {
		return err
	}
	if err := concurrencyTables(db); err != nil {
		return err
	}
	fmt.Println("Closing the database")
	if err := db.Close(); err != nil {
		fmt.Printf(" Error occured while closing the database: \"%s\"\n", err.Error())
	}
	return nil
}
