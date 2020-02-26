package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mervick/aes-everywhere/go/aes256"

	// "github.com/devit-tel/discord-bot-go/pkg/aes256"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

const (
	contentFormat string = "Nickname, Role, Internal E-mail"
)

var (
	verifyServiceBaseURL string
	emailServiceURL      string
	discordToken         string
	discordChannelID     string
	discordServerID      string
	passphrase           string
	serverAddress        string
)

// EmailPayload Email payload
type EmailPayload struct {
	Sender   string    `json:"sender"`
	Receiver string    `json:"receiver"`
	Subject  string    `json:"subject"`
	Data     EmailBody `json:"data"`
}

// EmailBody Email body
type EmailBody struct {
	VerifyLink  string `json:"verify_link"`
	DiscordName string `json:"discord_name"`
	ProfileName string `json:"profile_name"`
	Role        string `json:"role"`
	IssuedAt    string `json:"issued_at"`
}

// VerifyBody Verify Body
type VerifyBody struct {
	ProfileName string `json:"profile_name"`
	Role        string `json:"role"`
	IssuedAt    string `json:"issued_at"`
}

func main() {
	setupEnv()
	go setupDiscord(discordToken)
	go setupRouter(serverAddress)

	for {
		// Do some thing
	}
}

func setupEnv() {
	if err := godotenv.Load(); err != nil {
		log.Println("Error loading .env file")
	}

	verifyServiceBaseURL = getEnv("VERIFY_SERVICE_BASE_URL", "")
	emailServiceURL = getEnv("EMAIL_SERVICE_URL", "")
	discordToken = getEnv("DISCORD_TOKEN", "")
	discordChannelID = getEnv("DISCORD_CHANNEL_ID", "")
	discordServerID = getEnv("DISCORD_SERVER_ID", "")
	passphrase = getEnv("PASSPHRASE", "P4S$W0Rd")
	serverAddress = getEnv("SERVER_ADDRESS", ":8080")
}

func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return defaultValue
	}
	return value
}

func setupRouter(address string) *gin.Engine {
	gin.DisableConsoleColor()
	r := gin.Default()

	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	r.GET("/verify/:secret", func(c *gin.Context) {
		secret := c.Param("secret")
		encrypted := aes256.Decrypt(secret, passphrase)
		var emailBody EmailBody

		if err := json.Unmarshal([]byte(encrypted), &emailBody); err != nil {
			c.String(500, "Cannot convert JSON")
			c.Error(err)
			c.Abort()
			return
		}

	})

	r.Run(address)

	return r
}

func sendEmail(emailPayload EmailPayload) error {
	jsonValue, err := json.Marshal(emailPayload)
	if err != nil {
		return err
	}

	_, err = http.Post(emailServiceURL, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return err
	}

	return nil
}

func setupDiscord(token string) (*discordgo.Session, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	dg.AddHandler(messageCreate)

	if dg.Open(); err != nil {
		return nil, err
	}

	fmt.Println("Bot is now running.")

	return dg, nil
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.ChannelID != discordChannelID {
		log.Println("Wirdo talking to me, help!!", m.Author.Username, m.Author.Email, m.Content)
		return
	}

	splitedContent := strings.Split(m.Content, ",")

	if l := len(splitedContent); l < 3 {
		_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Message must be in the format \"%s\"", contentFormat))
		if err != nil {
			log.Println(err)
		}
		return
	}

	verifyBytes, err := json.Marshal(VerifyBody{
		ProfileName: splitedContent[0],
		Role:        splitedContent[1],
		IssuedAt:    time.Now().Format("YY-MM-DD HH:mm:ss"),
	})

	secret := aes256.Encrypt(string(verifyBytes), passphrase)

	emailPayload :=
		EmailPayload{
			Receiver: splitedContent[2],
			Sender:   "do-not-reply@mail.service.drivs.io",
			Subject:  "Verify email for discord channel",
			Data: EmailBody{
				DiscordName: m.Author.Username,
				VerifyLink:  fmt.Sprintf("%s/verify/%s", verifyServiceBaseURL, secret),
				ProfileName: splitedContent[0],
				Role:        splitedContent[1],
				IssuedAt:    time.Now().Format("YY-MM-DD HH:mm:ss"),
			},
		}

	if err := sendEmail(emailPayload); err != nil {
		log.Println(err)
		return
	}

	_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Verify link sent to your email (%s)", splitedContent[2]))
	if err != nil {
		log.Println(err)
	}
}
