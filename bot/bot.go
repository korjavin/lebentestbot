package bot

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/lebentestbot/ai"
	"github.com/korjavin/lebentestbot/config"
	"github.com/korjavin/lebentestbot/database"
	"github.com/korjavin/lebentestbot/models"
)

// Bot represents the Telegram bot
type Bot struct {
	api           *tgbotapi.BotAPI
	db            *database.DB
	deepseek      *ai.DeepseekClient
	questions     []models.Question
	userQuestions map[int64]int // Maps user IDs to their current question number
}

const (
	cmdStart = "start"
	cmdNext  = "next"
	cmdHelp  = "help"
	cmdStat  = "stat"

	callbackPrefix = "answer:"
)

// New creates a new bot instance
func New(cfg *config.Config) (*Bot, error) {
	// Create bot API
	botAPI, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}

	// Set bot debugging mode
	botAPI.Debug = os.Getenv("DEBUG") == "true"

	// Initialize database
	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Load questions
	questions, err := loadQuestions()
	if err != nil {
		return nil, fmt.Errorf("failed to load questions: %w", err)
	}

	log.Printf("Loaded %d questions", len(questions))

	return &Bot{
		api:           botAPI,
		db:            db,
		deepseek:      ai.NewDeepseekClient(cfg.DeepseekAPIKey),
		questions:     questions,
		userQuestions: make(map[int64]int),
	}, nil
}

// loadQuestions loads questions from the JSON file
func loadQuestions() ([]models.Question, error) {
	file, err := os.ReadFile("assets/questions.json")
	if err != nil {
		return nil, err
	}

	var questions []models.Question
	if err := json.Unmarshal(file, &questions); err != nil {
		return nil, err
	}

	// Filter out questions with Number -1 as they might be broken
	var validQuestions []models.Question
	for _, q := range questions {
		if q.Number != -1 {
			validQuestions = append(validQuestions, q)
		}
	}

	return validQuestions, nil
}

// Start starts the bot and listens for updates
func (b *Bot) Start() {
	log.Println("Starting bot polling...")

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			b.handleCallback(update.CallbackQuery)
		} else if update.Message != nil {
			b.handleMessage(update.Message)
		}
	}
}

// handleMessage processes incoming messages
func (b *Bot) handleMessage(message *tgbotapi.Message) {
	userID := message.From.ID
	log.Printf("Received message from %s (ID: %d): %s", message.From.UserName, userID, message.Text)

	switch {
	case strings.HasPrefix(message.Text, "/"+cmdStart):
		b.handleStartCommand(message)
	case strings.HasPrefix(message.Text, "/"+cmdNext):
		b.handleNextCommand(message)
	case strings.HasPrefix(message.Text, "/"+cmdHelp):
		b.handleHelpCommand(message)
	case strings.HasPrefix(message.Text, "/"+cmdStat):
		b.handleStatCommand(message)
	default:
		// Send a help message for unknown commands
		b.sendMessage(message.Chat.ID, "Unknown command. Use /start to begin, /next for a new question, or /help for assistance.")
	}
}

// handleStartCommand handles the /start command
func (b *Bot) handleStartCommand(message *tgbotapi.Message) {
	welcomeText := `Welcome to LebenTestBot! 

This bot will help you practice for your German test by presenting questions from the test material.

Commands:
/start - Start the bot and get a random question
/next - Get another random question
/help - Get assistance with the current question
/stat - View your statistics

Let's begin with your first question!`

	b.sendMessage(message.Chat.ID, welcomeText)

	// Send a random question
	b.sendRandomQuestion(message.Chat.ID)
}

// handleNextCommand handles the /next command
func (b *Bot) handleNextCommand(message *tgbotapi.Message) {
	b.sendRandomQuestion(message.Chat.ID)
}

