package database

import (
	"database/sql"
	"sort"
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

// GetUnansweredQuestions returns questions that the user has never answered
func (db *DB) GetUnansweredQuestions(userID int64, allQuestions []models.Question) ([]models.Question, error) {
	// Get all question numbers that the user has answered
	rows, err := db.conn.Query(
		"SELECT DISTINCT question_number FROM user_activity WHERE user_id = ?",
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Create a set of answered questions
	answeredQuestions := make(map[int]bool)
	for rows.Next() {
		var questionNumber int
		if err := rows.Scan(&questionNumber); err != nil {
			return nil, err
		}
		answeredQuestions[questionNumber] = true
	}

	// Filter out questions that have been answered
	var unansweredQuestions []models.Question
	for _, question := range allQuestions {
		if !answeredQuestions[question.Number] {
			unansweredQuestions = append(unansweredQuestions, question)
		}
	}

	return unansweredQuestions, nil
}

// GetLeastRecentlyAnsweredQuestions returns questions ordered by how long ago they were last answered
func (db *DB) GetLeastRecentlyAnsweredQuestions(userID int64, allQuestions []models.Question) ([]models.Question, error) {
	// Get the most recent timestamp for each question
	rows, err := db.conn.Query(`
		SELECT question_number, MAX(timestamp) as last_answered
		FROM user_activity 
		WHERE user_id = ? 
		GROUP BY question_number 
		ORDER BY last_answered ASC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Map of question number to last answered timestamp
	lastAnswered := make(map[int]int64)
	for rows.Next() {
		var questionNumber int
		var timestamp int64
		if err := rows.Scan(&questionNumber, &timestamp); err != nil {
			return nil, err
		}
		lastAnswered[questionNumber] = timestamp
	}

	// Create a map for quick lookup of questions by number
	questionMap := make(map[int]models.Question)
	for _, q := range allQuestions {
		questionMap[q.Number] = q
	}

	// Create a slice of questions ordered by last answered time
	type questionWithTime struct {
		question  models.Question
		timestamp int64
	}

	var questionsWithTime []questionWithTime

	// First, add questions that have been answered before, ordered by time
	for qNum, timestamp := range lastAnswered {
		if q, exists := questionMap[qNum]; exists {
			questionsWithTime = append(questionsWithTime, questionWithTime{q, timestamp})
		}
	}

	// Sort by timestamp (oldest first)
	sort.Slice(questionsWithTime, func(i, j int) bool {
		return questionsWithTime[i].timestamp < questionsWithTime[j].timestamp
	})

	// Extract just the questions in order
	result := make([]models.Question, len(questionsWithTime))
	for i, qwt := range questionsWithTime {
		result[i] = qwt.question
	}

	return result, nil
}
