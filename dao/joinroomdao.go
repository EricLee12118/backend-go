package dao

import (
	"database/sql"
)

type JoinRoomDAO interface {
}

type joinRoomDAOImpl struct {
	DB *sql.DB
}

func NewJoinRoomDAO(db *sql.DB) JoinRoomDAO {
	return &joinRoomDAOImpl{DB: db}
}