// handleHelpCommand handles the /help command
func (b *Bot) handleHelpCommand(message *tgbotapi.Message) {
	questionNum, exists := b.userQuestions[message.From.ID]
	if !exists {
		b.sendMessage(message.Chat.ID, "Please use /start to get your first question before asking for help.")
		return
	}

	// Find the current question for this user
	var currentQuestion *models.Question
	for i := range b.questions {
		if b.questions[i].Number == questionNum {
			currentQuestion = &b.questions[i]
			break
		}
	}

	if currentQuestion == nil {
		b.sendMessage(message.Chat.ID, "Sorry, I couldn't find your current question. Please use /next to get a new question.")
		return
	}

	// Try to get cached response first
	cachedResponse, rightAnswer, err := b.db.GetCachedDeepseekResponse(questionNum)
	if err != nil {
		log.Printf("Error retrieving cached response: %v", err)
	}

	if cachedResponse != "" {
		b.sendMessage(message.Chat.ID, "Here's some help with this question:\n\n"+cachedResponse)
		return
	}

	// If no cached response, call Deepseek API
	b.sendMessage(message.Chat.ID, "Analyzing this question, please wait a moment...")

	response, rightAnswer, err := b.deepseek.AnalyzeQuestion(currentQuestion)
	if err != nil {
		log.Printf("Error calling Deepseek API: %v", err)
		b.sendMessage(message.Chat.ID, "Sorry, I couldn't analyze this question. Please try again later.")
		return
	}

	// Cache the response
	if err := b.db.CacheDeepseekResponse(questionNum, response, rightAnswer); err != nil {
		log.Printf("Error caching Deepseek response: %v", err)
	}

	b.sendMessage(message.Chat.ID, "Here's some help with this question:\n\n"+response)
}

// handleStatCommand handles the /stat command
func (b *Bot) handleStatCommand(message *tgbotapi.Message) {
	correct, incorrect, err := b.db.GetUserStats(message.From.ID)
	if err != nil {
		log.Printf("Error getting user stats: %v", err)
		b.sendMessage(message.Chat.ID, "Sorry, I couldn't retrieve your statistics. Please try again later.")
		return
	}

	total := correct + incorrect
	var accuracy float64
	if total > 0 {
		accuracy = float64(correct) / float64(total) * 100
	}

	statMessage := fmt.Sprintf(`📊 Your Statistics:

Total Questions Attempted: %d
Correct Answers: %d ✅
Incorrect Answers: %d ❌
Accuracy: %.1f%%`, total, correct, incorrect, accuracy)

	if total > 0 {
		// Get most frequently incorrect questions
		incorrectQuestions, err := b.db.GetMostFrequentIncorrectQuestions(message.From.ID, 3)
		if err != nil {
			log.Printf("Error getting incorrect questions: %v", err)
		}

		if len(incorrectQuestions) > 0 {
			statMessage += "\n\nMost Challenging Questions:\n"
			for i, q := range incorrectQuestions {
				for _, question := range b.questions {
					if question.Number == q.QuestionNumber {
						// Truncate long questions
						questionText := question.Question
						if len(questionText) > 50 {
							questionText = questionText[:47] + "..."
						}
						statMessage += fmt.Sprintf("%d. Question #%d: %s\n", i+1, question.Number, questionText)
						break
					}
				}
			}
		}
	}

	b.sendMessage(message.Chat.ID, statMessage)
}

