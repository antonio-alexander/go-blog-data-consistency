# lv-blog-data-consistency (github.com/antonio-alexander/lv-blog-data-consistency)

This is a companion repository for an article describing data consistency, the goal of this repository is to try to show the differences between data consistency and logical consistency to understand how to properly use sagas to solve the problem of logical inconsistency.

This code should attempt to solve three problems specific to data consistency:

- When an entity could be created concurrently, how can we ensure that the “same” object isn’t created twice?
- If there are two instances of a given application (e.g. horizontal scaling) and the same endpoint is executed on the same entity at the same time, how can we ensure data consistency?
- If I have a service with a single responsibility, but it’s duties require multiple tables within its own database, how do I ensure data consistency with endpoints that have to mutate multiple tables?

The following sections are attempting to show the queries (for both mysql/postgres) and attempt to explain what they're trying to accomplish as well as attempt to answer the obvious (will this work in concurrent situations?).

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

## Creating an entity with a candidate key concurrently

In this query, we want to ensure that if we attempt to create the same "employee" as indicated by the candidate key, it won't create another employee. Things to keep in mind (in terms of the shema/table):

- the id is incremental and automatically created by the database (we don't use this in the code)
- the uuid is suppleid by the application
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

If we attempt to execute the same insert again, we get the error that we have a duplicate entry for 'uuid', if we change the uuid to something different, then we get an error for the email address. Thus we can have no two rows with the same uuid, id (implicitly) OR email_address. We can make our lives a bit easier, by fashioning our create as an "upsert" rather than an insert:

```sql
INSERT INTO employee (uuid, email_address) 
    VALUES ('c135e156-bd83-4d20-9574-9c6ac147800d', 'antonio.alexander@mistersoftwaredeveloper.com') 
    ON DUPLICATE KEY UPDATE id=id 
    RETURNING uuid, first_name, last_name, email_address, version;
```

This is does a handful of things for us (with some drawbacks), but considering that the ONLY reason we're doing this is to ensure that we don't accidentally create an employee twice, it's really nice in that we: (1) won't get an error if we attempt to insert twice if the candidate key already exists and (2) as coded, for this specific insert we ONLY edit primary key data hence we don't need to increment the version. In order to mutate the employee first/last name or email address, you'll need to use the following hypothetical update query.

```sql
UPDATE employee (first, name, last_name, email_address, version)
    VALUES('Antonio', 'Alexander', 'antonio.alexander@mistersoftwaredeveloper.com', version+1)
    WHERE uuid='c135e156-bd83-4d20-9574-9c6ac147800d'
```

It's not necessary that you restrict the "upsert" to only limiting primary key data, but if you do allow other fields to be updated in the same call, you'll need to edit the query to do the following (to maintain sanity). There's value in doing it this way to simplify the api such that you can provide the entire entity keeping in mind that this also has the drawback in that if done concurrently it will squash (but notify) the initial values for first/last name:

```sql
INSERT INTO employee (uuid, first_name, last_name, email_address) 
    VALUES ('c135e156-bd83-4d20-9574-9c6ac147800d', 'Antonio', 'Alexander', 'antonio.alexander@mistersoftwaredeveloper.com') 
    ON DUPLICATE KEY UPDATE id=id, first_name=first_name, last_name=last_name, version=version+1
    RETURNING uuid, first_name, last_name, email_address, version;
```

Also keep in mind that both of these solutions won't overwrite the initial uuid, so even though you generate a new uuid for the secondary create, it's thrown away and the original is returned.

Be careful when creating any entity concurrently that DOES NOT have a candidate key. It should ONLY occur in situations where the entity itself if incredibly specific and localized. For comparison to an employee (which would obviously be shared), a timer which exists for a specific employee is unlikely to be used by anyone other than that employee and if the emploee creates two timers, they would know which one was valid and which one wasn't. In this case there would be no candidate key and no way to prevent duplicate timers from being made. And in this case, thats OK.

## How can we identify concurent mutations?

This is the basic question about data consistency, once you've "successfully" created an entity, if you attempt to mutate it, how can you be sure that you're mutating the version that you read? We can ensure that we're mutating it by knowing that __all__ queries that mutate the entity will increment the version by one atomically. If we know what the current version is, we know what the current version SHOULD be once we mutate it, we can fashion our query (within a transaction) to attempt the mutation using a WHERE clause that contains both the id and version; we know if rows were affected, then there has been no other mutation, but if no rows are affected, then a mutation has happened and we can rollback our changes.

There is an appropriate sequence of SQL queries to do this interactively, but it's not super helpful; this is the Go code below that executes, and below that i'll describe the individual queries. Keep in mind that all of these queries are done with a transaction to maintain it's atomicity. We also HAVE to use a transaction since mysql doesn't support the RETURNING clause post update (unlike Postgres). It's also really important to keep in mind that doing the SELECT within the transaction is important since if you were to do it outside of the transaction, there's no guarantee that it would be "your" mutation.

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

1. You'll have to always perform at least one read (and/or cache) to know the current version of the entity OR
2. You'll have to make the "input" version optional (which kind of defeats the purpose)

Although it's a bit cumbersome, the query makes the implicit explicit: you will always have to perform a read before you mutate, but it gives you the ability to have a workflow where you can re-read and perform some action; whether that's re-reading the entity and attempting to mutate again or if you're really cool, perform some auditing to determine "who" is attempting to mutate the entity concurrently.

## How can we ensure data consistency between tables?

This isn't really a microservices specific problem, it's a problem that has to do with database architecture/schemas. It's a problem solved using one or more of the following tools:

- transactions
- foreign key constraints
- database normalization

Of the three above, [database normalization](https://en.wikipedia.org/wiki/Database_normalization) is the most important thing in this list, and for all intents and purposes combines both transactions, foreign key constraints and the necessary logic to ensure data consistency. A consistent database ensures that the following questions have very obvious answers:

- If I create a one way relationship between rows in two tables, how can I ensure that the dependent row can't be deleted
- How do I ensure that if I reference another id in a table, that the id is valid?
- If I have a one-to-many or many-to-many relationship between two tables, how can I ensure data consistency

As this topic is befitting it's own article altogether, I'll only show how we can use foreign constraints. This example is also a poor candidate for microservices, but understand that in a microservice architecture, it's incredibly unlikely that timers and employees would be within the same responsibility.

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
- You can delete an employee if that employee has timers associated with them
- You can "update" a timer with an invalid employee

The series of queries below can be used to create a "timer" which has a FK for employee id. Something you'll notice almost immediately, is that the "id" used within code, is not the same as the "id" ACTUALLY used as the FK. Within the database, the id is an primary key (PK) auto-number, while within "code" the id is a [uuid](https://en.wikipedia.org/wiki/Universally_unique_identifier#Version_4_(random)) generated within the application. Thus we have to perform a sort of "lookup" to be able to properly insert.

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

## Bibliography

- [https://www.sqlshack.com/dirty-reads-and-the-read-uncommitted-isolation-level/](https://www.sqlshack.com/dirty-reads-and-the-read-uncommitted-isolation-level/)
