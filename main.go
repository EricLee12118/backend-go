package main

import (
	"log"
	"os"

	"awesomeProject/config"
	"awesomeProject/dao"
	"awesomeProject/handler"
	"awesomeProject/middleware"
	"awesomeProject/service"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func main() {
	// 初始化数据库
	config.InitDB()
	defer config.CloseDB()

	db := config.DB

	// 初始化 DAO 和 Service
	userDAO := dao.NewUserDAO(db)
	userService := service.NewUserService(userDAO)
	userHandler := handler.NewUserHandler(userService)

	joinGameDAO := dao.NewJoinGameDAO(db)
	joinGameService := service.NewJoinGameService(joinGameDAO)
	joinGameHandler := handler.NewJoinGameHandler(joinGameService)

	// 初始化 Gin 路由
	r := gin.Default()

	// 配置 CORS
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"http://localhost:3000"}
	corsConfig.AllowCredentials = true
	r.Use(cors.New(corsConfig))

	// 配置 Session
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		secret = "default-secret-key" // 或者退出程序
		log.Println("Warning: SESSION_SECRET not set, using default secret key")
	}
	store := cookie.NewStore([]byte(secret))
	r.Use(sessions.Sessions("mysession", store))

	// User
	r.GET("/api/hello", userHandler.Hello)
	r.POST("/api/login", userHandler.Login)
	r.POST("/api/logout", userHandler.Logout)
	r.POST("api/register", userHandler.Register)

	// Game
	r.POST("/api/join-game", joinGameHandler.JoinGame)
	r.POST("/api/start-game/", joinGameHandler.StartGame)
	protected := r.Group("/api/protected")
	protected.Use(middleware.AuthRequired)
	{
		protected.GET("/data", userHandler.ProtectedData)
	}

	// 启动服务器
	if err := r.Run(":8080"); err != nil {
		log.Fatal("Error starting server: ", err)
	}
}
