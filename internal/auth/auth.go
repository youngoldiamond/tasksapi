package auth

import (
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/youngoldiamond/tasksapi/internal/db"
	"github.com/youngoldiamond/tasksapi/internal/types"
)

const secretKey = "my-test-secret-key"

type Config struct {
	ExpirationTime time.Duration
	SecretKey      []byte
}

// Возвращает тестовую конфигурацию с временем истечения в 10 мин. Не использовать в проде
func DefaultConfig() Config {
	return Config{
		ExpirationTime: time.Minute * 10,
		SecretKey:      []byte(secretKey),
	}
}

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

type Auth struct {
	config Config
	db     *db.DB
}

func New(config Config, db *db.DB) *Auth {
	return &Auth{config: config, db: db}
}

// Отдаёт строку jwt токена
func (a *Auth) Login(credentials types.Credentials) (string, error) {
	user, err := a.db.User(credentials.Username)
	if err != nil {
		return "", err
	}

	if user.Password != credentials.Password {
		return "", fmt.Errorf("auth: invalid password")
	}

	expirationTime := time.Now().Add(a.config.ExpirationTime)
	claims := &Claims{
		Username: credentials.Username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(a.config.SecretKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// Проверяет jwt токен и сверяет его с именем пользователя
func (a *Auth) CheckToken(username string, tokenString string) error {
	if tokenString == "" {
		return fmt.Errorf("auth: missing token")
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		return a.config.SecretKey, nil
	})

	if err != nil || !token.Valid {
		return err
	}

	if claims.Username != username {
		return fmt.Errorf("auth: access violation")
	}

	return nil
}
