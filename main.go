package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
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

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

func register(c *gin.Context) {
	var newUser User

	if err := c.BindJSON(&newUser); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	query := `INSERT INTO users (username, password) VALUES ($1, $2) RETURNING user_id`
	err := db.QueryRow(query, newUser.Username, newUser.Password).Scan(&newUser.ID)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	createTableQuery := `CREATE TABLE %v (
		task_id serial PRIMARY KEY,
		body varchar(100) NOT NULL,
		date varchar(10),
		project varchar(30),
		context varchar(30),
		done BOOLEAN
	)`
	createTableQuery = fmt.Sprintf(createTableQuery, pq.QuoteIdentifier(newUser.Username))

	_, err = db.Exec(createTableQuery)
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"message": "User registered successfully"})
}

func login(c *gin.Context) {
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.BindJSON(&credentials); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	var user User
	row := db.QueryRow("SELECT * FROM users WHERE username = $1", credentials.Username)
	if err := row.Scan(&user.ID, &user.Username, &user.Password); err != nil {
		var mess string
		if err == sql.ErrNoRows {
			mess = "Invalid username"
		} else {
			mess = err.Error()
		}
		c.IndentedJSON(http.StatusUnauthorized, gin.H{"message": mess})
		return
	}

	if user.Password != credentials.Password {
		c.IndentedJSON(http.StatusUnauthorized, gin.H{"message": "Invalid password"})
	}

	expirationTime := time.Now().Add(time.Minute * 5)
	claims := &Claims{
		Username: credentials.Username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokeString, err := token.SignedString([]byte("my-test-secret-key"))
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"token": tokeString})
}

func getTasks(c *gin.Context) {
	var tasks []Task

	table := pq.QuoteIdentifier(c.Param("username"))
	query := fmt.Sprintf("SELECT * FROM %v", table)

	rows, err := db.Query(query)
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

	table := pq.QuoteIdentifier(c.Param("username"))
	query := fmt.Sprintf("INSERT INTO %v (body, date, project, context, done) VALUES ($1, $2, $3, $4, $5)", table)

	result, err := db.Exec(query, newTask.Body, newTask.Date, newTask.Project, newTask.Context, newTask.Done)
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
	id, err := strconv.Atoi(c.Param("taskId"))
	if err != nil {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	table := pq.QuoteIdentifier(c.Param("username"))
	query := fmt.Sprintf("SELECT * FROM %v WHERE task_id = $1", table)

	row := db.QueryRow(query, id)
	if err := row.Scan(&task.ID, &task.Body, &task.Date, &task.Project, &task.Context, &task.Done); err != nil {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, task)
}

func updateTask(c *gin.Context) {
	var task Task

	id, err := strconv.Atoi(c.Param("taskId"))
	if err != nil {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	if err := c.BindJSON(&task); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	table := pq.QuoteIdentifier(c.Param("username"))
	query := fmt.Sprintf("UPDATE %v SET body = $1, date = $2, project = $3, context = $4, done = $5 WHERE task_id = $6", table)

	_, err = db.Exec(query, task.Body, task.Date, task.Project, task.Context, task.Done, id)
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
		log.Fatal(err)
	}
	defer db.Close()

	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
	}
	log.Println("Connected to DB")

	router := gin.Default()
	router.POST("/register", register)
	router.GET("/login", login)
	router.GET("/:username/tasks", getTasks)
	router.GET("/:username/tasks/:taskId", getTaskByID)
	router.POST("/:username/tasks", postTask)
	router.PUT("/:username/tasks/:taskId", updateTask)

	router.Run("localhost:8080")
}
