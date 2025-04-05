package models

// Question represents a question from the questions.json file
type Question struct {
	Number      int      `json:"Number"`
	Question    string   `json:"Question"`
	Answers     []string `json:"Answers"`
	RightAnswer int      `json:"Right answer"`
	Category    string   `json:"Category"`
	Image       string   `json:"Image,omitempty"`
}

// UserActivity stores user interaction with questions
type UserActivity struct {
	UserID         int64
	QuestionNumber int
	AnswerNumber   int
	Correct        bool
	Timestamp      int64
}

// DeepseekCache stores cached responses from the Deepseek API
type DeepseekCache struct {
	QuestionNumber int
	Response       string
	RightAnswer    int
}
