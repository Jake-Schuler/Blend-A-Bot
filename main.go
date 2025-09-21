package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var removeCommands = flag.Bool("remove", false, "Remove all commands from the bot")

var s *discordgo.Session

func init() { flag.Parse() }

func init() {
	godotenv.Load(".env")
	var err error
	s, err = discordgo.New("Bot " + os.Getenv("DISCORD_BOT_TOKEN"))
	if err != nil {
		log.Fatalf("Invalid bot parameters: %v", err)
	}
}

type TBATeam struct {
	Nickname   string `json:"nickname"`
	City       string `json:"city"`
	StateProv  string `json:"state_prov"`
	Country    string `json:"country"`
	RookieYear int    `json:"rookie_year"`
	Sponsors   string `json:"name"`
	School     string `json:"school_name"`
	Website    string `json:"website"`
	TeamLogo   string `json:"team_logo"`
	TeamNumber int    `json:"team_number"`
}

type TBAEvent struct {
	Name     string `json:"name"`
	EventKey string `json:"key"`
}

var (
	commands = []*discordgo.ApplicationCommand{
		{
			Name:        "ping",
			Description: "Ping the bot",
		},
		{
			Name:        "lmgtfy",
			Description: "Generate a LMGTFY link",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "search",
					Description: "The search",
					Required:    true,
				},
			},
		},
		{
			Name:        "tba",
			Description: "Fetch data from The Blue Alliance",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "eventsfor",
					Description: "get data about an event",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        "teamnumber",
							Description: "The number of the team",
							Required:    true,
							Type:        discordgo.ApplicationCommandOptionString,
						},
					},
				},
				{
					Name:        "team",
					Description: "get data about a team",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        "teamnumber",
							Description: "The number of the team",
							Required:    true,
							Type:        discordgo.ApplicationCommandOptionString,
						},
					},
				},
			},
		},
		{
			Name:        "httpcat",
			Description: "cat image for a HTTP status code",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "statuscode",
					Description: "The HTTP status code",
					Required:    true,
					Type:        discordgo.ApplicationCommandOptionInteger,
				},
			},
		},
	}
	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"ping": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Pong!",
				},
			})
			if err != nil {
				log.Println("Error responding to ping command:", err)
			}
		},
		"lmgtfy": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "https://letmegooglethat.com/?q=" + i.ApplicationCommandData().Options[0].StringValue(),
				},
			})
			if err != nil {
				log.Println("Error responding to LMGTFY command:", err)
			}
		},
		"tba": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options
			subCommand := options[0]
			var content discordgo.MessageEmbed
			switch subCommand.Name {
			case "team":
				teamNumber := subCommand.Options[0].StringValue()
				content = getTBATeam("frc" + teamNumber)
			case "eventsfor":
				teamNumber := subCommand.Options[0].StringValue()
				content = getTBAEventsFor("frc" + teamNumber)
			}
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{&content},
				},
			})
			if err != nil {
				log.Println("Error responding to TBA command:", err)
			}
		},
		"httpcat": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "https://http.cat/" + fmt.Sprintf("%d", i.ApplicationCommandData().Options[0].IntValue()) + ".jpg",
				},
			})
			if err != nil {
				log.Println("Error responding to HTTP cat command:", err)
			}
		},
	}
)

func init() {
	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
}

func main() {
	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	})
	err := s.Open()
	if err != nil {
		log.Fatalf("Cannot open the session: %v", err)
	}

	log.Println("Adding commands...")
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, v := range commands {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, "", v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
	}

	defer s.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	log.Println("Press Ctrl+C to exit")
	<-stop

	if *removeCommands {
		log.Println("Removing commands...")
		// // We need to fetch the commands, since deleting requires the command ID.
		// // We are doing this from the returned commands on line 375, because using
		// // this will delete all the commands, which might not be desirable, so we
		// // are deleting only the commands that we added.
		// registeredCommands, err := s.ApplicationCommands(s.State.User.ID, *GuildID)
		// if err != nil {
		// 	log.Fatalf("Could not fetch registered commands: %v", err)
		// }

		for _, v := range registeredCommands {
			err := s.ApplicationCommandDelete(s.State.User.ID, "", v.ID)
			if err != nil {
				log.Panicf("Cannot delete '%v' command: %v", v.Name, err)
			}
		}
	}

	log.Println("Gracefully shutting down.")
}

