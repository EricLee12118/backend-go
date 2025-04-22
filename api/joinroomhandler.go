package api

import (
	"backend/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

type JoinRoomHandler struct {
	RoomService service.RoomService
}

type JoinRoomRequest struct {
	Username string `json:"username" binding:"required,min=3,max=20"`
}

func NewJoinRoomHandler(joinRoomService service.RoomService) *JoinRoomHandler {
	return &JoinRoomHandler{RoomService: joinRoomService}
}

func (h *JoinRoomHandler) JoinRoom(c *gin.Context) {
	var joinRoom struct {
		Username string `json:"username" binding:"required"`
	}

	if err := c.ShouldBindJSON(&joinRoom); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go func() {
		err := h.RoomService.RoomListen()
		if err != nil {
			return
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "room connected",
		"user":    joinRoom.Username,
		"code":    http.StatusOK,
	})
}
