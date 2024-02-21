package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nsf/termbox-go"
)

type GameStats struct {
	Stats []StatPair
}

type StatPair struct {
	Label string
	Value interface{}
}

type Quote struct {
	Quote  string `json:"q"`
	Author string `json:"a"`
}

type Game struct {
	db            *sql.DB
	currentQuote  Quote
	quoteY        int
	inputY        int
	userInput     string
	accuracy      int
	roundChars    int
	totalChars    int
	startedTyping bool
	wordsPerMin   float64
	typingSpeed   float64
	startTime     time.Time
	score         int
	highScore     int
	roundTime     float64
	totalTime     float64
	totalErrors   int
	collectKeys   bool
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

func (g *Game) drawAll() {
	g.drawSentenceWithAuthor()
	g.drawInput()
	g.drawScore()
	g.drawTypingSpeed()
}

func (g *Game) runGameLoop() {
	// Initialize the game state
	g.initGame()

	for {
		termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
		g.drawAll()
		termbox.Flush()

		// if g.collectKeys {
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
		// } else {
		// 	time.Sleep(2 * time.Second)
		// }
	}
}

func playPing() error {
	// Create a beep stream for a short beep
	streamer := beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		frequency := 440.0 // Adjust the frequency as needed
		for i := range samples {
			// Generate a simple sine wave for the beep
			samples[i][0] = 0.5 * math.Sin(2.0*math.Pi*frequency*float64(i)/44100) // left channel
			samples[i][1] = samples[i][0]                                          // right channel
		}
		return len(samples), true
	})

	// Initialize the speaker
	speaker.Init(44100, 44100/10)

	// Play the beep sound
	speaker.Play(streamer)

	// Wait for the sound to finish playing
	time.Sleep(time.Second)

	speaker.Clear()

	return nil
}

func (g *Game) initGame() {
	quote, err := getRandomQuote(g.db)
	if err != nil {
		log.Fatal(err)
	}
	quote.Quote = strings.Trim(quote.Quote, " ")

	g.currentQuote = quote
	g.userInput = ""
	g.startedTyping = false
}

func (g *Game) handleBackspace() {
	if len(g.userInput) > 0 {
		g.userInput = g.userInput[:len(g.userInput)-1]
		g.calculateAccuracy()
		g.calculateErrors()
		g.calculateWordsPerMinute()
	}
}

func (g *Game) handleControlBackspace() {
	if len(g.userInput) > 0 {
		lastChar := g.userInput[len(g.userInput)-1]
		if lastChar != ' ' {
			g.deleteLastWord()
		} else {
			g.handleBackspace()
		}
	}
}

func (g *Game) deleteLastWord() {
	// Find the index of the last space in the user input
	lastSpaceIndex := len(g.userInput) - 1
	for lastSpaceIndex >= 0 && g.userInput[lastSpaceIndex] != ' ' {
		lastSpaceIndex--
	}

	// Remove the last word from the user input
	g.userInput = g.userInput[:lastSpaceIndex+1]
}

func (g *Game) handleInputCharacter(ev termbox.Event) {
	if !g.startedTyping {
		g.startedTyping = true
		g.startTime = time.Now()
		g.roundChars = 0
		g.roundTime = 0
	}

	g.roundChars++

	if ev.Ch != 0 {
		g.userInput += string(ev.Ch)
	} else if ev.Key == termbox.KeySpace {
		g.userInput += " "
	}

	g.roundTime = time.Since(g.startTime).Seconds()
	g.typingSpeed = float64(len(g.userInput)) / g.roundTime
	g.calculateWordsPerMinute()
	g.calculateAccuracy()
	g.calculateErrors()

	if len(g.userInput) >= len(g.currentQuote.Quote) {
		g.totalTime = g.totalTime + g.roundTime
		g.totalChars = g.totalChars + g.roundChars

		if g.score > g.highScore {
			g.highScore = g.score
		}
		// playPing()

		g.initGame()
		addSqlQuote(g.db, g.currentQuote.Quote, g.currentQuote.Author)
	}
}

func (g *Game) calculateErrors() {
	roundErrors := 0
	for i := range g.userInput {
		if g.currentQuote.Quote[i] != g.userInput[i] {
			roundErrors += 1
		}
	}
	g.totalErrors = roundErrors
}

func (g *Game) calculateWordsPerMinute() {
	g.wordsPerMin = g.typingSpeed * (60 / 5)
}

func (g *Game) calculateScore() {
	g.score = (2 * g.accuracy) * int(g.typingSpeed)
}

