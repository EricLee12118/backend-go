package handler

import (
	"awesomeProject/dao"
	"awesomeProject/models"
	"awesomeProject/service"
	"errors"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	UserService service.UserService
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=20"`
	Password string `json:"password" binding:"required,min=6,max=30"`
}

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{UserService: userService}
}

func (h *UserHandler) Hello(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get("user")
	if user != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "Hello from Go Backend! (Authenticated)",
			"user":    user,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"message": "Hello from Go Backend! (Guest)",
		})
	}
}

func (h *UserHandler) Login(c *gin.Context) {
	var loginData struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&loginData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.UserService.AuthenticateUser(loginData.Username, loginData.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials), errors.Is(err, dao.ErrUserNotFound):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		}
		return
	}

	session := sessions.Default(c)
	session.Set("user", user.Username)
	session.Set("userID", user.ID)
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged in successfully",
		"user": gin.H{
			"username": user.Username,
			"id":       user.ID,
		},
	})
}

func (h *UserHandler) Register(c *gin.Context) {
	var req RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		return
	}

	user := &models.User{
		Username: req.Username,
		Password: req.Password,
	}

	h.UserService.Register(user)

	c.JSON(http.StatusOK, gin.H{"message": "Registered successfully"})
}

func (h *UserHandler) Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Options(sessions.Options{MaxAge: -1})
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func (h *UserHandler) ProtectedData(c *gin.Context) {
	session := sessions.Default(c)
	username, _ := session.Get("user").(string)
	userID, _ := session.Get("userID").(int64)

	c.JSON(http.StatusOK, gin.H{
		"message": "This is protected data",
		"user": gin.H{
			"username": username,
			"id":       userID,
		},
	})
}
