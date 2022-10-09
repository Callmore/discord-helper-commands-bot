package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var logger *log.Logger

const (
	DefaultDuration = time.Hour
	MaxDuration     = 24 * time.Hour
)

func init() {
	godotenv.Load()
}

func init() {
	// Initialise logger
	logFile, err := os.Create("log.log")
	if err != nil {
		fmt.Printf("WARNING: Error opening log file: %s, logging to standard error instead.\n", err)
		logFile = os.Stderr
	}
	logger = log.New(logFile, "bot", log.LstdFlags)
}

func main() {
	season, err := discordgo.New("Bot " + os.Getenv("DISCORD_BOT_TOKEN"))
	if err != nil {
		logger.Fatal("Error creating Discord session: ", err)
	}

	season.AddHandler(eventReady)
	season.AddHandler(eventInteractionCreate)

	// season

	if err = season.Open(); err != nil {
		logger.Fatal("Error opening Discord session: ", err)
	}

	// Register slash commands
	registerCommands(season)

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-exit

	fmt.Println("Exiting...")
	season.Close()
}
