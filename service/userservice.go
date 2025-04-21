// service/userservice.go
package service

import (
	"errors"

	"backend/dao"
	"backend/models"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
)

type UserService interface {
	AuthenticateUser(username, password string) (*models.User, error)
	Register(user *models.User) error
}

type userServiceImpl struct {
	UserDAO dao.UserDAO
}

func (s *userServiceImpl) Register(user *models.User) error {
	err := s.UserDAO.CreateUser(user)
	if err != nil {
		return err
	}
	return nil
}

func NewUserService(userDAO dao.UserDAO) UserService {
	return &userServiceImpl{UserDAO: userDAO}
}

func (s *userServiceImpl) AuthenticateUser(username, password string) (*models.User, error) {
	user, err := s.UserDAO.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}

	// 比较密码哈希
	//err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	//if err != nil {
	//	return nil, ErrInvalidCredentials
	//}

	return user, nil
}
