package email

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func getConfig() *oauth2.Config {
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("unable to read credentials.json: %v", err)
	}

	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
	if err != nil {
		log.Fatalf("unable to parse credentials.json: %v", err)
	}

	return config
}

func getToken(config *oauth2.Config) *oauth2.Token {
	tokenFile := "token.json"

	if f, err := os.Open(tokenFile); err == nil {
		defer f.Close()
		token := &oauth2.Token{}
		json.NewDecoder(f).Decode(token)
		return token
	}

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("open this link in your browser:\n%v\n\nenter the auth code: ", authURL)

	var authCode string
	fmt.Scan(&authCode)

	token, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("unable to get token: %v", err)
	}

	f, _ := os.Create(tokenFile)
	defer f.Close()
	json.NewEncoder(f).Encode(token)

	return token
}

func NewGmailService() *gmail.Service {
	config := getConfig()
	token := getToken(config)
	client := config.Client(context.Background(), token)

	service, err := gmail.NewService(context.Background(),
		option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("unable to create gmail service: %v", err)
	}

	return service
}
