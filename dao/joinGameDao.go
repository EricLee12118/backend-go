package dao

import (
	"database/sql"
)

type JoinGameDAO interface {
}

type joinGameDAOImpl struct {
	DB *sql.DB
}

func NewJoinGameDAO(db *sql.DB) JoinGameDAO {
	return &joinGameDAOImpl{DB: db}
}
