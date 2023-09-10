package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nsf/termbox-go"
)
type Quote struct {
	Quote  string `json:"q"`
	Author string `json:"a"`
}

var startTime time.Time
var typedCharacters int
var typingSpeed float64

type Game struct {
	db            *sql.DB
	currentQuote  Quote
	userInput     string
	score         int
	typedChars    int
	startedTyping bool
	typingSpeed   float64
    startTime     time.Time
}

func main() {
    // Create a new game instance
    game, err := NewGame()
    if err != nil {
        // Handle the error if initialization fails
        panic(err)
    }

    // Start the game loop
    game.Start()
}

func NewGame() (*Game, error) {
	err := termbox.Init()
	if err != nil {
		return nil, err
	}
	rand.Seed(time.Now().UnixNano())

	db, err := openDatabase()
	if err != nil {
		return nil, err
	}

	return &Game{
		db: db,
	}, nil
}

func (g *Game) Start() {
	defer termbox.Close()
	g.runGameLoop()
}

func (g *Game) runGameLoop() {
	// Initialize the game state
	g.initGame()

	for {
		termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
		g.drawSentenceWithAuthor()
		g.drawInput()
		g.drawScore()
		g.drawTypingSpeed()
		termbox.Flush()

		ev := termbox.PollEvent()
		if ev.Type == termbox.EventKey {
			if ev.Key == termbox.KeyEsc {
				break
			} else if ev.Key == termbox.KeyBackspace || ev.Key == termbox.KeyBackspace2 {
				g.handleBackspace()
			} else if ev.Ch != 0 || ev.Key == termbox.KeySpace {
				g.handleInputCharacter(ev)
			}
		}
	}
}

func (g *Game) initGame() {
	quote, err := getRandomQuote(g.db)
	if err != nil {
		log.Fatal(err)
	}

	g.currentQuote = quote
	g.userInput = ""
	// Remove the following line to preserve the score
	// g.score = 0
	// Remove the following line to retain the typing speed
	// g.typedChars = 0
	// g.startedTyping = false

	// Retain the typing speed and startTime
	if !g.startedTyping {
		g.startTime = time.Now()
	}
}

func (g *Game) handleBackspace() {
	if len(g.userInput) > 0 {
		g.userInput = g.userInput[:len(g.userInput)-1]
	}
}

func (g *Game) handleInputCharacter(ev termbox.Event) {
	if !g.startedTyping {
		g.startedTyping = true
		g.startTime = time.Now()
	}

	g.typedChars++
	if ev.Ch != 0 {
		g.userInput += string(ev.Ch)
	} else if ev.Key == termbox.KeySpace {
		g.userInput += " "
	}

	elapsedTime := time.Since(g.startTime).Seconds()
	g.typingSpeed = float64(g.typedChars) / elapsedTime

	accuracy := calculateAccuracy(g.userInput, g.currentQuote.Quote)
	g.score = int(accuracy * 100)

	if len(g.userInput) >= len(g.currentQuote.Quote) {
		g.initGame()
		addSqlQuote(g.db, g.currentQuote.Quote, g.currentQuote.Author)
	}
}

func (g *Game) drawScore() {
	width, _ := termbox.Size()
	x := width - 10
	y := 1
	scoreStr := fmt.Sprintf("Score: %d", g.score)

	for i, char := range scoreStr {
		termbox.SetCell(x+i, y, char, termbox.ColorDefault, termbox.ColorDefault)
	}
}

func (g *Game) drawInput() {
	width, height := termbox.Size()
	x := (width - len(g.userInput)) / 2
	y := height/2 + 1

	for i, char := range g.userInput {
		termbox.SetCell(x+i, y, char, termbox.ColorDefault, termbox.ColorDefault)
	}
}

func (g *Game) drawSentenceWithAuthor() {
	width, height := termbox.Size()
	maxLineWidth := int(float64(width) * 0.8)
	lines := []string{}

	for len(g.currentQuote.Quote) > maxLineWidth {
		lines = append(lines, g.currentQuote.Quote[:maxLineWidth])
		g.currentQuote.Quote = g.currentQuote.Quote[maxLineWidth:]
	}
	lines = append(lines, g.currentQuote.Quote)

	sentenceHeight := len(lines)
	y := (height - sentenceHeight) / 2
	authorX := (width - len(g.currentQuote.Author)) / 2
	authorY := y - 2

	for i, char := range g.currentQuote.Author {
		termbox.SetCell(authorX+i, authorY, char, termbox.ColorMagenta, termbox.ColorDefault)
	}

	for _, line := range lines {
		x := (width - len(line)) / 2
		for i, char := range line {
			termbox.SetCell(x+i, y, char, termbox.ColorDefault, termbox.ColorDefault)
		}
		y++
	}
}

func (g *Game) drawTypingSpeed() {
	width, height := termbox.Size()
	speedStr := fmt.Sprintf("Speed: %.2f CPS", g.typingSpeed)
	x := width - len(speedStr) - 1
	y := height - 1

	for i, char := range speedStr {
		termbox.SetCell(x+i, y, char, termbox.ColorDefault, termbox.ColorDefault)
	}
}

func calculateAccuracy(input string, actual string) float64 {
	commonLength := min(len(input), len(actual))
	correctChars := 0

	for i := 0; i < commonLength; i++ {
		if input[i] == actual[i] {
			correctChars++
		}
	}

	accuracy := float64(correctChars) / float64(len(actual))
	return accuracy
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getRandomQuote(db *sql.DB) (Quote, error) {
	quotes, err := getRandomQuoteFromAPI()
	if err != nil {
		return getRandomQuoteFromDatabase(db)
	}
	return quotes[0], nil
}

func getRandomQuoteFromAPI() ([]Quote, error) {
	client := http.Client{}
	resp, err := client.Get("https://zenquotes.io/api/random")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quote from API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var quotes []Quote
	if err := json.Unmarshal(body, &quotes); err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w", err)
	}

	return quotes, nil
}

func getRandomQuoteFromDatabase(db *sql.DB) (Quote, error) {
	var text, author string
	query := "SELECT text, author FROM quotes ORDER BY RANDOM() LIMIT 1"
	err := db.QueryRow(query).Scan(&text, &author)
	if err != nil {
		return Quote{}, fmt.Errorf("failed to fetch quote from database: %w", err)
	}
	return Quote{Quote: text, Author: author}, nil
}

func openDatabase() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "quotes.db")
	if err != nil {
		return nil, err
	}

	// Check if the database file exists
	_, err = os.Stat("quotes.db")
	if os.IsNotExist(err) {
		// Create the database file and any necessary tables
		_, err = db.Exec("CREATE TABLE quotes (quote TEXT, author TEXT)")
		if err != nil {
			db.Close() // Close the connection if table creation fails
			return nil, err
		}
	}

	return db, nil
}

func addSqlQuote(db *sql.DB, quote, author string) error {
	_, err := db.Exec("INSERT INTO quotes (text, author) VALUES (?, ?)", quote, author)
	return err
}
