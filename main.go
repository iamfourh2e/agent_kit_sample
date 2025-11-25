package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"
)

// --- 1. STATIC TOOL FUNCTIONS ---
// As requested, these are static functions that just return a
// confirmation string. This is where you'd call a real API.
type bookHotelArg struct {
	Location string `json:"location" jsonschema:"the location of the hotel"`
	Date     string `json:"date" jsonschema:"the date of the booking"`
}
type bookHotelResult struct {
	Status       string `json:"status"`
	Report       string `json:"report,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

func bookHotel(c tool.Context, arg bookHotelArg) bookHotelResult {
	confirmation := "CONF_HOTEL_98765"
	fmt.Printf("%v", arg)
	return bookHotelResult{
		Status:       "success",
		Report:       fmt.Sprintf("Hotel booked in %s on %s. Confirmation: %s", arg.Location, arg.Date, confirmation),
		ErrorMessage: "",
	}
}

type bookFlightArg struct {
	Origin      string `json:"origin" jsonschema:"the origin of the flight"`
	Destination string `json:"destination" jsonschema:"the destination of the flight"`
	Date        string `json:"date" jsonschema:"the date of the booking"`
}
type bookFlightResult struct {
	Status       string `json:"status"`
	Report       string `json:"report,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

func bookFlight(c tool.Context, arg bookFlightArg) bookFlightResult {
	confirmation := "CONF_FLIGHT_12345"
	fmt.Printf("%v", arg)
	return bookFlightResult{
		Status:       "success",
		Report:       fmt.Sprintf("Flight booked from %s to %s on %s. Confirmation: %s", arg.Origin, arg.Destination, arg.Date, confirmation),
		ErrorMessage: "",
	}
}

// ---------------------------------

func main() {
	if err := runAgent(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runAgent() error {
	if err := godotenv.Load(); err != nil {
		return fmt.Errorf("loading .env file: %w", err)
	}

	ctx := context.Background()
	key := os.Getenv("API_KEY")
	if key == "" {
		return fmt.Errorf("API_KEY environment variable is not set")
	}

	model, err := gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{
		APIKey: key,
	})
	if err != nil {
		return fmt.Errorf("creating Gemini model: %w", err)
	}

	// --- 2. CREATE TOOLS FROM YOUR FUNCTIONS ---
	hotelTool, err := functiontool.New(
		functiontool.Config{
			Name:        "bookHotel",
			Description: "Use this function to book a hotel. Requires location and date.",
		},
		bookHotel,
	)
	if err != nil {
		return fmt.Errorf("creating hotel tool: %w", err)
	}

	flightTool, err := functiontool.New(
		functiontool.Config{
			Name:        "bookFlight",
			Description: "Use this function to book a flight. Requires origin, destination, and date.",
		},
		bookFlight,
	)
	if err != nil {
		return fmt.Errorf("creating flight tool: %w", err)
	}

	// -------------------------------------------

	// --- 3. ADD TOOLS TO YOUR AGENT ---
	bookingAgent, err := llmagent.New(llmagent.Config{
		Name:        "Booker",
		Description: "Handles flight and hotel bookings. Use your tools for any booking request.",
		Model:       model,
		Tools:       []tool.Tool{hotelTool, flightTool},
	})
	if err != nil {
		return fmt.Errorf("creating booking agent: %w", err)
	}

	infoAgent, err := llmagent.New(llmagent.Config{
		Name:        "Info",
		Description: "Provides general information and answers questions.",
		Model:       model,
	})
	if err != nil {
		return fmt.Errorf("creating info agent: %w", err)
	}

	coordinator, err := llmagent.New(llmagent.Config{
		Name:        "Coordinator",
		Model:       model,
		Instruction: "You are an assistant. Delegate booking tasks to Booker and info requests to Info.",
		Description: "Main coordinator.",
		SubAgents:   []agent.Agent{bookingAgent, infoAgent},
	})
	if err != nil {
		return fmt.Errorf("creating coordinator agent: %w", err)
	}

	sessionService := session.InMemoryService()
	runner, err := runner.New(runner.Config{
		AppName:        "booking_planner",
		Agent:          coordinator,
		SessionService: sessionService,
	})
	if err != nil {
		log.Fatal(err)
	}

	session, err := sessionService.Create(ctx, &session.CreateRequest{
		AppName: "booking_planner",
		UserID:  "user1234",
	})
	if err != nil {
		log.Fatal(err)
	}

	run(ctx, runner, session.Session.ID(), "i want to visit in london?")
	run(ctx, runner, session.Session.ID(), "on 2025-11-14")
	run(ctx, runner, session.Session.ID(), "also book a hotel for me ")

	return nil

}
func run(ctx context.Context, r *runner.Runner, sessionID string, prompt string) {
	fmt.Printf("\n> %s\n", prompt)
	events := r.Run(
		ctx,
		"user1234",
		sessionID,
		genai.NewContentFromText(prompt, genai.RoleUser),
		agent.RunConfig{
			StreamingMode: agent.StreamingModeNone,
		},
	)
	for event, err := range events {
		if err != nil {
			log.Fatalf("ERROR during agent execution: %v", err)
		}

		if event.Content.Parts[0].Text != "" {
			fmt.Printf("Agent Response: %s\n", event.Content.Parts[0].Text)
		}
	}
}
