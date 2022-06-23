# go-blog-data-consistency (github.com/antonio-alexander/go-blog-data-consistency)

This is a companion repository for an article describing data consistency, the goal of this repository is to try to show the differences between data consistency and logical consistency to understand how to properly use sagas to solve the problem of logical inconsistency.

This code should attempt to solve four problems specific to data consistency:

- When an object could be created concurrently, how can we ensure that the “same” object isn’t created twice?
- If there are two instances of a given application (e.g., horizontal scaling) and the same endpoint is executed on the same object at the same time, how can we ensure data consistency?
- If I have a service with a single responsibility, but its duties require multiple tables within its own database, how do I ensure data consistency with endpoints that have to mutate multiple tables?
- If I have two services with their own disconnected databases but reference each other’s data, how can I ensure data consistency?

The following sections are attempting to show the queries and attempt to explain what they're trying to accomplish as well as attempt to answer the obvious: will this work in concurrent situations?

## Lexicon

Below I'll provide some definitions for some terms that may be unknown (or ambiguous):

- alternate key: a combination of columns that can be used in part or whole to uniquely identify a row other than its primary key
- data consistency: the idea that it's impossible for data to "not" make logical sense at any point in time with respect to a given unit of work and its relationships
- single responsibility principle: an idea in microservice architecture where a service is only responsible for one thing (for a resource service, a single object or group of related objects, and for other services, a specific group of business logics)
- concurrency: very similar to running in parallel, but less exact hence more prone to architecture mistakes with the unit of work
- data normalization: the process of structuring a relational database in accordance with a series of so-called normal forms in order to reduce data redundancy and improve data integrity.

## Running the example via docker

Within the root directory of this repo is a docker-compose.yml that can be used to run the database as well as a build version of the example (built from the code). You can run the following commands to run it and see it work.

```sh
docker compose build
docker compose up -d
docker logs example -f
```

```output
Attempting to initialize and ping the database
============================================
--Testing Concurrent Create with Employees--
============================================
  Attempting to delete all current employees/timers
  Created employee:

  {
   "id": "a74f8be3-1672-46f9-acdc-0662187a8024",
   "first_name": "Antonio",
   "last_name": "Alexander",
   "email_address": "antonio.alexander@mistersoftwaredeveloper.com",
   "version": 1
  }

  Attempting to create the same employee, but with a different ID:

  {
   "id": "696d8624-b0f5-41c5-8c18-60eb201f02e2",
   "first_name": "Antonio",
   "last_name": "Alexander",
   "email_address": "antonio.alexander@mistersoftwaredeveloper.com",
   "version": 0
  }
  Notice that although there was no error, the id remains the same,
   the names were mutated and the version was incremented:

  {
   "id": "a74f8be3-1672-46f9-acdc-0662187a8024",
   "first_name": "Antonio",
   "last_name": "Alexander",
   "email_address": "antonio.alexander@mistersoftwaredeveloper.com",
   "version": 2
  }

============================================
--------Testing Concurrent Mutations--------
============================================
  Attempt to mutate the employee by maintaining the latest version of 1
  Notice that this employee mutation was successful:

  {
   "id": "2aea715c-3617-4430-a3b4-1f3395a2fbf4",
   "first_name": "Theodore",
   "last_name": "Perkins",
   "email_address": "",
   "version": 2
  }

  Attempt to mutate the employee again, but use the older version 1 rather than the new version 2
  Notice that the mutation failed with the error:
   "no rows affected, version mismatch or non-existent employee"
 because the version wasn't as expected
=====================================================
-Testing Concurrent Mutations with Different Timings-
=====================================================
  Created employee:

  {
   "id": "35b3883c-8768-4cd2-8c8b-01c9607a92b4",
   "first_name": "Antonio",
   "last_name": "Alexander",
   "email_address": "antonio.alexander@mistersoftwaredeveloper.com",
   "version": 1
  }

  We're going to start to go routines running at different rates
   and record the number of times a mutation failure occurs within 30s

  Attempting at a rate of 2s and an offset of 1s
  >Routine 1, experienced 0 failures
  >Routine 0, experienced 0 failures

  Attempting at a rate of 1s and an offset of 1s
  >Routine 1, experienced 17 failures
  >Routine 0, experienced 11 failures

============================================
-----Testing Concurrency Between Tables-----
============================================
  Attempting to create a timer with a non-existent employee id:

  {
   "id": "cd5d18bb-a7c3-45d4-90dd-c7d1683005d0",
   "comment": "This is a comment",
   "start": 1646804232936414000,
   "finish": 0,
   "elapsed_time": 0,
   "completed": false,
   "version": 0,
   "employee_id": ""
  }
  This create failed with the following error:
   "sql: no rows in result set"
   because of the foreign key constraint
  If we update the timer with a valid employee id, we can now be successful
  We've created the following timer:

  {
   "id": "cd5d18bb-a7c3-45d4-90dd-c7d1683005d0",
   "comment": "This is a comment",
   "start": 1646804232936414000,
   "finish": 0,
   "elapsed_time": 0,
   "completed": false,
   "version": 1,
   "employee_id": "10"
  }

Closing the database
```

