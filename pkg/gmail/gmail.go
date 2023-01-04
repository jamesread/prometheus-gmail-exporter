package gmail

import (
	"context"
	"google.golang.org/api/gmail/v1"
	"golang.org/x/oauth2/google"
	log "github.com/sirupsen/logrus"

	"os"
	"time"
)

func getCredentials() ([]byte) {
	for {
		_, err := os.Stat("credentials"); 

		if os.IsNotExist(err) {
			log.Infof("Credentials does not exist. Sleeping.")
			time.Sleep(10 * time.Second)
			continue
		} else {
			break;
		}
	}

	return []byte {}
}

func getClient() (*gmail.Service, error) {
	credentials := getCredentials()

	google.ConfigFromJSON(credentials, gmail.GmailReadonlyScope)

	ctx := context.Background()
	client, err := gmail.NewService(ctx)

	return client, err
}

func UpdateLoop() {
	client, err := getClient()

	if err != nil {
		log.Fatalf("%v", err)
	}

	foo, err := client.Users.Labels.List("me").Do()

	if err != nil {
		log.Fatalf("Label list: %v", err)
	}

	log.Infof("%v", foo)
}
