package api

import (
	"fmt"
	"log"
	"time"

	"rip/internal/pkg/api/middleware"
	"rip/internal/pkg/redis"
	"rip/internal/pkg/repo"

	"github.com/gin-gonic/gin"
)

type JWTConfig struct {
	ExpiresIn time.Duration
	Secret    string
}

type Server struct {
	host string
	port int

	jwtConfig JWTConfig

	calculateSecret   string
	calculateCallback string
}

func WithHost(host string) func(*Server) {
	return func(s *Server) {
		s.host = host
	}
}

func WithPort(port int) func(*Server) {
	return func(s *Server) {
		s.port = port
	}
}

func WithJWTConfig(c JWTConfig) func(*Server) {
	return func(s *Server) {
		s.jwtConfig = c
	}
}

func WithCalculate(callback, secret string) func(*Server) {
	return func(s *Server) {
		s.calculateSecret = secret
		s.calculateCallback = callback
	}
}

func NewServer(options ...func(*Server)) *Server {
	srv := &Server{}
	for _, o := range options {
		o(srv)
	}
	return srv
}

// TODO: add abort() to errors
func (s *Server) StartServer(rep repo.Repository, avatar repo.Avatar, redis *redis.RedisClient) {
	log.Println("Server start up")

	moderatorMiddleware := []gin.HandlerFunc{middleware.WithAuthCheck(s.jwtConfig.Secret, redis), middleware.WithModeratorCheck}
	userMiddleware := middleware.WithAuthCheck(s.jwtConfig.Secret, redis)

	r := gin.Default()
	api := r.Group("/api")

	dataService := api.Group("/dataService")
	dataService.GET("/", filterDataService(rep))
	dataService.GET("/:id", getDataServiceByID(rep))

	dataService. /*.Use(moderatorMiddleware...)*/ POST("/:id/image", putImage(rep, avatar))
	dataService.Use(moderatorMiddleware...).POST("/", createDataService(rep))
	dataService.Use(moderatorMiddleware...).DELETE("/:id", deleteDataService(rep, avatar))
	dataService.Use(moderatorMiddleware...).PUT("/", updateDataService(rep))

	dataService.Use(userMiddleware).POST("/draft/:id", addToDraft(rep))
	dataService.Use(userMiddleware).DELETE("/draft/:id", deleteFromDraft(rep)) //

	encDecRequest := api.Group("/encryptDecryptRequest")
	encDecRequest.GET("/filter", getEncryptDecryptRequests(rep))
	encDecRequest.GET("/:id", getEncryptDecryptRequestsByID(rep))

	encDecRequest.PUT("/update_calculated", calculated(rep, s.calculateSecret))

	encDecRequest.Use(userMiddleware).PUT("/form/:id", formEncryptDecryptRequest(rep, s.makeCalculationRequest))
	encDecRequest.Use(userMiddleware).DELETE("/:req_id", deleteEncryptDecryptRequest(rep))
	encDecRequest.Use(userMiddleware).DELETE("/:req_id/delete/:data_id", deleteDataFromEncryptDecryptRequest(rep))
	encDecRequest.Use(moderatorMiddleware...).PUT("/update_moderator/:id", updateModeratorEncryptDecryptRequest(rep))

	auth := api.Group("/auth")
	auth.POST("/login", login(rep, s.jwtConfig.Secret, s.jwtConfig.ExpiresIn))
	auth.POST("/register", register(rep))

	// удаление услуги из заявки + мб тогда delete draft не нужен
	// TODO: get draft???

	r.Run(fmt.Sprintf("%s:%d", s.host, s.port))
	log.Println("Server down")
}