package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"time"
    "log"
    "database/sql"
	"github.com/nsf/termbox-go"
    _ "github.com/mattn/go-sqlite3"
)

type Quote struct {
	Quote  string `json:"q"`
	Author string `json:"a"`
}

var startTime time.Time
var typedCharacters int
var typingSpeed float64

func main() {
	err := termbox.Init()
	if err != nil {
		fmt.Println("Failed to initialize termbox:", err)
		os.Exit(1)
	}
	defer termbox.Close()

	rand.Seed(time.Now().UnixNano())

    gameLoop()
}

func gameLoop() {
	quote, err := getRandomQuote()
	if err != nil {
		fmt.Println("Failed to fetch quote:", err)
		os.Exit(1)
	}

    db, err := openDatabase()
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

	currentSentence := quote[0].Quote
    currentAuthor := quote[0].Author
	userInput := ""
	score := 0
	startedTyping := false // Keep track of whether the user started typing
    
    err = addSqlQuote(db, currentSentence, currentAuthor)
    if err != nil {
        log.Fatal(err)
    }

	for {
		termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
		drawSentenceWithAuthor(currentSentence, currentAuthor)
		drawInput(userInput)
		drawScore(score)
		drawTypingSpeed()

		termbox.Flush() // Flush the changes to the terminal screen

		ev := termbox.PollEvent()
		if ev.Type == termbox.EventKey {
			if ev.Key == termbox.KeyEsc {
				break
			} else if ev.Key == termbox.KeyBackspace || ev.Key == termbox.KeyBackspace2 {
				if len(userInput) > 0 {
					userInput = userInput[:len(userInput)-1] // Remove the last character
				}
			} else if ev.Ch != 0 || ev.Key == termbox.KeySpace {
				if !startedTyping {
					startedTyping = true
					startTime = time.Now() // Start the timer when user starts typing
				}

				typedCharacters++
				if ev.Ch != 0 {
					userInput += string(ev.Ch)
				} else if ev.Key == termbox.KeySpace {
					userInput += " "
				}

				// Calculate typing speed
				elapsedTime := time.Since(startTime).Seconds()
				typingSpeed = float64(typedCharacters) / elapsedTime

				accuracy := calculateAccuracy(userInput, currentSentence)
				score = int(accuracy * 100) // Convert accuracy to percentage

				if len(userInput) >= len(currentSentence) {
					quote, err := getRandomQuote()
					if err != nil {
						fmt.Println("Failed to fetch quote:", err)
						os.Exit(1)
					}

					currentSentence = quote[0].Quote
                    currentAuthor = quote[0].Author
					userInput = ""
					typedCharacters = 0   // Reset typed characters for the new sentence
					startedTyping = false // Reset the typing start flag
                    err = addSqlQuote(db, currentSentence, currentAuthor)
                    if err != nil {
                        log.Fatal(err)
                    }
				}
			}
		}
	}
}

func drawScore(score int) {
	width, _ := termbox.Size() // Get terminal width
	x := width - 10
	y := 1
	scoreStr := fmt.Sprintf("Score: %d", score)

	for i, char := range scoreStr {
		termbox.SetCell(x+i, y, char, termbox.ColorDefault, termbox.ColorDefault)
	}
}

func drawInput(input string) {
	width, height := termbox.Size() // Get terminal size
	x := (width - len(input)) / 2
	y := height/2 + 1
	// Draw the user input using termbox.SetCell
	for i, char := range input {
		termbox.SetCell(x+i, y, char, termbox.ColorDefault, termbox.ColorDefault)
	}
}

func drawSentenceWithAuthor(sentence, author string) {
	width, height := termbox.Size() // Get terminal size

	maxLineWidth := int(float64(width) * 0.8) // 80% of the width
	lines := []string{}

	// Split the sentence into lines that fit within the maxLineWidth
	for len(sentence) > maxLineWidth {
		lines = append(lines, sentence[:maxLineWidth])
		sentence = sentence[maxLineWidth:]
	}
	lines = append(lines, sentence)

	// Calculate starting y-coordinate for vertical centering
	sentenceHeight := len(lines)
	y := (height - sentenceHeight) / 2

	// Draw each line of the author's name above the sentence block
	authorX := (width - len(author)) / 2
	authorY := y - 2 // Adjust the gap between author and sentence

	for i, char := range author {
		// Use a different font or style for the author, if supported
		termbox.SetCell(authorX+i, authorY, char, termbox.ColorMagenta, termbox.ColorDefault)
	}

	// Draw each line of the sentence in a separate area
	for _, line := range lines {
		x := (width - len(line)) / 2
		for i, char := range line {
			termbox.SetCell(x+i, y, char, termbox.ColorDefault, termbox.ColorDefault)
		}
		y++
	}
}

func drawSentence(sentence string) {
	width, height := termbox.Size() // Get terminal size

	maxLineWidth := int(float64(width) * 0.8) // 80% of the width
	lines := []string{}

	// Split the sentence into lines that fit within the maxLineWidth
	for len(sentence) > maxLineWidth {
		lines = append(lines, sentence[:maxLineWidth])
		sentence = sentence[maxLineWidth:]
	}
	lines = append(lines, sentence)

	// Calculate starting y-coordinate for vertical centering
	sentenceHeight := len(lines)
	y := (height - sentenceHeight) / 2

	// Draw each line of the sentence in a separate area
	for _, line := range lines {
		x := (width - len(line)) / 2
		for i, char := range line {
			termbox.SetCell(x+i, y, char, termbox.ColorDefault, termbox.ColorDefault)
		}
		y++
	}
}

func drawTypingSpeed() {
	width, height := termbox.Size()
	speedStr := fmt.Sprintf("Speed: %.2f CPS", typingSpeed)
	x := width - len(speedStr) - 1 // Leave a margin from the right edge
	y := height - 1                // Bottom of the terminal

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

func getRandomQuote() ([]Quote, error) {
	client := http.Client{}
	resp, err := client.Get("https://zenquotes.io/api/random")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quote: %w", err)
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

func openDatabase() (*sql.DB, error) {
    db, err := sql.Open("sqlite3", "mydatabase.db")
    if err != nil {
        return nil, err
    }

    // Check if the database file exists
    _, err = os.Stat("mydatabase.db")
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
    _, err := db.Exec("INSERT INTO quotes (quote, author) VALUES (?, ?)", quote, author)
    return err
}

func getAllQuotes(db *sql.DB) ([]Quote, error) {
    var quotes []Quote

    rows, err := db.Query("SELECT quote, author FROM quotes")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    for rows.Next() {
        var quote, author string
        if err := rows.Scan(&quote, &author); err != nil {
            return nil, err
        }

        quotes = append(quotes, Quote{Quote: quote, Author: author})
    }

    if err := rows.Err(); err != nil {
        return nil, err
    }

    return quotes, nil
}


