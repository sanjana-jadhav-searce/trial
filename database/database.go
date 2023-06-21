package database

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

func ConnectDB() (*sql.DB, error) {

	dsn := "root:Sanju2001@/interview_system?charset=utf8&parseTime=True&loc=Local"

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to the database: %v", err)
	}

	// Test the database connection
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping the database: %v", err)
	}

	fmt.Println("Connected to the database!")

	return db, nil
}

func ConnectToDB() *sql.DB {
	return db
}
