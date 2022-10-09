package main

import (
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func eventReady(s *discordgo.Session, m *discordgo.Ready) {
	// Handle currently running polls
	startupPolls(s)

	logger.Printf("Logged in as %v#%v\n", m.User.Username, m.User.Discriminator)
}

func startupPolls(s *discordgo.Session) {
	for poll := range databasePollGetAll() {
		if time.Now().After(poll.EndTime) {
			endPoll(s, poll.ID)
		}

		// Queue the poll to be ended
		time.AfterFunc(time.Until(poll.EndTime), func() {
			endPoll(s, poll.ID)
		})
	}
}

func eventInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		commandName := i.ApplicationCommandData().Name
		if command, ok := registeredCommands[commandName]; ok {
			command.Handler(s, i)
		} else {
			logger.Print("Got unknown command: ", commandName)
		}
	case discordgo.InteractionMessageComponent:
		data := i.MessageComponentData()
		if data.ComponentType == discordgo.ButtonComponent {
			handleButton(s, i)
			logger.Print("Message component interaction from ", i.MessageComponentData().CustomID)
		}
	}
}

func handleButton(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	buttonArgs := strings.Split(data.CustomID, "|")

	if tryHandlePollButton(s, i, buttonArgs) {
		return
	} else {
		logger.Print("Got unknown button interaction: ", data.CustomID)
	}
}

func tryHandlePollButton(s *discordgo.Session, i *discordgo.InteractionCreate, buttonArgs []string) bool {
	if len(buttonArgs) != 3 {
		return false
	}

	if buttonArgs[0] != "poll" {
		return false
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	// data := i.MessageComponentData()

	choice, err := strconv.ParseInt(buttonArgs[2], 10, 64)
	if err != nil {
		logger.Print("Got button interaction with invalid choice: ", buttonArgs[2])
		return true
	}

	err = databasePollVote(buttonArgs[1], i.Member.User.ID, int(choice))
	if err != nil {
		logger.Print("Failed to write vote to database: ", err)
	}

	// Get the poll to build the embed from
	poll, err := databasePollGet(buttonArgs[1])
	if err != nil {
		logger.Print("Failed to get poll from database: ", err)
		return true
	}

	user, err := s.User(poll.Creator)
	if err != nil {
		logger.Print("Failed to get user: ", err)
		user = nil
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{
			ptr(generatePollEmbed(poll, user)),
		},
	})

	return true
}
