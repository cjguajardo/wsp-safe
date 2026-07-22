package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/cgc/wsp-safe/internal/classifier"
	"github.com/cgc/wsp-safe/internal/config"
	"github.com/cgc/wsp-safe/internal/filter"
	waadapter "github.com/cgc/wsp-safe/internal/whatsapp"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if isListGroupsMode(os.Args[1:], os.Getenv) {
		keepAlive := strings.EqualFold(strings.TrimSpace(os.Getenv("WSP_MODE")), "list-groups")
		return runListGroups(ctx, config.SessionDB(os.Getenv), os.Stdout, keepAlive)
	}

	settings, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	container, err := sqlstore.New(ctx, "sqlite3", settings.SessionDB, nil)
	if err != nil {
		return fmt.Errorf("open WhatsApp session: %w", err)
	}
	defer container.Close()
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("load WhatsApp device: %w", err)
	}
	client := whatsmeow.NewClient(device, nil)

	httpClassifier, err := classifier.NewHTTP(
		settings.ClassifierURL,
		settings.ClassifierToken,
		&http.Client{Timeout: 45 * time.Second},
	)
	if err != nil {
		return fmt.Errorf("configure classifier: %w", err)
	}
	service, err := filter.New(filter.Config{
		TargetChatID:    settings.TargetGroupJID,
		SexualThreshold: settings.SexualThreshold,
		DeleteUncertain: settings.DeleteUncertain,
		DeleteOnError:   settings.DeleteOnError,
	}, httpClassifier, waadapter.NewDeleter(client), nil)
	if err != nil {
		return fmt.Errorf("configure filter: %w", err)
	}
	mapper := waadapter.NewMapper(client, settings.MaxMediaBytes)
	registerMessageHandler(ctx, client, mapper, service, settings.TargetGroupJID, settings.Workers)

	if err := connect(ctx, client); err != nil {
		return err
	}
	log.Printf("filter active for group %s", settings.TargetGroupJID)
	<-ctx.Done()
	client.Disconnect()
	return nil
}

func runListGroups(ctx context.Context, sessionDB string, output io.Writer, keepAlive bool) error {
	container, err := sqlstore.New(ctx, "sqlite3", sessionDB, nil)
	if err != nil {
		return fmt.Errorf("open WhatsApp session: %w", err)
	}
	defer container.Close()
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("load WhatsApp device: %w", err)
	}
	client := whatsmeow.NewClient(device, nil)
	if err := connect(ctx, client); err != nil {
		return err
	}
	defer client.Disconnect()
	groups, err := client.GetJoinedGroups(ctx)
	if err != nil {
		return fmt.Errorf("list WhatsApp groups: %w", err)
	}
	printGroups(output, groups)
	if keepAlive {
		log.Print("discovery mode active; set WSP_MODE=run and WSP_TARGET_GROUP_JID, then redeploy")
		<-ctx.Done()
	}
	return nil
}

func printGroups(output io.Writer, groups []*types.GroupInfo) {
	for _, group := range groups {
		if group != nil {
			fmt.Fprintf(output, "%s\t%s\n", group.JID.String(), group.Name)
		}
	}
}

func isListGroupsMode(arguments []string, getenv func(string) string) bool {
	for _, argument := range arguments {
		if argument == "--list-groups" {
			return true
		}
	}
	return strings.EqualFold(strings.TrimSpace(getenv("WSP_MODE")), "list-groups")
}

func connect(ctx context.Context, client *whatsmeow.Client) error {
	if client.Store.ID != nil {
		if err := client.Connect(); err != nil {
			return fmt.Errorf("connect WhatsApp: %w", err)
		}
		return nil
	}

	qrChannel, err := client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("start QR login: %w", err)
	}
	if err := client.Connect(); err != nil {
		return fmt.Errorf("connect WhatsApp for QR login: %w", err)
	}
	for event := range qrChannel {
		if event.Event == "code" {
			fmt.Println("Escaneá este QR desde WhatsApp > Dispositivos vinculados:")
			qrterminal.GenerateHalfBlock(event.Code, qrterminal.L, os.Stdout)
			continue
		}
		log.Printf("WhatsApp login event: %s", event.Event)
	}
	if client.Store.ID == nil {
		return errors.New("WhatsApp login ended before the account was linked")
	}
	return nil
}

type messageMapper interface {
	Map(context.Context, *events.Message) (filter.Message, error)
}

type messageService interface {
	Handle(context.Context, filter.Message) (filter.Decision, error)
}

func registerMessageHandler(
	ctx context.Context,
	client *whatsmeow.Client,
	mapper messageMapper,
	service messageService,
	targetChatID string,
	workerCount int,
) {
	workers := make(chan struct{}, workerCount)
	client.AddEventHandler(func(raw any) {
		event, ok := raw.(*events.Message)
		if !ok || event.Info.Chat.String() != targetChatID || event.Info.IsFromMe {
			return
		}
		select {
		case workers <- struct{}{}:
		case <-ctx.Done():
			return
		}
		go func() {
			defer func() { <-workers }()
			message, err := mapper.Map(ctx, event)
			if err != nil {
				log.Printf("map WhatsApp message %s: %v", event.Info.ID, err)
				return
			}
			decision, err := service.Handle(ctx, message)
			if err != nil {
				log.Printf("process WhatsApp message %s: %v", event.Info.ID, err)
				return
			}
			if decision.Delete {
				log.Printf("deleted message %s for me: %s", event.Info.ID, decision.Reason)
			}
		}()
	})
}
