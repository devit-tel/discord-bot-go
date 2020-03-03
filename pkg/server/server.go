package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/devit-tel/discord-bot-go/pkg/aes256"
	"github.com/gin-gonic/gin"
)

// VerifyBody Verify Body
type VerifyBody struct {
	UserID      string `json:"user_id"`
	ProfileName string `json:"profile_name"`
	RoleID      string `json:"role_id"`
	MessageID   string `json:"message_id"`
	IssuedAt    string `json:"issued_at"`
}

// DiscordConfig DiscordConfig
type DiscordConfig struct {
	Session          *discordgo.Session
	DiscordServerID  string
	DiscordChannelID string
}

// SetupServer setup server XD
func SetupServer(key []byte, dc DiscordConfig) *gin.Engine {
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

		err = dc.Session.GuildMemberNickname(dc.DiscordServerID, verifyBody.UserID, verifyBody.ProfileName)
		if err != nil {
			c.String(500, "Cannot modify user's role")
			c.Error(err)
			c.Abort()
			return
		}

		err = dc.Session.GuildMemberRoleAdd(dc.DiscordServerID, verifyBody.UserID, verifyBody.RoleID)
		if err != nil {
			c.String(500, "Cannot modify user's role")
			c.Error(err)
			c.Abort()
			return
		}

		err = dc.Session.MessageReactionAdd(dc.DiscordChannelID, verifyBody.MessageID, "🎉")
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
