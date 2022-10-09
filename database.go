package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"discordhelperbot/set"

	_ "github.com/mattn/go-sqlite3"
)

type dbPoll struct {
	ID          string
	Guild       string
	Channel     string
	Message     string
	Question    string
	Options     []string
	Votes       []set.Set[string]
	Creator     string
	CreatedTime time.Time
	EndTime     time.Time
}

var db *sql.DB

func init() {
	var err error
	db, err = sql.Open("sqlite3", "database.db")
	if err != nil {
		fmt.Println("Error opening database: ", err)
		os.Exit(1)
	}

	// Drop the old table for testing
	// _, err = db.Exec(`DROP TABLE IF EXISTS polls`)
	// if err != nil {
	// 	fmt.Println("Error dropping table: ", err)
	// 	os.Exit(1)
	// }

	// Setup the database
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS polls (
		id TEXT PRIMARY KEY,
		guild TEXT,
		channel TEXT,
		message TEXT,
		question TEXT,
		options BLOB,
		votes BLOB,
		creator TEXT,
		createdtime TIMESTAMP,
		endtime TIMESTAMP
	)`)
	if err != nil {
		fmt.Println("Error creating database: ", err)
		os.Exit(1)
	}
}

func databasePollCreate(poll dbPoll) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Marshal the options into JSON and construct the votes array
	optionsJSON, err := json.Marshal(poll.Options)
	if err != nil {
		return fmt.Errorf("error marshalling options: %w", err)
	}

	votes := make([]set.Set[string], len(poll.Options))
	votesJSON, err := json.Marshal(votes)
	if err != nil {
		return fmt.Errorf("error marshalling votes: %w", err)
	}

	logger.Printf("POLL: %s, %s, %s, %s, %s, %s, %s, %s, %s, %s",
		poll.ID,
		poll.Guild,
		poll.Channel,
		poll.Message,
		poll.Question,
		optionsJSON,
		votesJSON,
		poll.Creator,
		poll.CreatedTime,
		poll.EndTime)

	// Add the poll to the database
	_, err = tx.Exec(`INSERT INTO polls VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		poll.ID,
		poll.Guild,
		poll.Channel,
		poll.Message,
		poll.Question,
		optionsJSON,
		votesJSON,
		poll.Creator,
		poll.CreatedTime,
		poll.EndTime,
	)
	if err != nil {
		return fmt.Errorf("error adding poll to database: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

func databasePollGet(id string) (dbPoll, error) {
	var (
		pollId      string
		guild       string
		channel     string
		message     string
		question    string
		optionsJSON []byte
		votesJSON   []byte
		creator     string
		createdtime time.Time
		endtime     time.Time
	)

	err := db.QueryRow(`SELECT * FROM polls WHERE id = ?`, id).Scan(&pollId, &guild, &channel, &message, &question, &optionsJSON, &votesJSON, &creator, &createdtime, &endtime)
	if err != nil {
		return dbPoll{}, fmt.Errorf("error getting poll: %w", err)
	}

	if pollId != id {
		panic("pollId != id")
	}

	var options []string
	err = json.Unmarshal(optionsJSON, &options)
	if err != nil {
		return dbPoll{}, fmt.Errorf("error unmarshalling options: %w", err)
	}

	var votes []set.Set[string]
	err = json.Unmarshal(votesJSON, &votes)
	if err != nil {
		return dbPoll{}, fmt.Errorf("error unmarshalling votes: %w", err)
	}

	return dbPoll{
		ID:          id,
		Guild:       guild,
		Channel:     channel,
		Message:     message,
		Question:    question,
		Options:     options,
		Votes:       votes,
		Creator:     creator,
		CreatedTime: createdtime,
		EndTime:     endtime,
	}, nil
}

func databasePollVote(pollId, userId string, option int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the poll
	var (
		votesJSON []byte
		endtime   time.Time
	)

	err = tx.QueryRow(`SELECT votes, endtime FROM polls WHERE id = ?`, pollId).Scan(&votesJSON, &endtime)
	if err != nil {
		return fmt.Errorf("error getting poll: %w", err)
	}

	// Unmarshal the votes
	var votes []set.Set[string]
	err = json.Unmarshal(votesJSON, &votes)
	if err != nil {
		return fmt.Errorf("error unmarshalling votes: %w", err)
	}

	// Check if the option is past the length of votes
	if option >= len(votes) {
		return fmt.Errorf("option %d is out of range", option)
	}

	// Check if the poll has ended
	if endtime.Before(time.Now()) {
		return fmt.Errorf("poll has ended")
	}

	// Update the voting status for the user
	for i := 0; i < len(votes); i++ {
		if option == i {
			votes[i].Add(userId)
		} else {
			votes[i].Remove(userId)
		}
	}

	// Marshal the votes
	votesJSON, err = json.Marshal(votes)
	if err != nil {
		return fmt.Errorf("error marshalling votes: %w", err)
	}

	// Update the poll
	_, err = tx.Exec(`UPDATE polls SET votes = ? WHERE id = ?`, votesJSON, pollId)
	if err != nil {
		return fmt.Errorf("error updating poll: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}
	return nil
}

func databasePollEnd(pollId string) (dbPoll, error) {
	tx, err := db.Begin()
	if err != nil {
		return dbPoll{}, err
	}
	defer tx.Rollback()

	// Get the poll
	poll, err := databasePollGet(pollId)
	if err != nil {
		return dbPoll{}, fmt.Errorf("error getting poll: %w", err)
	}

	// Delete the poll
	_, err = tx.Exec(`DELETE FROM polls WHERE id = ?`, pollId)
	if err != nil {
		return dbPoll{}, fmt.Errorf("error deleting poll: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return dbPoll{}, fmt.Errorf("error committing transaction: %w", err)
	}
	return poll, nil
}

// databasePollsGet gets all the polls in the database and returns a channel to range over
func databasePollGetAll() <-chan dbPoll {
	ch := make(chan dbPoll)

	go func() {
		defer close(ch)

		rows, err := db.Query(`SELECT * FROM polls`)
		if err != nil {
			logger.Printf("error getting polls: %v", err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var (
				pollId      string
				guild       string
				channel     string
				message     string
				question    string
				optionsJSON []byte
				votesJSON   []byte
				creator     string
				createdtime time.Time
				endtime     time.Time
			)

			err := rows.Scan(&pollId, &guild, &channel, &message, &question, &optionsJSON, &votesJSON, &creator, &createdtime, &endtime)
			if err != nil {
				logger.Printf("error scanning poll: %v", err)
				continue
			}

			var options []string
			err = json.Unmarshal(optionsJSON, &options)
			if err != nil {
				logger.Printf("error unmarshalling options: %v", err)
				continue
			}

			var votes []set.Set[string]
			err = json.Unmarshal(votesJSON, &votes)
			if err != nil {
				logger.Printf("error unmarshalling votes: %v", err)
				continue
			}

			ch <- dbPoll{
				ID:          pollId,
				Guild:       guild,
				Channel:     channel,
				Message:     message,
				Question:    question,
				Options:     options,
				Votes:       votes,
				Creator:     creator,
				CreatedTime: createdtime,
				EndTime:     endtime,
			}
		}
	}()

	return ch
}

func databasePollGetUser(userId, guildId string) (dbPoll, error) {
	// Find a poll ID
	var pollId string
	err := db.QueryRow(`SELECT id FROM polls WHERE guild = ? AND creator = ?`, guildId, userId).Scan(&pollId)
	if err != nil {
		return dbPoll{}, fmt.Errorf("error getting poll: %w", err)
	}

	// Get the poll
	poll, err := databasePollGet(pollId)
	if err != nil {
		return dbPoll{}, fmt.Errorf("error getting poll: %w", err)
	}

	return poll, nil
}

// databasePollCheckUser checks if a user has already created a poll within a specified guild and returns true is they have.
func databasePollCheckUser(userId, guildId string) bool {
	var pollId string
	err := db.QueryRow(`SELECT id FROM polls WHERE creator = ? AND guild = ?`, userId, guildId).Scan(&pollId)
	return err == nil
}