func getTBATeam(teamNumber string) discordgo.MessageEmbed {
	var team TBATeam
	req, err := http.NewRequest("GET", "https://www.thebluealliance.com/api/v3/team/"+teamNumber, nil)
	if err != nil {
		log.Println("Error creating TBA request:", err)
		return discordgo.MessageEmbed{
			Title:       "Error",
			Description: "Error creating TBA request",
		}
	}

	req.Header.Set("X-TBA-Auth-Key", os.Getenv("TBA_AUTH_KEY"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error fetching TBA data:", err)
		return discordgo.MessageEmbed{
			Title:       "Error",
			Description: "Error fetching TBA data",
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading TBA response body:", err)
		return discordgo.MessageEmbed{
			Title:       "Error",
			Description: "Error reading TBA response body",
		}
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error fetching TBA data: received status code %d, body: %s", resp.StatusCode, string(body))
		return discordgo.MessageEmbed{
			Title:       "Error",
			Description: fmt.Sprintf("Error: received non-200 status code from TBA (%d)", resp.StatusCode),
		}
	}

	if err := json.Unmarshal(body, &team); err != nil {
		log.Println("Error unmarshalling TBA response:", err)
		return discordgo.MessageEmbed{
			Title:       "Error",
			Description: "Error parsing TBA response",
		}
	}

	log.Println("TBA data fetched successfully:", team)
	return formatTBATeam(team)
}

func getTBAEventsFor(teamNumber string) discordgo.MessageEmbed {
	var events []TBAEvent
	req, err := http.NewRequest("GET", "https://www.thebluealliance.com/api/v3/team/"+teamNumber+"/events/"+fmt.Sprint(time.Now().Year()), nil)
	if err != nil {
		log.Println("Error creating TBA request:", err)
		return discordgo.MessageEmbed{
			Title:       "Error",
			Description: "Error creating TBA request",
		}
	}

	req.Header.Set("X-TBA-Auth-Key", os.Getenv("TBA_AUTH_KEY"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error fetching TBA data:", err)
		return discordgo.MessageEmbed{
			Title:       "Error",
			Description: "Error fetching TBA data",
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading TBA response body:", err)
		return discordgo.MessageEmbed{
			Title:       "Error",
			Description: "Error reading TBA response body",
		}
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error fetching TBA data: received status code %d, body: %s", resp.StatusCode, string(body))
		return discordgo.MessageEmbed{
			Title:       "Error",
			Description: fmt.Sprintf("Error: received non-200 status code from TBA (%d)", resp.StatusCode),
		}
	}

	if err := json.Unmarshal(body, &events); err != nil {
		log.Println("Error unmarshalling TBA response:", err)
		return discordgo.MessageEmbed{
			Title:       "Error",
			Description: "Error parsing TBA response",
		}
	}

	log.Println("TBA data fetched successfully:", events)
	if len(events) == 0 {
		return discordgo.MessageEmbed{
			Title:       "No Events Found",
			Description: "No events found for the specified team this year.",
		}
	}
	eventList := ""
	for _, event := range events {
		eventList += fmt.Sprintf("[%s](https://www.thebluealliance.com/event/%s)\n", event.Name, event.EventKey)
	}
	return discordgo.MessageEmbed{
		Title:       "Events for Team " + teamNumber,
		Description: eventList,
	}
}

// formatTBATeam formats the TBATeam struct as a string for display.
func formatTBATeam(team TBATeam) discordgo.MessageEmbed {
	return discordgo.MessageEmbed{
		Title: "Team " + fmt.Sprint(team.TeamNumber) + "(" + team.Nickname + ")",
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Team", Value: team.Nickname, Inline: true},
			{Name: "Location", Value: fmt.Sprintf("%s, %s, %s", team.City, team.StateProv, team.Country), Inline: true},
			{Name: "Rookie Year", Value: fmt.Sprint(team.RookieYear), Inline: true},
			{Name: "Sponsors", Value: team.Sponsors, Inline: true},
			{Name: "School", Value: team.School, Inline: true},
			{Name: "Website", Value: team.Website, Inline: true},
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: fmt.Sprintf("https://www.thebluealliance.com/avatar/%d/frc%d.png", time.Now().Year(), team.TeamNumber),
		},
		URL: fmt.Sprintf("https://www.thebluealliance.com/team/%d", team.TeamNumber),
	}
}
