package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/youngoldiamond/tasksapi/internal/db"
	"github.com/youngoldiamond/tasksapi/internal/types"
)

var DB *db.DB

const secretKey = "my-test-secret-key"

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

func main() {
	cfg := db.DefaultConfig()
	cfg.Password = os.Getenv("MYPASS")

	var err error
	DB, err = db.Open(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer DB.Close()

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
	var newUser types.User

	if err := c.BindJSON(&newUser.Credentials); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	if err := DB.AddUser(&newUser); err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"message": "User registered successfully"})
}

func login(c *gin.Context) {
	var credentials types.Credentials

	if err := c.BindJSON(&credentials); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	user, err := DB.User(credentials.Username)
	if err != nil {
		c.IndentedJSON(http.StatusUnauthorized, gin.H{"message": err.Error()})
		return
	}

	if user.Password != credentials.Password {
		c.IndentedJSON(http.StatusUnauthorized, gin.H{"message": "Invalid password"})
		return
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

	tasks, err := DB.Tasks(c.Param("username"))
	if err != nil {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, tasks)
}

func postTask(c *gin.Context) {
	var newTask types.Task

	if err := c.BindJSON(&newTask); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	if err := DB.AddTask(c.Param("username"), newTask); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusCreated, newTask)
}

func getTaskByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": "Invalid task's ID"})
		return
	}

	task, err := DB.Task(c.Param("username"), id)
	if err != nil {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, task)
}

func updateTask(c *gin.Context) {
	var task types.Task

	id, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid task's ID"})
		return
	}

	if err := c.BindJSON(&task); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	if err := DB.UpdateTask(c.Param("username"), id, task); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusCreated, task)
}

func deleteTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": "Invalid task's ID"})
		return
	}

	if err := DB.DeleteTask(c.Param("username"), id); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"message": "Task was deleted successfully"})
}

func getField(fieldName string) gin.HandlerFunc {
	return func(c *gin.Context) {

		values, err := DB.Field(c.Param("username"), fieldName)
		if err != nil {
			c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
			return
		}

		c.IndentedJSON(http.StatusOK, values)
	}
}

func getByField(fieldName string) func(*gin.Context) {
	return func(c *gin.Context) {

		tasks, err := DB.TasksByField(c.Param("username"), fieldName, c.Param("value"))
		if err != nil {
			c.IndentedJSON(http.StatusNotFound, gin.H{"message": err.Error()})
			return
		}

		c.IndentedJSON(http.StatusOK, tasks)
	}
}