## Creating an object with an alternate key concurrently

In this query, we want to ensure that if we attempt to create the same "employee" as indicated by the alternate key, it won't create another employee. Things to keep in mind (in terms of the schema/table):

- the id is incremental and automatically created by the database (we don't use this in the code)
- the uuid is supplied by the application
- the first/last name aren't required (can be null)
- the email address is required and must not be null
- the unique keys (separate, NOT TOGETHER) are uuid and email address.

```sql
INSERT INTO employee (uuid, first_name, last_name, email_address, version)
    VALUES ('c135e156-bd83-4d20-9574-9c6ac147800d', 'Antonio', 'Alexander', 'antonio.alexander@mistersoftwaredeveloper.com', version+1);
```

```sh
MariaDB [bludgeon]> select * from employee;
+----+--------------------------------------+------------+-----------+-----------------------------------------------+---------+
| id | uuid                                 | first_name | last_name | email_address                                 | version |
+----+--------------------------------------+------------+-----------+-----------------------------------------------+---------+
|  1 | c135e156-bd83-4d20-9574-9c6ac147800d | Antonio    | Alexander | antonio.alexander@mistersoftwaredeveloper.com |       1 |
+----+--------------------------------------+------------+-----------+-----------------------------------------------+---------+
```

If we attempt to execute the same insert again, we get the error that we have a duplicate entry for 'uuid', if we change the uuid to something different, then we get an error for the email address. Thus we can't have two rows with the same uuid, id (implicitly) OR email_address. We can make our lives a bit easier, by fashioning our create as an "upsert" rather than an insert:

```sql
INSERT INTO employee (uuid, email_address)
    VALUES ('c135e156-bd83-4d20-9574-9c6ac147800d', 'antonio.alexander@mistersoftwaredeveloper.com')
    ON DUPLICATE KEY UPDATE id=id
    RETURNING uuid, first_name, last_name, email_address, version;
```

This is does a handful of things for us (with some drawbacks), but considering that the ONLY reason we're doing this is to ensure that we don't accidentally create an employee twice, it's really nice in that we: (1) won't get an error if we attempt to insert twice if the alternate key already exists and (2) as coded, for this specific insert we ONLY edit primary key data hence we don't need to increment the version. In order to mutate the employee first/last name or email address, you'll need to use the following hypothetical update query.

```sql
UPDATE employee (first, name, last_name, email_address, version)
    VALUES('Antonio', 'Alexander', 'antonio.alexander@mistersoftwaredeveloper.com', version+1)
    WHERE uuid='c135e156-bd83-4d20-9574-9c6ac147800d'
```

It's not necessary that you restrict the "upsert" to only alternate key data (id, uuid and email address). There's value in doing it this way to simplify the api such that you can provide the entire object. This has the drawback that if done concurrently it can squash the initial values for first/last name; there's some built in notification since the version won't be 1; see this query:

```sql
INSERT INTO employee (uuid, first_name, last_name, email_address)
    VALUES ('c135e156-bd83-4d20-9574-9c6ac147800d', 'Antonio', 'Alexander', 'antonio.alexander@mistersoftwaredeveloper.com')
    ON DUPLICATE KEY UPDATE id=id, first_name=first_name, last_name=last_name, version=version+1
    RETURNING uuid, first_name, last_name, email_address, version;
```

Also keep in mind that both of these solutions won't overwrite the initial uuid, so even though you generate a new uuid for the secondary create, it's thrown away and the original is returned.

> Be careful when creating any object concurrently that DOES NOT have a alternate key. It should ONLY occur in situations where the object itself if incredibly specific and localized. For comparison to an employee (which would obviously be shared), a timer which exists for a specific employee is unlikely to be used by anyone other than that employee and if the employee creates two timers, they would know which one was valid and which one wasn't. In this case there would be no alternate key and no way to prevent duplicate timers from being made. And in this case, that’s OK.

## How can we identify concurrent mutations?

> The core idea behind this section is that although there are endpoints where we just write data (e.g. an endpoint to update first name for an employee), the actual flow of modifying data is often read > write > read in terms of the UI/UX. So when you see something on the screen, and then you attempt to modify it, the expectation is that you see your modification; but this isn't always true. This section is about how to determine that at runtime, programmatically.

This is the basic question about data consistency, once you've "successfully" created an object, if you attempt to mutate it, how can you be sure that you're mutating the version that you read? We can ensure that we're mutating it by knowing that **all** queries that mutate the object will increment the version atomically. If we know what the current version is, we know what the current version SHOULD be once we mutate it and we can fashion our query (within a transaction) to attempt the mutation using a WHERE clause that contains both the id and version; we know if rows were affected, then there has been no other mutation, but if no rows are affected, then a mutation has happened and we can rollback our changes.

The sequence of queries below can be used to perform this operation interactively. All of the queries below are done within a transaction to maintain the query's ACIDity (atomicity, consistency, isolation, and durability. Also specific to MySQL, the RETURNING clause post update isn't supported (like Postgres); it's important to remember that doing the SELECT within the transaction guarantees that you get "your" mutation rather than a concurrent mutation that occurs "after". The sequence of queries below will create an employee and then attempt to update that employee.

```sql
BEGIN;
INSERT INTO employee (uuid, first_name, last_name, email_address, version)
    VALUES ('c135e156-bd83-4d20-9574-9c6ac147800d', 'Antonio', 'Alexander', 'antonio.alexander@mistersoftwaredeveloper.com', version+1);
UPDATE employee
    SET first_name='Antonio', last_name='Alexander', email_address='antonio.alexander@mistersoftwaredeveloper.com', version=version+1
    WHERE uuid='c135e156-bd83-4d20-9574-9c6ac147800d' AND version=1;
SELECT id, first_name, last_name, email_address, version FROM employee
    WHERE uuid='c135e156-bd83-4d20-9574-9c6ac147800d' AND version=2;
COMMIT;
```

This idea only has the downside that whatever API you implement will require one of two things:

1. You'll have to always perform at least one read (and/or cache) to know the current version of the object OR
2. You'll have to make the "input" version optional

Although it's a bit cumbersome, the query makes the implicit explicit: you will always have to perform a read before you mutate, but it gives you the ability to have a workflow where you can re-read and perform some action; whether that's re-reading the object and attempting to mutate again or if you're really cool, perform some auditing to determine "who" is attempting to mutate the object concurrently.

## How can we ensure data consistency between tables?

This isn't really a microservices specific problem, it's a problem that has to do with database architecture/schemas. It's a problem solved using one or more of the following tools:

- database normalization
- transactions
- foreign key constraints

Of the three above, [database normalization](https://en.wikipedia.org/wiki/Database_normalization) is the most important thing in this list, and for all intents and purposes combines both transactions, foreign key constraints and the necessary logic to ensure data consistency. A consistent database ensures that the following questions have very obvious answers:

- If I create a one-way relationship between rows in two tables, how can I ensure that the dependent row can't be deleted
- How do I ensure that if I reference another id in a table, that the id is valid?
- If I have a one-to-many or many-to-many relationship between two tables, how can I ensure data consistency

As this topic is befitting its own article altogether, I'll only show how we can use foreign constraints. This example is also a poor alternate for microservices, but understand that in a microservice architecture, it's incredibly unlikely that timers and employees would be within the same responsibility.

We have two data types, a timer and an employee:

```go
type Employee struct {
    ID           string `json:"id"`
    FirstName    string `json:"first_name,omitempty"`
    LastName     string `json:"last_name,omitempty"`
    EmailAddress string `json:"email_address"`
    Version      int    `json:"version"`
}

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
```

There is a one-to-many relationship between a timer and an employee: an employee can have multiple timers, but a timer can only have one employee. We model this relationship with the tables using foreign key constraints.

```sql
CREATE TABLE IF NOT EXISTS employee (
    id BIGINT NOT NULL AUTO_INCREMENT,
    uuid TEXT NOT NULL,
    first_name TEXT,
    last_name TEXT,
    email_address TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,

    PRIMARY KEY (id),
    UNIQUE(uuid),
    UNIQUE(email_address)
) ENGINE = InnoDB;

CREATE TABLE IF NOT EXISTS timer (
    id BIGINT NOT NULL AUTO_INCREMENT,
    uuid TEXT(36) NOT NULL,
    start BIGINT NOT NULL,
    finish BIGINT DEFAULT 0,
    comment TEXT NOT NULL DEFAULT "",
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    version INT NOT NULL DEFAULT 1,
    employee_id BIGINT NOT NULL,

    PRIMARY KEY (id),
    FOREIGN KEY (employee_id) REFERENCES employee(id),
    UNIQUE(uuid(36)),
    INDEX(id)
) ENGINE = InnoDB;
```

The above schema/architecture ensures the following:

- You can't assign a timer to an invalid employee
- You can't delete an employee if that employee has timers associated with them
- You can't "update" a timer with an invalid employee

The series of queries below can be used to create a "timer" which has a FK for employee id. Something you'll notice almost immediately, is that the "id" used within code, is not the same as the "id" ACTUALLY used as the FK. Within the database, the id is an primary key (PK) auto-number, while within "code" the id is a [uuid](<https://en.wikipedia.org/wiki/Universally_unique_identifier#Version_4_(random)>) generated within the application. Thus we have to perform a sort of "lookup" to be able to properly insert.

```sql
SELECT id from employee WHERE uuid='c135e156-bd83-4d20-9574-9c6ac147800d';
INSERT INTO timer (uuid, start, comment, employee_id)
    VALUES ('646c2317-8878-4a36-aff7-819b804ace28', 1645411339, 'This is a comment', 1)
    ON DUPLICATE KEY UPDATE
        comment='This is a comment', employee_id=1, version=version+1
    RETURNING
        uuid, start, comment, employee_id, version;
```

This could also be solved using sub-queries, which although a bit cleaner is significantly harder to read. I also had trouble trying to use the sub queries AND parameterized queries (it would cause mariadb to crash).

```sql
INSERT INTO timer (uuid, start, comment, employee_id)
    VALUES ('646c2317-8878-4a36-aff7-819b804ace28', 1645411339, 'This is a comment', (SELECT id FROM employee WHERE uuid='c135e156-bd83-4d20-9574-9c6ac147800d'))
    ON DUPLICATE KEY UPDATE
        comment='This is a comment', employee_id=(SELECT id FROM employee WHERE uuid='c135e156-bd83-4d20-9574-9c6ac147800d'), version=version+1
    RETURNING
        uuid, start, comment, employee_id, version;
```

The queries above would fail in the event there was an attempt to create a timer with an employee that didn't exist because that would cause a data inconsistency (it would fail/override a FK).

> Alternatively you could also allow the employee_id to NOT be set, but removing the NOT NULL parameter to the table, in case you wanted to be able to create a timer without specifying an employee.

## How can we ensure data consistency between services?

Even though we use microservices, some of our ideas are still monolithic; some of these monolithic ideas can be simplified to maintaining that the data is consistent, while other require that the logic is consistent; one we can solve here, but the other would generally require sagas.

I'll use [go-bludgeon](https://github.com/antonio-alexander/go-bludgeon) as an example; it contains timers and employees like above, but implemented as separate services. There's a timer service that manages [timers](https://github.com/antonio-alexander/go-bludgeon/tree/develop/timers) and an employees service that manages [employees](https://github.com/antonio-alexander/go-bludgeon/tree/develop/employees). The timer and employees service implement a monolithic idea by referencing the employee in the timer: A timer belongs to/is associated with an employee. See the timer data type:

```json
{
  "id": "24dfe1eb-26a7-41db-a647-fe6cc5e77ab8",
  "start": 1653719208,
  "finish": 1653719229,
  "elasped_time": 21,
  "active_time_slice_id": "a33f813e-e9bc-46ad-9956-0c4b6c1367ab",
  "completed": true,
  "archived": false,
  "comment": "This is a timer for lunch",
  "employee_id": "2e3a4156-b415-4120-982f-399182e99588",
  "last_updated": 1652417242000,
  "last_updated_by": "bludgeon_employee_memory",
  "version": 1
}
```

The timer has a foreign reference in employee_id, the employee_id represents a static id which could be easily represented as a foreign key constraint, but because it's in a separate database foreign key constraints are unavailable. The foreign reference is one way: timers depend on employees, but employees don't depend no timers.

If we create a lazy implementation, we can do nothing and the following would be true: employee_id is arbitrary and it's up to the consumer of the API to ensure that this value makes sense in the context of their use. Which is fair but is a lot like saying that you suggest someone pulls the door to open, but the door can be pulled and pushed (seemingly with no consequence). To complete the analogy: if you pull the door, you have the chance of not stepping far enough inside the room and the door hitting you as it swings back.

The worst case scenario of this kind of setup, is what happens when an id doesn’t exist or exists and is then deleted. As coded, this is the ONLY situation where the relationship can create data inconsistencies (because id is static and unique for the life of the employee). It's best to identify what the business logic should be when the employee is deleted (or rather mutated) and create logic to communicate that (from the employees service) to any services dependent on employees.

Some possible business logic to respond to the situation of employees being deleted:

- Delete all timers referencing that employee_id (cascade delete)
- Update all timers referencing that employee_id to be set to empty ("orphaned" timers is another problem, but maybe that's ok within the scope of the use of the timers application)

It's also important to note that because timers is "dependent" on employees, it can't tell employees to "not" delete those timers, it would have to come from an object that was aware of both employees AND timers. This is often where microservices fall apart; you can implement everything as a microservice but some ideas are simply monolithic.

This list of rules/maxims should make it easier to see those relationships and make it clearer what needs to be done to prevent data inconsistencies:

- when referencing data not under the purview of a service, NEVER reference the data in whole, only the id or data that is static post creation (referencing dynamic data means there's always an opportunity to be wrong)
- ensure that all services where there could be disconnected data that depends on the service have a way to know when dependent data has been mutated

## Bibliography

- [https://www.sqlshack.com/dirty-reads-and-the-read-uncommitted-isolation-level/](https://www.sqlshack.com/dirty-reads-and-the-read-uncommitted-isolation-level/)
- [https://en.wikipedia.org/wiki/Database_normalization](https://en.wikipedia.org/wiki/Database_normalization)