// handleCallback processes callback queries from inline buttons
func (b *Bot) handleCallback(callback *tgbotapi.CallbackQuery) {
	startTime := time.Now()
	log.Printf("Handling callback from user %s (ID: %d) with data: %s",
		callback.From.UserName, callback.From.ID, callback.Data)

	if !strings.HasPrefix(callback.Data, callbackPrefix) {
		log.Printf("Invalid callback prefix: %s", callback.Data)
		return
	}

	// Extract answer number from callback data
	parts := strings.Split(strings.TrimPrefix(callback.Data, callbackPrefix), ":")
	if len(parts) != 2 {
		log.Printf("Invalid callback format: %s", callback.Data)
		return
	}

	questionNum, err := strconv.Atoi(parts[0])
	if err != nil {
		log.Printf("Invalid question number in callback: %v", err)
		return
	}

	answerNum, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Printf("Invalid answer number in callback: %v", err)
		return
	}

	log.Printf("User selected answer %d for question %d", answerNum, questionNum)

	// Always acknowledge the callback immediately to prevent "query is too old" errors
	b.sendCallbackResponse(callback.ID, "Processing your answer...")

	// Find the question
	var question *models.Question
	for i := range b.questions {
		if b.questions[i].Number == questionNum {
			question = &b.questions[i]
			break
		}
	}

	if question == nil {
		log.Printf("Question %d not found", questionNum)
		b.sendMessage(callback.Message.Chat.ID, "Sorry, this question is no longer available.")
		return
	}

	// First, check if we have a cached response to determine the right answer
	cachedResponse := ""
	rightAnswer := question.RightAnswer
	isCorrect := false

	// Try to get cached response first to avoid API calls
	log.Printf("Checking for cached response for question %d", questionNum)
	cachedResp, cachedRightAnswer, err := b.db.GetCachedDeepseekResponse(questionNum)
	if err == nil && cachedRightAnswer != -1 {
		log.Printf("Found cached response for question %d with right answer: %d",
			questionNum, cachedRightAnswer)
		rightAnswer = cachedRightAnswer
		cachedResponse = cachedResp
	} else {
		log.Printf("No cached response found for question %d or error: %v", questionNum, err)
	}

	// Determine if the answer is correct based on what we know
	if rightAnswer != -1 {
		isCorrect = (answerNum == rightAnswer)
		log.Printf("Answer correctness determined: %v (user: %d, correct: %d)",
			isCorrect, answerNum, rightAnswer)
	}

	// Save the user activity
	if err := b.db.SaveUserActivity(callback.From.ID, questionNum, answerNum, isCorrect); err != nil {
		log.Printf("Error saving user activity: %v", err)
	} else {
		log.Printf("Saved user activity for question %d", questionNum)
	}

	// Prepare initial response message
	var responseText string

	if rightAnswer != -1 {
		// We already know the right answer, respond immediately
		if isCorrect {
			responseText = "✅ Correct! Well done!\n\nUse /help to get more information about this question or /next for a new question."
		} else {
			correctAnswerText := "Unknown"
			if rightAnswer >= 0 && rightAnswer < len(question.Answers) {
				correctAnswerText = question.Answers[rightAnswer]
			}
			responseText = fmt.Sprintf("❌ Sorry, that's not correct. The right answer is: %s\n\nUse /help to get more information or /next for a new question.", correctAnswerText)
		}

		b.sendMessage(callback.Message.Chat.ID, responseText)
		log.Printf("Sent immediate response for question %d (%.2fs)",
			questionNum, time.Since(startTime).Seconds())
		return
	}

	// If we don't know the right answer yet and there's no cached response,
	// inform the user we're processing their answer, but don't wait for Deepseek
	userAnswer := "Unknown"
	if answerNum >= 0 && answerNum < len(question.Answers) {
		userAnswer = question.Answers[answerNum]
	}

	// Send initial message and store the message ID for later editing
	initialMsg := fmt.Sprintf("Your answer: \"%s\"\n\nAnalyzing...", userAnswer)
	sentMsg, err := b.api.Send(tgbotapi.NewMessage(callback.Message.Chat.ID, initialMsg))
	if err != nil {
		log.Printf("Error sending initial message: %v", err)
		return
	}
	initialMessageID := sentMsg.MessageID
	log.Printf("Sent initial message with ID %d", initialMessageID)

	// Launch a goroutine to handle the Deepseek API call without blocking
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from panic in Deepseek goroutine: %v", r)
			}
		}()

		log.Printf("Starting async Deepseek analysis for question %d (may take up to 60s)", questionNum)

		// Check again if we have a cached response (might have been added by another request)
		cachedResp, cachedRightAnswer, err := b.db.GetCachedDeepseekResponse(questionNum)
		if err == nil && cachedRightAnswer != -1 && cachedResp != "" {
			log.Printf("Found cached response in async handler for question %d", questionNum)
			rightAnswer = cachedRightAnswer
			cachedResponse = cachedResp
		} else if cachedResponse == "" {
			// No cached response, call Deepseek API with longer timeout
			resp, rightAns, err := b.deepseek.AnalyzeQuestion(question)
			if err != nil {
				log.Printf("Error calling Deepseek API asynchronously: %v", err)
				b.editMessage(callback.Message.Chat.ID, initialMessageID,
					fmt.Sprintf("Your answer: \"%s\"\n\nI couldn't determine the correct answer at this time. Please use /help for more information about this question.", userAnswer))
				return
			}

			log.Printf("Received Deepseek analysis for question %d with right answer: %d",
				questionNum, rightAns)

			// Format the updated message
			updatedMessage := fmt.Sprintf("Your answer: \"%s\"\n\n%s\n\nUse /next to practice with a new question",
				userAnswer, resp)

			// Edit the original message with the Deepseek response
			b.editMessage(callback.Message.Chat.ID, initialMessageID, updatedMessage)
			log.Printf("Updated message %d with Deepseek response (length: %d)", initialMessageID, len(resp))

			// Cache the response
			if err := b.db.CacheDeepseekResponse(questionNum, resp, rightAns); err != nil {
				log.Printf("Error caching Deepseek response: %v", err)
			} else {
				log.Printf("Cached Deepseek response for question %d", questionNum)
			}

			rightAnswer = rightAns
			cachedResponse = resp
		}

		// Now determine if the answer was correct based on Deepseek's analysis
		if rightAnswer != -1 {
			isCorrect = (answerNum == rightAnswer)

			// Update the activity record with the correct status
			// This might require adding an update method to the database
			log.Printf("Async result: User's answer for question %d was %v",
				questionNum, isCorrect)

			// Prepare correctness indicator
			var correctnessText string
			if isCorrect {
				correctnessText = "✅ Based on my analysis, your answer was correct!"
			} else {
				correctAnswerText := "Unknown"
				if rightAnswer >= 0 && rightAnswer < len(question.Answers) {
					correctAnswerText = question.Answers[rightAnswer]
				}
				correctnessText = fmt.Sprintf("❌ Based on my analysis, the correct answer is: %s", correctAnswerText)
			}

			// If we already edited the message with the full response, there's no need to do it again
			// But if we got a cached response we might need to add the correctness info
			if cachedResponse != "" && len(cachedResponse) > 0 {
				updatedMessage := fmt.Sprintf("Your answer: \"%s\"\n\n%s\n\n%s\n\nUse /next to practice with a new question",
					userAnswer, correctnessText, cachedResponse)
				b.editMessage(callback.Message.Chat.ID, initialMessageID, updatedMessage)
				log.Printf("Updated message %d with cached response and correctness info", initialMessageID)
			}
		}
	}()
}

