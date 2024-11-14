package main

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create tables if they don't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS subscribers (
			chat_id INTEGER PRIMARY KEY,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

func (d *Database) AddSubscriber(chatID int64) error {
	_, err := d.db.Exec("INSERT OR IGNORE INTO subscribers (chat_id) VALUES (?)", chatID)
	return err
}

func (d *Database) RemoveSubscriber(chatID int64) error {
	_, err := d.db.Exec("DELETE FROM subscribers WHERE chat_id = ?", chatID)
	return err
}

func (d *Database) GetAllSubscribers() (map[int64]bool, error) {
	rows, err := d.db.Query("SELECT chat_id FROM subscribers")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subscribers := make(map[int64]bool)
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			return nil, err
		}
		subscribers[chatID] = true
	}
	return subscribers, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}
