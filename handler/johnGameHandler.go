package handler

import (
	"awesomeProject/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

type JoinGameHandler struct {
	GameService service.GameService
}

type JoinGameRequest struct {
	Username string `json:"username" binding:"required,min=3,max=20"`
}

func NewJoinGameHandler(joinGameService service.GameService) *JoinGameHandler {
	return &JoinGameHandler{GameService: joinGameService}
}

func (h *JoinGameHandler) JoinGame(c *gin.Context) {
	var joinGame struct {
		Username string `json:"username" binding:"required"`
	}

	if err := c.ShouldBindJSON(&joinGame); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go func() {
		err := h.GameService.GameListen()
		if err != nil {
			return
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "Game connected",
		"user":    joinGame.Username,
		"code":    http.StatusOK,
	})
}

func (h *JoinGameHandler) StartGame(c *gin.Context) {

}
