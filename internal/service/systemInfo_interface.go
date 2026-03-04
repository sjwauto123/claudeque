package service

import "github.com/gorilla/websocket"

type SystemInfoService interface {
	HandleSyMessage(conn *websocket.Conn, userID int)

	Stop()
}
