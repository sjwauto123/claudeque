package service

import (
	"cloudque/internal/model/dto/response"
	"context"

	"github.com/gorilla/websocket"
)

type HomeService interface {
	GetOverview(ctx context.Context) (*response.HomeOverviewResponse, error)
	HandleHomeMessage(conn *websocket.Conn, userID int)
	Stop()
}
