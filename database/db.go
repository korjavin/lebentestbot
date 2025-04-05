package database

import (
	"database/sql"
	"time"

	"github.com/korjavin/lebentestbot/models"
	_ "github.com/mattn/go-sqlite3"
)

// DB handles all database operations
type DB struct {
	conn *sql.DB
}

// New creates a new database connection and initializes tables
func New(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	if err = createTables(db); err != nil {
		return nil, err
	}

	return &DB{conn: db}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// createTables creates the necessary tables if they don't exist
func createTables(db *sql.DB) error {
	// Create user activity table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS user_activity (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			question_number INTEGER NOT NULL,
			answer_number INTEGER NOT NULL,
			correct BOOLEAN NOT NULL,
			timestamp INTEGER NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	// Create deepseek cache table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS deepseek_cache (
			question_number INTEGER PRIMARY KEY,
			response TEXT NOT NULL,
			right_answer INTEGER NOT NULL
		)
	`)
	return err
}

// SaveUserActivity records user interaction with a question
func (db *DB) SaveUserActivity(userID int64, questionNumber, answerNumber int, correct bool) error {
	_, err := db.conn.Exec(
		"INSERT INTO user_activity (user_id, question_number, answer_number, correct, timestamp) VALUES (?, ?, ?, ?, ?)",
		userID, questionNumber, answerNumber, correct, time.Now().Unix(),
	)
	return err
}

// GetUserStats retrieves statistics about the user's answers
func (db *DB) GetUserStats(userID int64) (correct int, incorrect int, err error) {
	err = db.conn.QueryRow(
		"SELECT COUNT(*) FROM user_activity WHERE user_id = ? AND correct = 1",
		userID,
	).Scan(&correct)
	if err != nil {
		return 0, 0, err
	}

	err = db.conn.QueryRow(
		"SELECT COUNT(*) FROM user_activity WHERE user_id = ? AND correct = 0",
		userID,
	).Scan(&incorrect)
	return correct, incorrect, err
}

// CacheDeepseekResponse stores a response from Deepseek API
func (db *DB) CacheDeepseekResponse(questionNumber int, response string, rightAnswer int) error {
	_, err := db.conn.Exec(
		"INSERT OR REPLACE INTO deepseek_cache (question_number, response, right_answer) VALUES (?, ?, ?)",
		questionNumber, response, rightAnswer,
	)
	return err
}

// GetCachedDeepseekResponse retrieves a cached response
func (db *DB) GetCachedDeepseekResponse(questionNumber int) (string, int, error) {
	var response string
	var rightAnswer int
	err := db.conn.QueryRow(
		"SELECT response, right_answer FROM deepseek_cache WHERE question_number = ?",
		questionNumber,
	).Scan(&response, &rightAnswer)

	if err == sql.ErrNoRows {
		return "", -1, nil // No cached response
	}

	return response, rightAnswer, err
}

// GetMostFrequentIncorrectQuestions gets the questions most frequently answered incorrectly
func (db *DB) GetMostFrequentIncorrectQuestions(userID int64, limit int) ([]models.UserActivity, error) {
	rows, err := db.conn.Query(`
		SELECT question_number, COUNT(*) as count 
		FROM user_activity 
		WHERE user_id = ? AND correct = 0 
		GROUP BY question_number 
		ORDER BY count DESC 
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.UserActivity
	for rows.Next() {
		var questionNumber, count int
		if err := rows.Scan(&questionNumber, &count); err != nil {
			return nil, err
		}
		result = append(result, models.UserActivity{
			UserID:         userID,
			QuestionNumber: questionNumber,
		})
	}

	return result, nil
}
