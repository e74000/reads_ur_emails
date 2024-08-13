package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
	"github.com/sashabaranov/go-openai"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"scheduler"
)

const (
	tokenFile       = "token.json"
	credentialsFile = "credentials.json"
	configFile      = "config.json"
	lastFetchFile   = "last_fetch.json"
)

var (
	config             *Config
	weeklySummaryQueue []*gmail.Message
)

var discordSession *discordgo.Session

func main() {
	log.SetLevel(log.DebugLevel)

	log.Info("Loading configuration...")
	var err error
	config, err = loadConfig()
	if err != nil {
		log.Fatal("Failed to load configuration", "error", err)
	}

	log.Info("Initializing components...")
	if err := setupAgent(config); err != nil {
		log.Fatal("Failed to initialize application", "error", err)
	}

	s := setupScheduler(config)
	log.Info("Scheduler initialized and running...")
	go s.Run(context.Background())

	log.Info("Initial OAuth client generation")
	_ = createOAuthClient()

	log.Info("Application is running, awaiting tasks...")
	defer func(discordSession *discordgo.Session) {
		err := discordSession.Close()
		if err != nil {
			log.Error("failed to close discord session", "error", err)
		}
	}(discordSession)
	select {}
}

func setupAgent(config *Config) error {
	var err error

	dailyTemplate, err = loadTemplate("daily_summary_prompt.tmpl")
	if err != nil {
		return fmt.Errorf("loading daily summary template: %w", err)
	}

	weeklyTemplate, err = loadTemplate("weekly_summary_prompt.tmpl")
	if err != nil {
		return fmt.Errorf("loading weekly summary template: %w", err)
	}

	summaryTemplate, err = loadTemplate("scratchpad_to_summary_prompt.tmpl")
	if err != nil {
		return fmt.Errorf("loading scratchpad to summary prompt: %w", err)
	}

	emailTemplate, err = loadTemplate("email_prompt.tmpl")
	if err != nil {
		return fmt.Errorf("loading email prompt: %w", err)
	}

	userContext, err = loadUserContext()
	if err != nil {
		return fmt.Errorf("loading user context: %w", err)
	}

	openAIClient = openai.NewClient(config.OpenAIKey)

	// Initialize Discord session
	discordSession, err = discordgo.New("Bot " + config.DiscordToken)
	if err != nil {
		return fmt.Errorf("error creating Discord session: %w", err)
	}

	// Open WebSocket connection to Discord
	err = discordSession.Open()
	if err != nil {
		return fmt.Errorf("error opening Discord connection: %w", err)
	}

	log.Info("Discord session initialized")
	return nil
}

func setupScheduler(config *Config) *scheduler.Scheduler {
	s := scheduler.New().SetLogger(slog.New(log.Default()))

	log.Info("Setting up scheduler...")
	dailyTime, err := time.Parse("15:04", config.DailySummaryTime)
	if err != nil {
		log.Fatal("Invalid daily summary time format", "error", err)
	}

	s.Add(
		createTask("Daily summary", sendDailySummary).
			Daily(time.Date(0, 0, 0, dailyTime.Hour(), dailyTime.Minute(), 0, 0, time.Local)).
			GlobalBlocking(),
	)

	weeklyTime, err := time.Parse("15:04", config.WeeklySummaryTime)
	if err != nil {
		log.Fatal("Invalid weekly summary time format", "error", err)
	}

	weekday := parseWeekday(config.WeeklySummaryDay)
	s.Add(
		createTask("Weekly summary", sendWeeklySummary).
			Weekly(
				map[time.Weekday]bool{weekday: true},
				time.Date(0, 0, 0, weeklyTime.Hour(), weeklyTime.Minute(), 0, 0, time.Local),
			).
			GlobalBlocking(),
	)

	s.Add(
		createTask("OAuth token refresh", refreshOAuthTokens).
			Every(time.Hour).
			GlobalBlocking(),
	)

	log.Info("Scheduler setup complete")
	return s
}

func createTask(name string, fn func() error) *scheduler.Task {
	return scheduler.NewTask(func() error {
		log.Info(name + " task starting...")
		err := fn()
		if err != nil {
			log.Error(name+" task error", "error", err)
		} else {
			log.Info(name + " task completed")
		}
		return err
	})
}

func sendDailySummary() error {
	lastFetchTime := getLastFetchTime()
	oauthClient := createOAuthClient()

	messages, err := fetchEmails(oauthClient, lastFetchTime)
	if err != nil {
		return fmt.Errorf("fetching emails: %w", err)
	}

	if len(messages) == 0 {
		log.Info("No new messages, skipping daily summary")
		return nil
	}

	summary, err := dailySummary(messages)
	if err != nil {
		return fmt.Errorf("generating daily summary: %w", err)
	}

	if err := sendToDiscord(config.DailySummaryChannelID, summary); err != nil {
		return fmt.Errorf("sending daily summary to Discord: %w", err)
	}

	weeklySummaryQueue = append(weeklySummaryQueue, messages...)
	updateLastFetchTime(time.Now())

	return nil
}

func sendWeeklySummary() error {
	if len(weeklySummaryQueue) == 0 {
		log.Info("No new messages, skipping weekly summary")
		return nil
	}

	summary, err := weeklySummary(weeklySummaryQueue)
	if err != nil {
		return fmt.Errorf("generating weekly summary: %w", err)
	}

	if err := sendToDiscord(config.WeeklySummaryChannelID, summary); err != nil {
		return fmt.Errorf("sending weekly summary to Discord: %w", err)
	}

	weeklySummaryQueue = nil
	return nil
}

func refreshOAuthTokens() error {
	log.Info("Refreshing OAuth tokens...")

	b, err := os.ReadFile(credentialsFile)
	if err != nil {
		log.Fatal("Unable to read client secret file", "error", err)
	}

	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
	if err != nil {
		log.Fatal("Unable to parse client secret file to config", "error", err)
	}

	tok, err := tokenFromFile(tokenFile)
	if err != nil {
		log.Fatal("Unable to load token file", "error", err)
	}

	if !tok.Valid() {
		log.Info("Token expired, refreshing...")
		newTok, err := config.TokenSource(context.Background(), tok).Token()
		if err != nil {
			return fmt.Errorf("unable to refresh token: %w", err)
		}
		saveToken(tokenFile, newTok)
		log.Info("Token successfully refreshed and saved")
	} else {
		log.Info("Token is still valid")
	}

	return nil
}