// sendRandomQuestion sends a random question to the user
func (b *Bot) sendRandomQuestion(chatID int64) {
	if len(b.questions) == 0 {
		b.sendMessage(chatID, "No questions available. Please try again later.")
		return
	}

	// Select a random question
	rand.Seed(time.Now().UnixNano())
	randomIndex := rand.Intn(len(b.questions))
	question := b.questions[randomIndex]

	// Store the user's current question
	userID := chatID // In private chats, the Chat ID equals the User ID
	b.userQuestions[userID] = question.Number

	// Prepare message text
	var messageText string
	if question.Image != "" {
		// If the question has an image, just send the number
		messageText = fmt.Sprintf("Question #%d:", question.Number)
	} else {
		// Otherwise, include the question text
		messageText = fmt.Sprintf("Question #%d: %s", question.Number, question.Question)
	}

	// Check if the question has an image
	if question.Image != "" {
		// Send the image first
		imagePath := filepath.Join("assets", question.Image)
		b.sendImage(chatID, imagePath, messageText)
	} else {
		// Send text message only
		b.sendMessage(chatID, messageText)
	}

	// Prepare answer buttons
	var keyboard [][]tgbotapi.InlineKeyboardButton
	for i, answer := range question.Answers {
		callbackData := fmt.Sprintf("%s%d:%d", callbackPrefix, question.Number, i)
		button := tgbotapi.NewInlineKeyboardButtonData(answer, callbackData)
		row := []tgbotapi.InlineKeyboardButton{button}
		keyboard = append(keyboard, row)
	}

	// If no answers provided, show a default option
	if len(keyboard) == 0 {
		callbackData := fmt.Sprintf("%s%d:%d", callbackPrefix, question.Number, 0)
		button := tgbotapi.NewInlineKeyboardButtonData("Not sure (no options provided)", callbackData)
		row := []tgbotapi.InlineKeyboardButton{button}
		keyboard = append(keyboard, row)
	}

	// Send answers as inline keyboard
	answerText := "Please select your answer:"
	msg := tgbotapi.NewMessage(chatID, answerText)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending answers message: %v", err)
	}
}

