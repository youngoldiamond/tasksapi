package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var db *sql.DB

type Task struct {
	ID      int64  `json:"id"`
	Body    string `json:"body"`
	Date    string `json:"date"`
	Project string `json:"project"`
	Context string `json:"context"`
	Done    bool   `json:"done"`
}

func getTasks(c *gin.Context) {
	var tasks []Task

	rows, err := db.Query("SELECT * FROM tasks")
	if err != nil {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	defer rows.Close()
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.Body, &task.Date, &task.Project, &task.Context, &task.Done); err != nil {
			c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
			return
		}
		tasks = append(tasks, task)
	}

	c.IndentedJSON(http.StatusOK, tasks)
}

func postTask(c *gin.Context) {
	var newTask Task

	if err := c.BindJSON(&newTask); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	row := db.QueryRow("SELECT * FROM tasks WHERE task_id = $1", newTask.ID)
	if row.Err() == nil {
		c.IndentedJSON(http.StatusOK, gin.H{"message": "Task already exists. Try POST tasks/:id if you want to update"})
		return
	}

	result, err := db.Exec("INSERT INTO tasks (body, date, project, context, done) VALUES ($1, $2, $3, $4, $5)", newTask.Body, newTask.Date, newTask.Project, newTask.Context, newTask.Done)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	_, err = result.RowsAffected()
	if err != nil {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusCreated, newTask)
}

func getTaskByID(c *gin.Context) {
	var task Task
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	row := db.QueryRow("SELECT * FROM tasks WHERE task_id = $1", id)
	if err := row.Scan(&task.ID, &task.Body, &task.Date, &task.Project, &task.Context, &task.Done); err != nil {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, task)
}

func updateTask(c *gin.Context) {
	var task Task

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	if err := c.BindJSON(&task); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	_, err = db.Exec("UPDATE tasks SET body = $1, date = $2, project = $3, context = $4, done = $5 WHERE task_id = $6", task.Body, task.Date, task.Project, task.Context, task.Done, id)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusCreated, task)
}

func main() {
	pass := os.Getenv("MYPASS")
	connStr := fmt.Sprintf("user=postgres password=%s dbname=tasks sslmode=disable", pass)
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	pingErr := db.Ping()
	if pingErr != nil {
		panic(pingErr)
	}
	log.Println("Connected to DB")

	router := gin.Default()
	router.GET("/tasks", getTasks)
	router.GET("/tasks/:id", getTaskByID)
	router.POST("/tasks", postTask)
	router.POST("/tasks/:id", updateTask)

	router.Run("localhost:8080")
}
