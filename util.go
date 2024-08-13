package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/sashabaranov/go-openai"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type Config struct {
	DailySummaryTime       string `json:"daily_summary_time"`
	WeeklySummaryDay       string `json:"weekly_summary_day"`
	WeeklySummaryTime      string `json:"weekly_summary_time"`
	OpenAIKey              string `json:"open_ai_key"`
	DiscordToken           string `json:"discord_token"`
	DailySummaryChannelID  string `json:"daily_summary_channel_id"`
	WeeklySummaryChannelID string `json:"weekly_summary_channel_id"`
	OAuthDebugChannelID    string `json:"oauth_debug_channel_id"`
}

func parseWeekday(day string) time.Weekday {
	weekdays := map[string]time.Weekday{
		"Sunday":    time.Sunday,
		"Monday":    time.Monday,
		"Tuesday":   time.Tuesday,
		"Wednesday": time.Wednesday,
		"Thursday":  time.Thursday,
		"Friday":    time.Friday,
		"Saturday":  time.Saturday,
	}
	if weekday, ok := weekdays[day]; ok {
		return weekday
	}
	log.Error("Invalid weekday", "day", day)
	return time.Sunday
}

func loadConfig() (*Config, error) {
	log.Info("Loading configuration", "file", configFile)
	f, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("unable to open config file: %v", err)
	}
	defer closeFile(f, "config file")

	config := &Config{}
	if err := json.NewDecoder(f).Decode(config); err != nil {
		return nil, fmt.Errorf("unable to parse config file: %v", err)
	}

	log.Info("Configuration loaded successfully")
	return config, nil
}

func getLastFetchTime() time.Time {
	log.Info("Retrieving last fetch time", "file", lastFetchFile)
	f, err := os.Open(lastFetchFile)
	if err != nil {
		log.Warn("Last fetch file not found, defaulting to 1 day ago")
		return time.Now().AddDate(0, 0, -1)
	}
	defer closeFile(f, "last fetch file")

	var lastFetchTime time.Time
	if err := json.NewDecoder(f).Decode(&lastFetchTime); err != nil {
		log.Fatal("Unable to parse last fetch time", "error", err)
	}

	log.Info("Last fetch time retrieved", "time", lastFetchTime)
	return lastFetchTime
}

func updateLastFetchTime(fetchTime time.Time) {
	log.Info("Updating last fetch time", "time", fetchTime)
	f, err := os.Create(lastFetchFile)
	if err != nil {
		log.Fatal("Unable to save last fetch time", "error", err)
	}
	defer closeFile(f, "last fetch file")

	if err := json.NewEncoder(f).Encode(fetchTime); err != nil {
		log.Error("Failed to encode last fetch time", "error", err)
	} else {
		log.Info("Last fetch time updated successfully")
	}
}

func getClient(config *oauth2.Config) *http.Client {
	tok, err := tokenFromFile(tokenFile)
	if err != nil || !tok.Valid() {
		log.Warn("Token not found or invalid, obtaining a new one")
		tok = getTokenFromWeb(config)
		saveToken(tokenFile, tok)
	} else {
		log.Info("Using existing valid token")
	}
	return config.Client(context.Background(), tok)
}

func getTokenFromWeb(oauthConfig *oauth2.Config) *oauth2.Token {
	authURL := oauthConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	// Send the auth URL to the debug channel on Discord
	err := sendToDiscord(config.OAuthDebugChannelID, fmt.Sprintf("OAuth token has expired. Please authorize this app by visiting the following URL and provide the authorization code here: %s", authURL))
	if err != nil {
		log.Fatal("Unable to send OAuth request to Discord", "error", err)
	}

	log.Info("Waiting for user to provide authorization code in Discord...")

	// Set up a channel to receive the authorization code from Discord
	authCodeChan := make(chan string)

	// Inside your message handler
	discordSession.AddHandlerOnce(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		// Check if the message starts with a mention of the bot
		if strings.HasPrefix(m.Content, "<@"+s.State.User.ID+">") {
			// Remove the mention part
			messageContent := strings.TrimSpace(strings.Replace(m.Content, "<@"+s.State.User.ID+">", "", 1))

			log.Info("Message received", "original content", m.Content, "stripped content", messageContent)

			// Process the stripped message content
			if m.ChannelID == config.OAuthDebugChannelID && m.Author != nil && !m.Author.Bot {
				authCodeChan <- messageContent
			}
		}
	})

	// Wait for the authorization code
	authCode := <-authCodeChan

	// Exchange the authorization code for a token
	tok, err := oauthConfig.Exchange(context.Background(), authCode)
	if err != nil {
		log.Fatal("Unable to retrieve token from web", "error", err)
	}

	// Notify the user of success
	err = sendToDiscord(config.OAuthDebugChannelID, "OAuth token successfully retrieved and saved.")
	if err != nil {
		log.Fatal("Unable to send OAuth success message to Discord", "error", err)
	}

	return tok
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	log.Info("Loading token from file", "file", file)
	f, err := os.Open(file)
	if err != nil {
		log.Error("Failed to open token file", "file", file, "error", err)
		return nil, err
	}
	defer closeFile(f, "token file")

	tok := &oauth2.Token{}
	if err := json.NewDecoder(f).Decode(tok); err != nil {
		log.Error("Failed to decode token", "error", err)
		return nil, err
	}
	log.Info("Token loaded successfully")
	return tok, nil
}

