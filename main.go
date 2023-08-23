package main

import (
  "log"
  "encoding/json"
  "fmt"
  "io"
  "net/http"
  "time"
)

type Quote struct {
	Quote string `json:"q"`
	Author string `json:"a"`
}

func main() {
	quotes, err := getRandomQuote()
	if err != nil {
		log.Fatalf("Error fetching quote: %v", err)
	}

  quote := quotes[0]

	for {
		fmt.Println("Type the following sentence:")
		fmt.Println(quote.Quote)

		startTime := time.Now()

		var userInput string
		fmt.Print("Your input: ")
		fmt.Scanln(&userInput)

		endTime := time.Now()

		typingTime := endTime.Sub(startTime).Seconds()
		typingSpeed := float64(len(quote.Quote)) / typingTime // characters per second
		accuracy := calculateAccuracy(quote.Quote, userInput) // percentage

    fmt.Printf("Typing Speed: %.2f characters per second\n", typingSpeed)
    fmt.Printf("Accuracy: %.2f%%\n", accuracy)
    
		fmt.Print("Do you want to play again? (y/n): ")
		var playAgain string
		fmt.Scanln(&playAgain)
		if playAgain != "y" {
			break
		}
	}
}

func calculateAccuracy(original, typed string) float64 {
	correctCount := 0
	for i := 0; i < len(original) && i < len(typed); i++ {
		if original[i] == typed[i] {
			correctCount++
		}
	}
	return float64(correctCount) / float64(len(original)) * 100
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
