package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/segmentio/ksuid"
)

func handlePollCmd(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.ApplicationCommandData().Options[0].Name {
	case "create":
		createPollCmd(s, i)
	case "end":
		endPollCmd(s, i)
	}
}

// createPollCmd is the handler for the create subcommand of the poll command
func createPollCmd(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	// Check if the user already has a poll running in this guild.
	if databasePollCheckUser(i.Member.User.ID, i.GuildID) {
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: ptr("You already have a poll running in this server!"),
		})
		return
	}

	id := ksuid.New().String()
	options := i.ApplicationCommandData().Options[0].Options

	question := options[0].StringValue()
	duration := DefaultDuration

	// Parse the options
	choices := make([]discordgo.MessageComponent, 0, len(options))
	choicesString := make([]string, 0, len(options))
	for n, option := range options[1:] {
		if strings.HasPrefix(option.Name, "option") {
			// Make sure the option doesn't exceed 80 characters
			if len(option.StringValue()) > 80 {
				s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Content: ptr(fmt.Sprintf("Failed to create poll: option %d exceeds 80 characters", n+1)),
				})
				return
			}

			choices = append(choices, discordgo.Button{
				Label:    option.StringValue(),
				CustomID: fmt.Sprintf("poll|%s|%d", id, n),
				Style:    discordgo.SuccessButton,
			})

			choicesString = append(choicesString, option.StringValue())
		} else if option.Name == "duration" {
			var err error
			duration, err = parseDuration(option.StringValue())
			if err != nil {
				s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Content: ptr("Failed to create poll: invalid duration"),
				})
				return
			}
			if duration > MaxDuration {
				s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
					Content: ptr(fmt.Sprintf("Failed to create poll: duration cannot exceed %.f hours", MaxDuration.Hours())),
				})
				return
			}
		}
	}

	creationTime, err := discordgo.SnowflakeTimestamp(i.ID)
	if err != nil {
		logger.Print("Failed to get creation time: ", err)
		return
	}

	// Create a dummy message to edit later
	msg, err := s.ChannelMessageSend(i.ChannelID, "Creating poll...")
	if err != nil {
		logger.Print("Failed to send message: ", err)
		return
	}

	// Add the poll to the database
	err = databasePollCreate(dbPoll{
		ID:          id,
		Guild:       i.GuildID,
		Channel:     i.ChannelID,
		Message:     msg.ID,
		Question:    question,
		Options:     choicesString,
		Votes:       nil,
		Creator:     i.Member.User.ID,
		CreatedTime: creationTime,
		EndTime:     creationTime.Add(duration),
	})

	if err != nil {
		logger.Print("Failed to create poll: ", err)
		return
	}

	poll, err := databasePollGet(id)
	if err != nil {
		logger.Print("Failed to get poll: ", err)
		return
	}

	_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:      msg.ID,
		Channel: i.ChannelID,
		Content: ptr(""),
		Embeds: []*discordgo.MessageEmbed{
			ptr(generatePollEmbed(poll, i.Member.User)),
		},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: choices,
			},
		},
	})
	if err != nil {
		logger.Print("Failed to send message: ", err)
		return
	}

	// Update the interaction response to say that the poll was created
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: ptr(fmt.Sprintf("Poll created! It will end at %s.", Timestamp(poll.EndTime, TimestampShortDateTime))),
	})

	// Start a goroutine to end the poll
	time.AfterFunc(time.Until(poll.EndTime), func() {
		endPoll(s, id)
	})
}

// endPollCmd is the handler for the end subcommand of the poll command
func endPollCmd(s *discordgo.Session, i *discordgo.InteractionCreate) {
}

