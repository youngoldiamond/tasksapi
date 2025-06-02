package db

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/lib/pq"
	"github.com/youngoldiamond/tasksapi/internal/types"
)

type Config struct {
	User     string
	Password string
	DBName   string
	SSLMode  string
}

var defaultConfig = Config{
	User:    "postgres",
	DBName:  "tasks",
	SSLMode: "disable",
}

func DefaultConfig() Config {
	return defaultConfig
}

type DB struct {
	db *sql.DB
}

func Open(config Config) (*DB, error) {
	d := &DB{db: &sql.DB{}}

	connStr := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=%s", config.User, config.Password, config.DBName, config.SSLMode)

	var err error
	d.db, err = sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	pingErr := d.db.Ping()
	if pingErr != nil {
		return nil, pingErr
	}
	log.Println("Connected to DB")

	return d, nil
}

func (d *DB) Close() {
	d.db.Close()
}

func (d *DB) DB() *sql.DB {
	return d.db
}

func (d *DB) Tasks(username string) ([]types.Task, error) {
	var tasks []types.Task

	table := pq.QuoteIdentifier(username)
	query := fmt.Sprintf("SELECT * FROM %v", table)

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var task types.Task
		var date sql.NullTime

		if err := rows.Scan(&task.ID, &task.Body, &date, &task.Project, &task.Context, &task.Done); err != nil {
			return nil, err
		}
		if date.Valid {
			task.Date = date.Time.Format("2006-01-02")
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func (d *DB) AddTask(username string, task types.Task) error {
	table := pq.QuoteIdentifier(username)
	query := fmt.Sprintf("INSERT INTO %v (body, date, project, context, done) VALUES ($1, $2, $3, $4, $5)", table)

	result, err := d.db.Exec(query, task.Body, task.Date, task.Project, task.Context, task.Done)
	if err != nil {
		return err
	}

	_, err = result.RowsAffected()
	if err != nil {
		return err
	}

	return nil
}