func (gs *GameStats) formatTopBarStr() string {
	var statStrings []string

	for _, pair := range gs.Stats {
		statStrings = append(statStrings, fmt.Sprintf("%s: %v", pair.Label, pair.Value))
	}

	return strings.Join(statStrings, " | ")
}

func (g *Game) drawTopBar() {
	width, _ := termbox.Size()

	g.calculateScore()
	g.calculateWordsPerMinute()

	// Create an array of StatPair objects
	stats := []StatPair{
		{"Highscore", g.highScore},
		{"Score", g.score},
		{"Accuracy", g.accuracy},
		{"WPM", int(g.wordsPerMin)},
		{"Time", int(g.roundTime)},
		{"Errors", g.totalErrors},
	}

	gameStats := &GameStats{
		Stats: stats,
	}

	// Set a maximum line length (adjust as needed)
	maxLineLength := 60

	// Generate the topBarStr from the GameStats struct
	topBarStr := gameStats.formatTopBarStr()

	// Split the string into lines
	lines := wrapText(topBarStr, maxLineLength, "|")

	// Calculate starting y position
	y := 1

	// Display each line
	for _, line := range lines {
		x := (width - len(line)) / 2
		for i, char := range line {
			termbox.SetCell(x+i, y, char, termbox.ColorDefault, termbox.ColorDefault)
		}
		y++
	}
}

func (g *Game) drawScore() {
	// Clear the top bar
	width, _ := termbox.Size()
	for i := 0; i < width; i++ {
		termbox.SetCell(i, 1, ' ', termbox.ColorDefault, termbox.ColorDefault)
	}

	g.drawTopBar()
}

func (g *Game) drawInput() {
	width, _ := termbox.Size()
	maxLineWidth := int(float64(width) * 0.8)
	g.inputY = g.quoteY + 1
	delimiter := " "

	// Use wrapText to get wrapped lines for user input
	userInputLines := wrapText(g.userInput, maxLineWidth, delimiter)

	var k int

	// Draw each line of the wrapped user input
	for i, line := range userInputLines {
		// Calculate x based on the length of the line
		x := (width - len(line)) / 2

		for j, char := range line {
			if g.userInput[k] == g.currentQuote.Quote[k] {
				termbox.SetCell(x+j, g.inputY+i, char, termbox.ColorDefault, termbox.ColorDefault)
			} else {
				termbox.SetCell(x+j, g.inputY+i, char, termbox.ColorBlack, termbox.ColorRed)
			}
			k++
		}
	}
}

func wrapText(text string, maxWidth int, delimiter string) []string {
	words := strings.Split(text, delimiter)
	lines := []string{}

	currentLine := ""
	for _, word := range words {
		if len(currentLine)+len(word)+1 <= maxWidth {
			currentLine += word + delimiter
		} else {
			lines = append(lines, strings.TrimSpace(currentLine))
			currentLine = word + delimiter
		}
	}

	lines = append(lines, strings.TrimSpace(currentLine))
	return lines
}

func (g *Game) drawSentenceWithAuthor() {
	width, height := termbox.Size()
	maxLineWidth := int(float64(width) * 0.8)
	quote := g.currentQuote.Quote
	delimiter := " "
	lines := wrapText(quote, maxLineWidth, delimiter)

	sentenceHeight := len(lines)
	g.quoteY = (height - sentenceHeight) / 2
	authorX := (width - len(g.currentQuote.Author)) / 2
	authorY := g.quoteY - 2

	for i, char := range g.currentQuote.Author {
		termbox.SetCell(authorX+i, authorY, char, termbox.ColorMagenta, termbox.ColorDefault)
	}

	for _, line := range lines {
		x := (width - len(line)) / 2
		for i, char := range line {
			termbox.SetCell(x+i, g.quoteY, char, termbox.ColorDefault, termbox.ColorDefault)
		}
		g.quoteY++
	}
}

func (g *Game) drawTypingSpeed() {
	// Clear the top bar
	width, _ := termbox.Size()
	for i := 0; i < width; i++ {
		termbox.SetCell(i, 1, ' ', termbox.ColorDefault, termbox.ColorDefault)
	}

	g.drawTopBar()
}

func (g *Game) calculateAccuracy() {
	commonLength := min(len(g.userInput), len(g.currentQuote.Quote))
	correctChars := 0

	for i := 0; i < commonLength; i++ {
		if g.currentQuote.Quote[i] == g.userInput[i] {
			correctChars++
		}
	}

	accuracy := float64(correctChars) / float64(commonLength)
	g.accuracy = int(accuracy * 100)
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
