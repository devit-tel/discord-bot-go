package discord

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/devit-tel/discord-bot-go/pkg/aes256"
)

const (
	contentFormat string = "Nickname, Role, Internal E-mail"
)

// Config Discord Config
type Config struct {
	Key                  []byte
	DiscordServerID      string
	DiscordChannelID     string
	AllowedEmailRegexp   string
	EmailServiceURL      string
	VerifyServiceBaseURL string
}

// EmailPayload Email payload
type EmailPayload struct {
	EmailServiceURL string
	Sender          string    `json:"sender"`
	Receiver        string    `json:"receiver"`
	Subject         string    `json:"subject"`
	Data            EmailBody `json:"data"`
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

// SetupDiscord SetupDiscord
func SetupDiscord(config Config, token string) (*discordgo.Session, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	dg.AddHandler(messageCreate(config))
	dg.AddHandler(userJoin(config))

	if dg.Open(); err != nil {
		return nil, err
	}

	fmt.Println("Bot is now running.")

	return dg, nil
}

func userJoin(config Config) func(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	return func(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
		_, err := s.ChannelMessageSendEmbed(config.DiscordChannelID, &discordgo.MessageEmbed{
			Title:       fmt.Sprintf("Hello %s, welcome to true e-logistics comunity :heart:", m.User.Username),
			Description: fmt.Sprintf("Please introduce yourself by send a message to this channel\n%s", m.User.Mention()),
			Color:       3071986,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:  "Format",
					Value: "```Nickname, Role Name, email@true-e-logistics.com```",
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
}

func messageCreate(config Config) func(s *discordgo.Session, m *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID || m.ChannelID != config.DiscordChannelID {
			return
		}

		if m.GuildID != config.DiscordServerID {
			log.Println("Wirdo talking to me, help!!", m.Author.Username, m.Author.Email, m.Content)
			return
		}

		splitedContent := strings.Split(m.Content, ",")

		if l := len(splitedContent); l < 3 {
			_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("> %s\nMessage must be in the format \"%s\"\n%s", m.Content, contentFormat, m.Author.Mention()))
			if err != nil {
				log.Println(err)
			}
			return
		}

		profileName := strings.TrimSpace(splitedContent[0])
		roleName := strings.TrimSpace(splitedContent[1])
		email := strings.TrimSpace(splitedContent[2])

		matched, err := regexp.Match(config.AllowedEmailRegexp, []byte(email))
		if err != nil {
			log.Println(err)
			_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("> %s\nOop something went wrong\n%s", m.Content, m.Author.Mention()))
			if err != nil {
				log.Println(err)
			}
			return
		}

		if matched != true {
			_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("> %s\nPlease process with internal email\n%s", m.Content, m.Author.Mention()))
			if err != nil {
				log.Println(err)
			}
			return
		}

		guildRoles, err := s.GuildRoles(config.DiscordServerID)
		if err != nil {
			_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("> %s\nOop something went wrong\n%s", m.Content, m.Author.Mention()))
			log.Println(err)
			return
		}

		roleID, err := findRoleIDByName(roleName, guildRoles)
		if err != nil {
			log.Println(err)

			_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("> %s\n%s\n%s", m.Content, err, m.Author.Mention()))
			if err != nil {
				log.Println(err)
			}
			return
		}

		verifyBytes, err := json.Marshal(VerifyBody{
			UserID:      m.Author.ID,
			RoleID:      roleID,
			ProfileName: profileName,
			IssuedAt:    time.Now().Format(time.RFC3339),
			MessageID:   m.Message.ID,
		})

		secret, err := aes256.Encrypt(config.Key, verifyBytes)
		if err != nil {
			log.Println(err)
			return
		}

		emailPayload :=
			EmailPayload{
				EmailServiceURL: config.EmailServiceURL,
				Receiver:        email,
				Sender:          "do-not-reply@mail.service.drivs.io",
				Subject:         "Verify email for discord channel",
				Data: EmailBody{
					DiscordName: m.Author.Username,
					VerifyLink:  fmt.Sprintf("%s/verify/%s", config.VerifyServiceBaseURL, secret),
					ProfileName: profileName,
					Role:        roleName,
				},
			}

		if err := sendEmail(emailPayload); err != nil {
			log.Println(err)
			return
		}

		_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Verify link sent to your email (%s)\nLink will be expired in 5 minute\n%s", email, m.Author.Mention()))
		if err != nil {
			log.Println(err)
		}
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

func sendEmail(emailPayload EmailPayload) error {
	jsonValue, err := json.Marshal(emailPayload)
	if err != nil {
		return err
	}

	_, err = http.Post(emailPayload.EmailServiceURL, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return err
	}

	return nil
}
