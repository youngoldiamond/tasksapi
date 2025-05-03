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

const secretKey = "my-test-secret-key"

type Task struct {
	ID      int64  `json:"id"`
	Body    string `json:"body"`
	Date    string `json:"date"`
	Project string `json:"project"`
	Context string `json:"context"`
	Done    bool   `json:"done"`
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type User struct {
	ID int64
	Credentials
}

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
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

	router := setupRouter()

	router.Run("localhost:8080")
}

func setupRouter() *gin.Engine {
	router := gin.Default()
	router.POST("/register", register)
	router.GET("/login", login)
	router.GET("/:username/tasks", authMiddleware, getTasks)
	router.POST("/:username/tasks", authMiddleware, postTask)
	router.GET("/:username/tasks/:taskId", authMiddleware, getTaskByID)
	router.PUT("/:username/tasks/:taskId", authMiddleware, updateTask)
	router.DELETE("/:username/tasks/:taskId", authMiddleware, deleteTask)
	router.GET("/:username/projects", authMiddleware, getField("project"))
	router.GET("/:username/contexts", authMiddleware, getField("context"))
	router.GET("/:username/dates", authMiddleware, getField("date"))
	router.GET("/:username/projects/:value", authMiddleware, getByField("project"))
	router.GET("/:username/contexts/:value", authMiddleware, getByField("context"))
	router.GET("/:username/dates/:value", authMiddleware, getByField("date"))
	return router
}

func register(c *gin.Context) {
	var newUser User

	if err := c.BindJSON(&newUser.Credentials); err != nil {
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
		date date,
		project varchar(30),
		context varchar(30),
		done BOOLEAN DEFAULT FALSE
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
	var credentials Credentials

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

	tokeString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"token": tokeString})
}

func authMiddleware(c *gin.Context) {
	tokenString := c.GetHeader("Authorization")
	if tokenString == "" {
		c.IndentedJSON(http.StatusUnauthorized, gin.H{"message": "Missing token"})
		c.Abort()
		return
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		return []byte(secretKey), nil
	})

	if err != nil || !token.Valid {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		c.Abort()
		return
	}

	if claims.Username != c.Param("username") {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "You don't have access"})
		c.Abort()
		return
	}

	c.Next()
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
		var date sql.NullTime

		if err := rows.Scan(&task.ID, &task.Body, &date, &task.Project, &task.Context, &task.Done); err != nil {
			c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
			return
		}
		if date.Valid {
			task.Date = date.Time.Format("2006-01-02")
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
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": "Invalid task's ID"})
		return
	}

	table := pq.QuoteIdentifier(c.Param("username"))
	query := fmt.Sprintf("SELECT * FROM %v WHERE task_id = $1", table)

	var date sql.NullTime

	row := db.QueryRow(query, id)
	if err := row.Scan(&task.ID, &task.Body, &date, &task.Project, &task.Context, &task.Done); err != nil {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	if date.Valid {
		task.Date = date.Time.Format("2006-01-02")
	}
	c.IndentedJSON(http.StatusOK, task)
}

func updateTask(c *gin.Context) {
	var task Task

	id, err := strconv.Atoi(c.Param("taskId"))
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid task's ID"})
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

func deleteTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("taskId"))
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid task's ID"})
		return
	}

	table := pq.QuoteIdentifier(c.Param("username"))
	query := fmt.Sprintf("DELETE FROM %v WHERE task_id = $1", table)

	_, err = db.Exec(query, id)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"message": "Task was deleted successfully"})
}

func getField(fieldName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ans := make([]string, 0, 5)

		table := pq.QuoteIdentifier(c.Param("username"))
		column := pq.QuoteIdentifier(fieldName)
		query := fmt.Sprintf("SELECT DISTINCT %v FROM %v", column, table)

		rows, err := db.Query(query)
		if err != nil {
			c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var value sql.NullString
			if err := rows.Scan(&value); err != nil {
				c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
				return
			}
			if value.Valid {
				if fieldName == "date" {
					value.String = value.String[:10]
				}
				if value.String != "" {
					ans = append(ans, value.String)
				}
			}
		}

		c.IndentedJSON(http.StatusOK, ans)
	}
}

func getByField(fieldName string) func(*gin.Context) {
	return func(c *gin.Context) {
		var tasks []Task

		table := pq.QuoteIdentifier(c.Param("username"))
		column := pq.QuoteIdentifier(fieldName)
		query := fmt.Sprintf("SELECT * FROM %v WHERE %v = $1", table, column)

		rows, err := db.Query(query, c.Param("value"))
		if err != nil {
			c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var task Task
			var date sql.NullTime

			if err := rows.Scan(&task.ID, &task.Body, &date, &task.Project, &task.Context, &task.Done); err != nil {
				c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
				return
			}
			if date.Valid {
				task.Date = date.Time.Format("2006-01-02")
			}
			tasks = append(tasks, task)
		}

		c.IndentedJSON(http.StatusOK, tasks)
	}
}