// endPoll removes the poll from the database and edits the message to show the results
func endPoll(s *discordgo.Session, pollId string) {
	poll, err := databasePollEnd(pollId)
	if err != nil {
		logger.Print("Failed to end poll: ", err)
		return
	}

	// Get the total number of votes
	totalVotes := 0
	highestVotes := 0
	for _, votes := range poll.Votes {
		totalVotes += votes.Len()

		if votes.Len() > highestVotes {
			highestVotes = votes.Len()
		}
	}

	// Create the message
	user, err := s.User(poll.Creator)
	if err != nil {
		logger.Print("Failed to get user: ", err)
		return
	}

	embed := generatePollEmbed(poll, user)

	embed.Description = fmt.Sprintf("Poll ended (%d vote%s)", totalVotes, plural(totalVotes))
	embed.Color = DiscordRed

	// Update the message
	_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:      poll.Message,
		Channel: poll.Channel,
		Embeds: []*discordgo.MessageEmbed{
			&embed,
		},
		Components: []discordgo.MessageComponent{},
	})
	if err != nil {
		logger.Print("Failed to edit message: ", err)
		return
	}

	// Send the results to the creator
	channel, err := s.UserChannelCreate(poll.Creator)
	if err != nil {
		logger.Print("Failed to create DM channel: ", err)
		return
	}

	// Sort the options
	entries := make([]struct {
		string
		int
	}, 0, len(poll.Options))

	for n, option := range poll.Options {
		entries = append(entries, struct {
			string
			int
		}{
			string: option,
			int:    poll.Votes[n].Len(),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].int > entries[j].int
	})

	// Generate the results fields
	fields := make([]*discordgo.MessageEmbedField, 0, len(poll.Options))
	for _, entry := range entries {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  entry.string,
			Value: formatVoteString(entry.int, totalVotes),
		})
	}

	embed.Fields = fields
	embed.Footer = nil
	embed.Color = DiscordBlurple

	guild, err := s.Guild(poll.Guild)
	if err != nil {
		logger.Print("Failed to get guild name: ", err)
		guild = nil
	}

	content := "The results for your poll are available below."
	if guild != nil {
		content = fmt.Sprintf("The results for your poll in %s are available below.", guild.Name)
	}

	_, err = s.ChannelMessageSendComplex(channel.ID, &discordgo.MessageSend{
		Content: content,
		Embed:   &embed,
	})
	if err != nil {
		logger.Print("Failed to send message: ", err)
		return
	}
}

// Helpers
func generatePollEmbed(poll dbPoll, creator *discordgo.User) discordgo.MessageEmbed {
	fields := []*discordgo.MessageEmbedField{}

	totalVotes := 0
	highestVotes := 0
	for _, voteOption := range poll.Votes {
		totalVotes += voteOption.Len()
		if voteOption.Len() > highestVotes {
			highestVotes = voteOption.Len()
		}
	}

	for i, option := range poll.Options {
		str := option
		if highestVotes > 0 && highestVotes == poll.Votes[i].Len() {
			str = fmt.Sprintf(":medal: %s", option)
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   str,
			Value:  formatVoteString(poll.Votes[i].Len(), totalVotes),
			Inline: true,
		})
	}

	footer := discordgo.MessageEmbedFooter{}
	if creator != nil {
		footer.Text = fmt.Sprintf("Poll created by %s", creator.Username)
		footer.IconURL = creator.AvatarURL("")
	}

	return discordgo.MessageEmbed{
		Title:       poll.Question,
		Description: "Poll ends " + Timestamp(poll.EndTime, TimestampRelative),
		Color:       DiscordYellow,
		Footer:      &footer,
		Timestamp:   poll.CreatedTime.Format(time.RFC3339),
		Fields:      fields,
	}
}

func formatVoteBar(votes, totalVotes int) string {
	if totalVotes == 0 {
		return strings.Repeat("░", 10)
	}

	fill := int(float64(votes) / float64(totalVotes) * 10)

	return fmt.Sprintf(
		"%s%s",
		strings.Repeat("▓", fill),
		strings.Repeat("░", 10-fill))
}

func formatVotePercentage(votes, totalVotes int) string {
	if totalVotes == 0 {
		return ""
	}
	return fmt.Sprintf(" (%.2f%%)", float64(votes)/float64(totalVotes)*100)
}

func formatVoteString(votes, totalVotes int) string {
	return fmt.Sprintf("%s %d vote%s%s", formatVoteBar(votes, totalVotes), votes, plural(votes), formatVotePercentage(votes, totalVotes))
}