// sendMessage sends a text message
func (b *Bot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)

	// Check if the text appears to be markdown by looking for markdown syntax
	if strings.Contains(text, "```") ||
		strings.Contains(text, "**") ||
		strings.Contains(text, "*") ||
		strings.Contains(text, "##") ||
		strings.Contains(text, "#") ||
		strings.Contains(text, "`") {
		log.Printf("Detected markdown content, setting parse mode to MarkdownV2")
		// Escape special characters required by Telegram's MarkdownV2
		//escapedText := escapeMarkdown(text)

		msg.Text = text
		msg.ParseMode = tgbotapi.ModeMarkdownV2
	} else {
		msg.ParseMode = tgbotapi.ModeHTML
	}

	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)

		// If sending fails with markdown, try without formatting
		if msg.ParseMode == tgbotapi.ModeMarkdownV2 {
			log.Printf("Markdown rendering failed, falling back to plain text")
			plainMsg := tgbotapi.NewMessage(chatID, text)
			if _, err := b.api.Send(plainMsg); err != nil {
				log.Printf("Plain text fallback also failed: %v", err)
			}
		}
	}
}

// sendMarkdownMessage sends a text message with MarkdownV2 formatting
func (b *Bot) sendMarkdownMessage(chatID int64, text string) {
	escapedText := escapeMarkdown(text)
	msg := tgbotapi.NewMessage(chatID, escapedText)
	msg.ParseMode = tgbotapi.ModeMarkdownV2

	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending markdown message: %v", err)
	}
}

// escapeMarkdown escapes special characters for Telegram's MarkdownV2 format
func escapeMarkdown(text string) string {
	// Characters that need escaping in MarkdownV2: _*[]()~`>#+-=|{}.!
	specialChars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}

	// Don't escape characters within code blocks
	parts := strings.Split(text, "```")
	for i := 0; i < len(parts); i++ {
		// Only escape characters in non-code parts (even indices)
		if i%2 == 0 {
			for _, char := range specialChars {
				parts[i] = strings.ReplaceAll(parts[i], char, "\\"+char)
			}
		}
	}

	// Rejoin the parts
	return strings.Join(parts, "```")
}

// sendImage sends an image with caption
func (b *Bot) sendImage(chatID int64, imagePath, caption string) {
	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(imagePath))
	photo.Caption = caption

	if _, err := b.api.Send(photo); err != nil {
		log.Printf("Error sending image %s: %v", imagePath, err)
		// Fall back to text message if image sending fails
		errorMsg := fmt.Sprintf("%s\n\n(Note: Image could not be sent. Path: %s)", caption, imagePath)
		b.sendMessage(chatID, errorMsg)
	}
}

// sendCallbackResponse sends a response to a callback query
func (b *Bot) sendCallbackResponse(callbackID, text string) {
	callback := tgbotapi.NewCallback(callbackID, text)
	if _, err := b.api.Request(callback); err != nil {
		log.Printf("Error sending callback response: %v", err)
	}
}

// editMessage edits an existing message
func (b *Bot) editMessage(chatID int64, messageID int, newText string) {
	edit := tgbotapi.NewEditMessageText(chatID, messageID, newText)

	// Check if the text appears to be markdown
	if strings.Contains(newText, "```") ||
		strings.Contains(newText, "**") ||
		strings.Contains(newText, "*") ||
		strings.Contains(newText, "##") ||
		strings.Contains(newText, "#") ||
		strings.Contains(newText, "`") {
		// Escape special characters required by Telegram's MarkdownV2
		escapedText := escapeMarkdown(newText)
		edit.Text = escapedText
		edit.ParseMode = tgbotapi.ModeMarkdownV2
	} else {
		edit.ParseMode = tgbotapi.ModeHTML
	}

	if _, err := b.api.Send(edit); err != nil {
		log.Printf("Error editing message: %v", err)

		// If editing fails with markdown, try without formatting
		if edit.ParseMode == tgbotapi.ModeMarkdownV2 {
			log.Printf("Markdown editing failed, falling back to plain text")
			plainEdit := tgbotapi.NewEditMessageText(chatID, messageID, newText)
			if _, err := b.api.Send(plainEdit); err != nil {
				log.Printf("Plain text edit fallback also failed: %v", err)
			}
		}
	}
}
