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
			chat_id INTEGER PRIMARY KEY
		);
		CREATE TABLE IF NOT EXISTS user_preferences (
			chat_id INTEGER PRIMARY KEY,
			show_cex BOOLEAN NOT NULL DEFAULT 1,
			FOREIGN KEY(chat_id) REFERENCES subscribers(chat_id)
		);
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

func (d *Database) SetShowCEX(chatID int64, show bool) error {
	_, err := d.db.Exec(`
		INSERT INTO user_preferences (chat_id, show_cex)
		VALUES (?, ?)
		ON CONFLICT(chat_id) DO UPDATE SET show_cex = ?`,
		chatID, show, show)
	return err
}

func (d *Database) GetShowCEX(chatID int64) (bool, error) {
	var show bool
	err := d.db.QueryRow(`
		SELECT COALESCE(
			(SELECT show_cex FROM user_preferences WHERE chat_id = ?),
			1
		)`,
		chatID).Scan(&show)
	if err != nil {
		return true, err // Default to showing CEX if there's an error
	}
	return show, nil
}

func (d *Database) LoadPreferences() (map[int64]bool, error) {
	preferences := make(map[int64]bool)
	rows, err := d.db.Query(`
		SELECT chat_id, show_cex 
		FROM user_preferences`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var chatID int64
		var showCEX bool
		if err := rows.Scan(&chatID, &showCEX); err != nil {
			return nil, err
		}
		preferences[chatID] = showCEX
	}
	return preferences, nil
}
