package main

import (
	"encoding/base64"
	"github.com/charmbracelet/log"
	"golang.org/x/net/html"
	"strings"

	"github.com/sashabaranov/go-openai"
	"google.golang.org/api/gmail/v1"
)

var (
	dailyTemplate   string
	weeklyTemplate  string
	summaryTemplate string
	emailTemplate   string
	userContext     string
	openAIClient    *openai.Client
)

func dailySummary(messages []*gmail.Message) (string, error) {
	scratchpad := "# Daily Summary:\n\n"

	for _, message := range messages {
		from := extractHeader(message, "From")
		to := extractHeader(message, "To")
		subject := extractHeader(message, "Subject")
		date := extractHeader(message, "Date")
		body := extractBody(message)

		systemPrompt := formatTemplate(dailyTemplate, scratchpad)
		userPrompt := formatEmailTemplate(emailTemplate, from, to, subject, date, body)
		updatedScratchpad, err := callOpenAI([]openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		})
		if err != nil {
			return "", err
		}
		scratchpad = updatedScratchpad
	}

	log.Debug("Email data collection complete:", "scratchpad", scratchpad)

	return convertScratchpadToHTML(scratchpad)
}

func weeklySummary(messages []*gmail.Message) (string, error) {
	scratchpad := "# Weekly Summary\n\n"

	for _, message := range messages {
		from := extractHeader(message, "From")
		to := extractHeader(message, "To")
		subject := extractHeader(message, "Subject")
		date := extractHeader(message, "Date")
		body := extractBody(message)

		systemPrompt := formatTemplate(weeklyTemplate, scratchpad)
		userPrompt := formatEmailTemplate(emailTemplate, from, to, subject, date, body)
		updatedScratchpad, err := callOpenAI([]openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		})
		if err != nil {
			return "", err
		}
		scratchpad = updatedScratchpad
	}

	log.Debug("Email data collection complete:", "scratchpad", scratchpad)

	return convertScratchpadToHTML(scratchpad)
}

func convertScratchpadToHTML(scratchpad string) (string, error) {
	prompt := strings.ReplaceAll(summaryTemplate, "{{scratchpad}}", scratchpad)
	prompt = strings.ReplaceAll(prompt, "{{context}}", userContext)
	return callOpenAI([]openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: prompt,
		},
	})
}

func extractHeader(message *gmail.Message, headerName string) string {
	for _, header := range message.Payload.Headers {
		if header.Name == headerName {
			return header.Value
		}
	}
	return ""
}

func extractBody(message *gmail.Message) string {
	var body string

	// Attempt to extract text from all parts of the email
	for _, part := range message.Payload.Parts {
		// Handle text/plain parts
		if part.MimeType == "text/plain" && part.Body.Data != "" {
			bodyBytes, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err != nil {
				log.Error("Error decoding text/plain part", "error", err)
				continue
			}
			body += string(bodyBytes) + "\n"
		}

		// Handle text/html parts
		if part.MimeType == "text/html" && part.Body.Data != "" {
			bodyBytes, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err != nil {
				log.Error("Error decoding text/html part", "error", err)
				continue
			}

			// Convert HTML to plain text
			htmlText := string(bodyBytes)
			body += htmlToText(htmlText) + "\n"
		}
	}

	if body == "" && message.Payload.Body.Data != "" {
		// Fallback to directly reading the body if it's present (e.g., for simple emails)
		bodyBytes, err := base64.URLEncoding.DecodeString(message.Payload.Body.Data)
		if err != nil {
			log.Error("Error decoding body", "error", err)
			return ""
		}
		body = string(bodyBytes)
	}

	log.Debug("Extracted email body", "body", body)
	return body
}

// htmlToText strips HTML tags and returns the plain text
func htmlToText(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		log.Error("Error parsing HTML", "error", err)
		return ""
	}
	return renderNode(doc)
}

func renderNode(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	if n.Type != html.ElementNode {
		return ""
	}

	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(renderNode(c))
	}

	// Preserve some basic block elements like paragraphs with line breaks
	if n.Data == "p" || n.Data == "br" {
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatTemplate(template, scratchpad string) string {
	prompt := strings.ReplaceAll(template, "{{scratchpad}}", scratchpad)
	prompt = strings.ReplaceAll(prompt, "{{context}}", userContext)
	return prompt
}

func formatEmailTemplate(template, from, to, subject, date, body string) string {
	prompt := strings.ReplaceAll(template, "{{from}}", from)
	prompt = strings.ReplaceAll(prompt, "{{to}}", to)
	prompt = strings.ReplaceAll(prompt, "{{subject}}", subject)
	prompt = strings.ReplaceAll(prompt, "{{date}}", date)
	prompt = strings.ReplaceAll(prompt, "{{body}}", body)
	return prompt
}
