-- DROP DATABASE IF EXISTS bludgeon;
CREATE DATABASE IF NOT EXISTS bludgeon;

USE bludgeon;

-- DROP TABLE IF EXISTS employee
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

-- DROP TABLE IF EXISTS timer
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