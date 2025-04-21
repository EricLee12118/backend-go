// package main
//
// import (
//
//	"database/sql"
//	"errors"
//	"github.com/gin-contrib/cors"
//	"github.com/gin-contrib/sessions"
//	"github.com/gin-contrib/sessions/cookie"
//	"github.com/gin-gonic/gin"
//	"log"
//	"net/http"
//
// )
//
//	func main() {
//		r := gin.Default()
//		config := cors.DefaultConfig()
//		config.AllowOrigins = []string{"http://localhost:3000"}
//		config.AllowCredentials = true
//		r.Use(cors.New(config))
//
//		store := cookie.NewStore([]byte("secret-key"))
//		r.Use(sessions.Sessions("mysession", store))
//		r.GET("/api/hello", func(c *gin.Context) {
//			session := sessions.Default(c)
//			user := session.Get("user")
//			if user != nil {
//				c.JSON(200, gin.H{
//					"message": "Hello from Go Backend! (Authenticated)",
//					"user":    user,
//				})
//			} else {
//				c.JSON(200, gin.H{
//					"message": "Hello from Go Backend! (Guest)",
//				})
//			}
//		})
//
//		r.POST("/api/login", func(c *gin.Context) {
//			var loginData struct {
//				Username string `json:"username" binding:"required"`
//				Password string `json:"password" binding:"required"`
//			}
//
//			if err := c.ShouldBindJSON(&loginData); err != nil {
//				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
//				return
//			}
//
//			db, err := initDB()
//			if err != nil {
//				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection error"})
//				return
//			}
//
//			defer func(db *sql.DB) {
//				err := db.Close()
//				if err != nil {
//
//				}
//			}(db)
//
//			var user User
//			query := "SELECT id, username, password FROM users WHERE username = ?"
//			err = db.QueryRow(query, loginData.Username).Scan(&user.ID, &user.Username, &user.Password)
//
//			if err != nil {
//				if errors.Is(err, sql.ErrNoRows) {
//					c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
//				} else {
//					c.JSON(http.StatusInternalServerError, gin.H{"error": "Database query error"})
//				}
//				return
//			}
//
//			if user.Password != loginData.Password {
//				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
//				return
//			}
//
//			session := sessions.Default(c)
//			session.Set("user", user.Username)
//			session.Set("userID", user.ID) // 存储用户ID
//			if err := session.Save(); err != nil {
//				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
//				return
//			}
//
//			c.JSON(http.StatusOK, gin.H{
//				"message": "Logged in successfully",
//				"user": gin.H{
//					"username": user.Username,
//					"id":       user.ID,
//				},
//			})
//		})
//
//		r.POST("/api/logout", func(c *gin.Context) {
//			session := sessions.Default(c)
//			session.Clear()
//			session.Options(sessions.Options{MaxAge: -1})
//			if err := session.Save(); err != nil {
//				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
//				return
//			}
//			c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
//		})
//
//		authGroup := r.Group("/api/protected")
//		authGroup.Use(AuthRequired)
//		{
//			authGroup.GET("/data", func(c *gin.Context) {
//				session := sessions.Default(c)
//				username := session.Get("user").(string)
//				userID := session.Get("userID").(int64)
//
//				c.JSON(http.StatusOK, gin.H{
//					"message": "This is protected data",
//					"user": gin.H{
//						"username": username,
//						"id":       userID,
//					},
//				})
//			})
//		}
//
//		err := r.Run(":8080")
//		if err != nil {
//			log.Fatal("Error starting server: ", err)
//		}
//	}
//
// main.go
package main

import (
	"log"
	"os"

	"backend/api"
	"backend/config"
	"backend/dao"
	"backend/middleware"
	"backend/service"

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
	userHandler := api.NewUserHandler(userService)

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

	// 定义路由
	r.GET("/api/hello", userHandler.Hello)
	r.POST("/api/login", userHandler.Login)
	r.POST("/api/logout", userHandler.Logout)
	r.POST("api/register", userHandler.Register)

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
