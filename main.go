package main

import (
	"log"
	"os"

	"github.com/devit-tel/discord-bot-go/pkg/discord"
	"github.com/devit-tel/discord-bot-go/pkg/server"
	"github.com/joho/godotenv"
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

func main() {
	setupEnv()

	session, err := discord.SetupDiscord(discord.Config{
		Key:                  key,
		DiscordServerID:      discordServerID,
		DiscordChannelID:     discordChannelID,
		AllowedEmailRegexp:   emailRegexp,
		EmailServiceURL:      emailServiceURL,
		VerifyServiceBaseURL: verifyServiceBaseURL,
	}, discordToken)

	if err != nil {
		log.Panicln(err)
	}

	r := server.SetupServer(key, server.DiscordConfig{
		Session:          session,
		DiscordChannelID: discordChannelID,
		DiscordServerID:  discordServerID,
	})

	err = r.Run(serverAddress)
	if err != nil {
		log.Panicln(err)
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