func saveToken(path string, token *oauth2.Token) {
	log.Info("Saving OAuth token", "path", path)
	f, err := os.Create(path)
	if err != nil {
		log.Fatal("Unable to save OAuth token", "error", err)
	}
	defer closeFile(f, "token file")

	if err := json.NewEncoder(f).Encode(token); err != nil {
		log.Error("Failed to encode token", "error", err)
	} else {
		log.Info("Token saved successfully")
	}
}

func createOAuthClient() *http.Client {
	log.Info("Creating OAuth client")
	b, err := os.ReadFile(credentialsFile)
	if err != nil {
		log.Fatal("Unable to read client secret file", "error", err)
	}

	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
	if err != nil {
		log.Fatal("Unable to parse client secret file to config", "error", err)
	}

	return getClient(config)
}

func fetchEmails(client *http.Client, after time.Time) ([]*gmail.Message, error) {
	log.Info("Fetching emails", "after", after)
	srv, err := gmail.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Gmail client: %v", err)
	}

	query := fmt.Sprintf("after:%d", after.Unix())
	r, err := srv.Users.Messages.List("me").Q(query).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve messages: %v", err)
	}

	if len(r.Messages) == 0 {
		log.Info("No new messages found")
		return nil, nil
	}

	var messages []*gmail.Message
	for _, m := range r.Messages {
		msg, err := srv.Users.Messages.Get("me", m.Id).Do()
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve message: %v", err)
		}
		messages = append(messages, msg)
		log.Info("Fetched message", "id", msg.Id, "snippet", msg.Snippet)
	}

	log.Info("Total messages fetched", "count", len(messages))
	return messages, nil
}

func loadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("could not read file: %v", err)
	}
	return string(data), nil
}

func loadUserContext() (string, error) {
	return loadFile("user_context.md")
}

func loadTemplate(templateName string) (string, error) {
	return loadFile("templates/" + templateName)
}

func callOpenAI(messages []openai.ChatCompletionMessage) (string, error) {
	resp, err := openAIClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:    openai.GPT4o,
			Messages: messages,
		},
	)
	if err != nil {
		return "", fmt.Errorf("ChatCompletion error: %v", err)
	}
	return resp.Choices[0].Message.Content, nil
}

func closeFile(f *os.File, description string) {
	if err := f.Close(); err != nil {
		log.Error("Failed to close file", "description", description, "error", err)
	}
}

func sendToDiscord(channelID string, message string) error {
	const maxMessageLength = 2000

	// Helper function to send a chunk of the message
	sendChunk := func(chunk string) error {
		_, err := discordSession.ChannelMessageSend(channelID, chunk)
		if err != nil {
			return fmt.Errorf("sending message chunk to Discord: %w", err)
		}
		return nil
	}

	// Split the message by newlines first
	lines := splitByNewlines(message)

	var currentChunk string

	for _, line := range lines {
		// If the line itself is too long, we need to split it further
		if len(line) > maxMessageLength {
			// Split the long line into chunks of maxMessageLength
			for len(line) > 0 {
				if len(line) > maxMessageLength {
					// Take a chunk of the max length
					chunk := line[:maxMessageLength]
					// Send the chunk
					if err := sendChunk(chunk); err != nil {
						return err
					}
					// Reduce the line by the chunk we just sent
					line = line[maxMessageLength:]
				} else {
					// If the remaining line is within the limit, send it and break
					if err := sendChunk(line); err != nil {
						return err
					}
					line = ""
				}
			}
			continue
		}

		// If adding this line would exceed the max length, send the current chunk and start a new one
		if len(currentChunk)+len(line)+1 > maxMessageLength {
			if err := sendChunk(currentChunk); err != nil {
				return err
			}
			currentChunk = line
		} else {
			// Otherwise, add the line to the current chunk
			if currentChunk != "" {
				currentChunk += "\n"
			}
			currentChunk += line
		}
	}

	// Send any remaining chunk
	if currentChunk != "" {
		if err := sendChunk(currentChunk); err != nil {
			return err
		}
	}

	return nil
}

// Helper function to split a string by newlines and return a slice of strings
func splitByNewlines(text string) []string {
	return strings.Split(text, "\n")
}
