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

// Возвращает дефолтную конфигурацию для локального развёртывания
func DefaultConfig() Config {
	return defaultConfig
}

type DB struct {
	db *sql.DB
}

// Открывет базу данных по конфигурации
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

// Закрывает базу данных
func (d *DB) Close() {
	d.db.Close()
}

// Возвращает все задачи пользователя
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

// Добавляет задачу в базу данных
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

// Возвращает задачу пользователя по её ID
func (d *DB) Task(username string, id int64) (*types.Task, error) {
	var task types.Task

	table := pq.QuoteIdentifier(username)
	query := fmt.Sprintf("SELECT * FROM %v WHERE task_id = $1", table)

	var date sql.NullTime

	row := d.db.QueryRow(query, id)
	if err := row.Scan(&task.ID, &task.Body, &date, &task.Project, &task.Context, &task.Done); err != nil {
		return nil, err
	}
	if date.Valid {
		task.Date = date.Time.Format("2006-01-02")
	}

	return &task, nil
}

// Изменяет данные задачи на новые
func (d *DB) UpdateTask(username string, id int64, task types.Task) error {
	table := pq.QuoteIdentifier(username)
	query := fmt.Sprintf("UPDATE %v SET body = $1, date = $2, project = $3, context = $4, done = $5 WHERE task_id = $6", table)

	result, err := d.db.Exec(query, task.Body, task.Date, task.Project, task.Context, task.Done, id)
	if err != nil {
		return err
	}

	_, err = result.RowsAffected()
	if err != nil {
		return err
	}

	return nil
}

// Удаляет задачу по ID
func (d *DB) DeleteTask(username string, id int64) error {
	table := pq.QuoteIdentifier(username)
	query := fmt.Sprintf("DELETE FROM %v WHERE task_id = $1", table)

	result, err := d.db.Exec(query, id)
	if err != nil {
		return err
	}

	_, err = result.RowsAffected()
	if err != nil {
		return err
	}

	return nil
}

// Возвращает все уникальные значения выбранного поля
func (d *DB) Field(username string, fieldName string) ([]string, error) {
	table := pq.QuoteIdentifier(username)
	column := pq.QuoteIdentifier(fieldName)
	query := fmt.Sprintf("SELECT DISTINCT %v FROM %v", column, table)

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := make([]string, 0, 5)

	for rows.Next() {
		var value sql.NullString

		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		if value.Valid {
			if fieldName == "date" {
				value.String = value.String[:10]
			}
			if value.String != "" {
				values = append(values, value.String)
			}
		}
	}

	return values, nil
}

// Возвращает все задачи с определённым значением поля
func (d *DB) TasksByField(username string, fieldName string, value string) ([]types.Task, error) {
	table := pq.QuoteIdentifier(username)
	column := pq.QuoteIdentifier(fieldName)
	query := fmt.Sprintf("SELECT * FROM %v WHERE %v = $1", table, column)

	rows, err := d.db.Query(query, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []types.Task

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

// Добавляет нового пользователя и создаёт таблицу для хранения его задач, присваивает пользователю полученный ID
func (d *DB) AddUser(user *types.User) error {
	query := `INSERT INTO users (username, password) VALUES ($1, $2) RETURNING user_id`

	row := d.db.QueryRow(query, user.Username, user.Password)
	if err := row.Scan(&user.ID); err != nil {
		return err
	}

	createTableQuery := `CREATE TABLE %v (
		task_id serial PRIMARY KEY,
		body varchar(100) NOT NULL,
		date date,
		project varchar(30),
		context varchar(30),
		done BOOLEAN DEFAULT FALSE
	)`
	createTableQuery = fmt.Sprintf(createTableQuery, pq.QuoteIdentifier(user.Username))

	_, err := d.db.Exec(createTableQuery)
	if err != nil {
		return err
	}

	return nil
}

// Возвращает пользователя по имени
func (d *DB) User(username string) (*types.User, error) {
	var user types.User

	row := d.db.QueryRow("SELECT * FROM users WHERE username = $1", username)
	if err := row.Scan(&user.ID, &user.Username, &user.Password); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("db: Invalid username")
		} else {
			return nil, err
		}
	}

	return &user, nil
}
