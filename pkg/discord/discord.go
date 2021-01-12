package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/devit-tel/discord-bot-go/pkg/aes256"
	"github.com/hbakhtiyor/strsim"
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

	st, _ := dg.GuildInvites(config.DiscordServerID)
	for _, inv := range st {
		fmt.Println(inv.Code, inv.Inviter.Username)
	}

	dg.AddHandler(config.messageCreate)
	dg.AddHandler(config.userJoin)
	dg.AddHandler(botDisconnect)

	if err = dg.Open(); err != nil {
		return nil, err
	}

	go func() {
		fmt.Println("Setting up roles")

		guildRoles, err := dg.GuildRoles(config.DiscordServerID)
		if err != nil {
			log.Println(err)
		}

		for _, r := range guildRoles {
			if r.Managed {
				// skip Bot's roles
			} else if r.Name == "@everyone" {
				_, err = dg.GuildRoleEdit(config.DiscordServerID, r.ID, r.Name, 0xdddddd, false, 0, true)
				if err != nil {
					log.Println(err, r)
				} else {
					log.Printf("Setup role %s with permission %d", r.Name, 1024)
				}
			} else if match, _ := regexp.MatchString("(?i)^\\[squad].+", r.Name); match == true {
				_, err = dg.GuildRoleEdit(config.DiscordServerID, r.ID, r.Name, 0xffc764, false, 1024, true)
				if err != nil {
					log.Println(err, r)
				} else {
					log.Printf("Setup role %s with permission %d", r.Name, 1024)
				}
			} else if match, _ := regexp.MatchString("(?i)^\\[gang].+", r.Name); match == true {
				_, err = dg.GuildRoleEdit(config.DiscordServerID, r.ID, r.Name, 0xff577f, false, 1024, true)
				if err != nil {
					log.Println(err, r)
				} else {
					log.Printf("Setup role %s with permission %d", r.Name, 1024)
				}
			} else {
				_, err = dg.GuildRoleEdit(config.DiscordServerID, r.ID, r.Name, 0x00af91, true, 1177943745, true)
				if err != nil {
					log.Println(err, r)
				} else {
					log.Printf("Setup role %s with permission %d", r.Name, 1177943745)
				}
			}
		}

	}()

	fmt.Println("Bot is now running.")

	return dg, nil
}

func botDisconnect(_ *discordgo.Session, m *discordgo.Disconnect) {
	fmt.Println(m)
	os.Exit(69)
}

func (config *Config) userJoin(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	log.Println("a")
	t, err := config.getTemplateMessage(fmt.Sprintf("Hello %s, welcome to true e-logistics comunity :heart:", m.User.Username), s, m.User)
	if err != nil {
		log.Println(err)
		return
	}

	_, err = s.ChannelMessageSendEmbed(config.DiscordChannelID, t)

	if err != nil {
		log.Println(err)
	}
}

func (config *Config) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID || m.ChannelID != config.DiscordChannelID {
		return
	}

	if m.GuildID != config.DiscordServerID {
		log.Println("Weirdo talking to me, help!!", m.Author.Username, m.Author.Email, m.Content)
		return
	}

	splitContent := strings.Split(m.Content, ",")

	if l := len(splitContent); l < 3 {

		t, err := config.getTemplateMessage(fmt.Sprintf("Hello %s", m.Author.Username), s, m.Author)
		if err != nil {
			log.Println(err)
			return
		}

		_, err = s.ChannelMessageSendEmbed(config.DiscordChannelID, t)
		if err != nil {
			log.Println(err)
		}
		return
	}

	profileName := strings.TrimSpace(splitContent[0])
	roleName := strings.TrimSpace(splitContent[1])
	email := strings.TrimSpace(splitContent[2])

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

	role, _, err := findRoleByName(roleName, guildRoles)
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
		RoleID:      role.ID,
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
				Role:        role.Name,
			},
		}

	if err := sendEmail(emailPayload); err != nil {
		log.Println(err)
		return
	}

	_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("`%s`:`%s`\nVerify link sent to your email (`%s`)\nLink will be expired in 5 minute\n%s", profileName, role.Name, email, m.Author.Mention()))
	if err != nil {
		log.Println(err)
	}
}

func (config *Config) getTemplateMessage(title string, s *discordgo.Session, m *discordgo.User) (*discordgo.MessageEmbed, error) {
	guildRoles, err := s.GuildRoles(config.DiscordServerID)
	rs := "\n"
	if err != nil {
		return nil, err
	}

	for _, r := range guildRoles {
		rs += fmt.Sprintf(" - %s\n", r.Name)
	}

	return &discordgo.MessageEmbed{
		Title:       title,
		Description: fmt.Sprintf("Please introduce yourself by send a message to this channel\n%s", m.Mention()),
		Color:       3071986,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Format",
				Value: "```Nickname, Role Name, email@true-e-logistics.com```",
			},
			{
				Name:  "Role Name",
				Value: fmt.Sprintf("```markdown%s```", rs),
			},
			{
				Name:  "Example",
				Value: "```Tod, Research, tossaporn_tem@true-e-logistics.com```",
			}},
	}, nil
}

func findRoleByName(name string, roles []*discordgo.Role) (*discordgo.Role, float64, error) {
	rn := make([]string, len(roles))
	for i, r := range roles {
		rn[i] = r.Name
	}

	matches, err := strsim.FindBestMatch(name, rn)
	if err != nil {
		return nil, 0, err
	}

	return roles[matches.BestMatchIndex], matches.BestMatch.Score, nil
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
