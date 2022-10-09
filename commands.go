package main

import (
	"os"

	"github.com/bwmarrin/discordgo"
)

type Command struct {
	ApplicationCommand *discordgo.ApplicationCommand
	Handler            func(*discordgo.Session, *discordgo.InteractionCreate)
}

var commands = []Command{
	{
		ApplicationCommand: &discordgo.ApplicationCommand{
			Name:        "ping",
			Description: "Ping the bot",
		},
		Handler: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Pong!",
				},
			})
		},
	},
	{
		ApplicationCommand: &discordgo.ApplicationCommand{
			Name:        "poll",
			Description: "Poll commands",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "create",
					Description: "Create a poll",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "question",
							Description: "The question to ask",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "option1",
							Description: "Name of an option that users can vote on",
							Required:    true,
							MaxLength:   80,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "option2",
							Description: "Name of an option that users can vote on",
							Required:    true,
							MaxLength:   80,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "option3",
							Description: "Name of an option that users can vote on",
							Required:    false,
							MaxLength:   80,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "option4",
							Description: "Name of an option that users can vote on",
							Required:    false,
							MaxLength:   80,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "option5",
							Description: "Name of an option that users can vote on",
							Required:    false,
							MaxLength:   80,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "duration",
							Description: "How long the poll should last",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "end",
					Description: "End your poll",
				},
			},
		},
		Handler: handlePollCmd,
	},
}

var registeredCommands = make(map[string]*Command)

func registerCommands(s *discordgo.Session) {
	devGuild, dev := os.LookupEnv("DEV_GUILD")

	for _, command := range commands {
		var err error
		if dev {
			_, err = s.ApplicationCommandCreate(s.State.User.ID, devGuild, command.ApplicationCommand)
		} else {
			_, err = s.ApplicationCommandCreate(s.State.User.ID, "", command.ApplicationCommand)
		}
		if err != nil {
			panic(err)
		}

		registeredCommands[command.ApplicationCommand.Name] = &command
	}
}
