package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/devit-tel/discord-bot-go/pkg/aes256"
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
	key                  []byte
	serverAddress        string
	emailRegexp          string
)

var dg *discordgo.Session

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
}

// VerifyBody Verify Body
type VerifyBody struct {
	UserID      string `json:"user_id"`
	ProfileName string `json:"profile_name"`
	RoleID      string `json:"role_id"`
	MessageID   string `json:"message_id"`
	IssuedAt    string `json:"issued_at"`
}

func main() {
	var err error

	setupEnv()
	dg, err = setupDiscord(discordToken)
	if err != nil {
		log.Panicln(err)
	}
	r := setupRouter()

	r.Run(serverAddress)
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
	key = []byte(getEnv("PASSPHRASE", "P4S$W0Rd_Th41_5i2e_32_by7E_long!"))[:32]
	serverAddress = getEnv("SERVER_ADDRESS", ":8080")
	emailRegexp = getEnv("EMAIL_REGEXP", "(?i)^[0-9a-z_\\-]{1,64}@[a-z]{1,64}.[0-9a-z]{1.3}")
}

func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return defaultValue
	}
	return value
}

func setupRouter() *gin.Engine {
	gin.DisableConsoleColor()
	r := gin.Default()

	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	r.GET("/verify/:secret", func(c *gin.Context) {
		secret := c.Param("secret")
		encrypted, err := aes256.Decrypt(key, secret)
		if err != nil {
			c.String(500, "Cannot decrypt")
			c.Error(err)
			c.Abort()
			return
		}

		var verifyBody VerifyBody

		if err := json.Unmarshal([]byte(encrypted), &verifyBody); err != nil {
			c.String(500, "Cannot convert JSON")
			c.Error(err)
			c.Abort()
			return
		}

		issuedAt, err := time.Parse(time.RFC3339, verifyBody.IssuedAt)
		if err != nil {
			c.String(500, "Invalid time format")
			c.Error(err)
			c.Abort()
			return
		}

		if issuedAt.Add(5 * time.Minute).Before(time.Now()) {
			c.String(500, "Token expired")
			c.Error(errors.New("Token expired"))
			c.Abort()
			return
		}

		err = dg.GuildMemberNickname(discordServerID, verifyBody.UserID, verifyBody.ProfileName)
		if err != nil {
			c.String(500, "Cannot modify user's role")
			c.Error(err)
			c.Abort()
			return
		}

		err = dg.GuildMemberRoleAdd(discordServerID, verifyBody.UserID, verifyBody.RoleID)
		if err != nil {
			c.String(500, "Cannot modify user's role")
			c.Error(err)
			c.Abort()
			return
		}

		err = dg.MessageReactionAdd(discordChannelID, verifyBody.MessageID, ":partying_face:")
		if err != nil {
			c.String(500, "Cannot React to message, but you good to go!")
			c.Error(err)
			c.Abort()
			return
		}

		c.String(200, "Welcome, please check your discord!")
	})

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
	dg.AddHandler(userJoin)

	if dg.Open(); err != nil {
		return nil, err
	}

	fmt.Println("Bot is now running.")

	return dg, nil
}

func userJoin(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	_, err := s.ChannelMessageSendEmbed(discordChannelID, &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("Hello `@%s`, welcome to true e-logistics comunity :heart:", m.User.Username),
		Description: "Please introduce yourself by send a message to this channel",
		Color:       3071986,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Format",
				Value: "```Nickname, Role Name email@true-e-logistics.com```",
			},
			{
				Name:  "Role ID",
				Value: "```md\n1. Admin\n2. Designer\n3. Developer\n4. Manager\n5. Mobile\n6. Research\n7. Sale\n8. Support\n9. Tester```",
			},
			{
				Name:  "Example",
				Value: "```Tod, Research, tossaporn_tem@true-e-logistics.com```",
			}},
	})

	if err != nil {
		log.Println(err)
	}
}

func findRoleIDByName(name string, roles []*discordgo.Role) (string, error) {
	for _, r := range roles {
		if strings.ToLower(r.Name) == strings.ToLower(name) {
			return r.ID, nil
		}
	}

	return "", errors.New("Role not found")
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID || m.ChannelID != discordChannelID {
		return
	}

	if m.GuildID != discordServerID {
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

	profileName := strings.TrimSpace(splitedContent[0])
	roleName := strings.TrimSpace(splitedContent[1])
	email := strings.TrimSpace(splitedContent[2])

	matched, err := regexp.Match(emailRegexp, []byte(email))
	if err != nil {
		log.Println(err)
		return
	}

	if matched != true {
		_, err := s.ChannelMessageSend(m.ChannelID, "Please process with internal email")
		if err != nil {
			log.Println(err)
		}
		return
	}

	guildRoles, err := s.GuildRoles(discordServerID)
	if err != nil {
		log.Println(err)
		return
	}

	roleID, err := findRoleIDByName(roleName, guildRoles)
	if err != nil {
		log.Println(err)
		return
	}

	verifyBytes, err := json.Marshal(VerifyBody{
		UserID:      m.Author.ID,
		RoleID:      roleID,
		ProfileName: profileName,
		IssuedAt:    time.Now().Format(time.RFC3339),
		MessageID:   m.Message.ID,
	})

	secret, err := aes256.Encrypt(key, verifyBytes)
	if err != nil {
		log.Println(err)
		return
	}

	emailPayload :=
		EmailPayload{
			Receiver: email,
			Sender:   "do-not-reply@mail.service.drivs.io",
			Subject:  "Verify email for discord channel",
			Data: EmailBody{
				DiscordName: m.Author.Username,
				VerifyLink:  fmt.Sprintf("%s/verify/%s", verifyServiceBaseURL, secret),
				ProfileName: profileName,
				Role:        roleName,
			},
		}

	if err := sendEmail(emailPayload); err != nil {
		log.Println(err)
		return
	}

	_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Verify link sent to your email (%s)", email))
	if err != nil {
		log.Println(err)
	}
}
