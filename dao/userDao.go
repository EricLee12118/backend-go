package dao

import (
	"awesomeProject/models"
	"database/sql"
	"errors"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

type UserDAO interface {
	GetUserByUsername(username string) (*models.User, error)
	CreateUser(user *models.User) error
}

type userDAOImpl struct {
	DB *sql.DB
}

func (dao *userDAOImpl) CreateUser(user *models.User) error {
	query := "INSERT INTO users (username, password) VALUES (?, ?)"
	_, err := dao.DB.Exec(query, user.Username, user.Password)
	return err
}

func NewUserDAO(db *sql.DB) UserDAO {
	return &userDAOImpl{DB: db}
}

func (dao *userDAOImpl) GetUserByUsername(username string) (*models.User, error) {
	query := "SELECT id, username, password FROM users WHERE username = ?"
	user := &models.User{}
	err := dao.DB.QueryRow(query, username).Scan(&user.ID, &user.Username, &user.Password)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}
