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
		return runListGroups(
			ctx,
			config.SessionDB(os.Getenv),
			strings.TrimSpace(os.Getenv("WSP_PAIR_PHONE")),
			os.Stdout,
			keepAlive,
		)
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
	registerMessageHandler(
		ctx,
		client,
		mapper,
		service,
		settings.TargetGroupJID,
		settings.Workers,
		settings.LogDecisions,
	)

	if err := connect(ctx, client, strings.TrimSpace(os.Getenv("WSP_PAIR_PHONE")), os.Stdout); err != nil {
		return err
	}
	log.Printf("filter active for group %s", settings.TargetGroupJID)
	<-ctx.Done()
	client.Disconnect()
	return nil
}

func runListGroups(
	ctx context.Context,
	sessionDB string,
	pairPhone string,
	output io.Writer,
	keepAlive bool,
) error {
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
	if err := connect(ctx, client, pairPhone, output); err != nil {
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

func connect(ctx context.Context, client *whatsmeow.Client, pairPhone string, output io.Writer) error {
	if client.Store.ID != nil {
		if err := client.Connect(); err != nil {
			return fmt.Errorf("connect WhatsApp: %w", err)
		}
		return nil
	}
	if err := connectUnlinked(ctx, client, pairPhone, output); err != nil {
		return err
	}
	if client.Store.ID == nil {
		return errors.New("WhatsApp login ended before the account was linked")
	}
	return nil
}

type pairingClient interface {
	GetQRChannel(context.Context) (<-chan whatsmeow.QRChannelItem, error)
	Connect() error
	PairPhone(context.Context, string, bool, whatsmeow.PairClientType, string) (string, error)
}

func connectUnlinked(ctx context.Context, client pairingClient, pairPhone string, output io.Writer) error {
	qrChannel, err := client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("start QR login: %w", err)
	}
	if err := client.Connect(); err != nil {
		return fmt.Errorf("connect WhatsApp for QR login: %w", err)
	}
	if pairPhone != "" {
		firstEvent, open := <-qrChannel
		if !open {
			return errors.New("WhatsApp login channel closed before phone pairing started")
		}
		if firstEvent.Event != whatsmeow.QRChannelEventCode {
			return pairingEventError(firstEvent)
		}
		code, err := client.PairPhone(
			ctx,
			pairPhone,
			true,
			whatsmeow.PairClientChrome,
			"Chrome (Linux)",
		)
		if err != nil {
			return fmt.Errorf("generate WhatsApp phone pairing code: %w", err)
		}
		fmt.Fprintln(output, "Código de vinculación de WhatsApp:", code)
		fmt.Fprintln(output, "En el teléfono, abre WhatsApp > Dispositivos vinculados > Vincular un dispositivo > Vincular con número de teléfono.")
		return waitForPairing(ctx, qrChannel, nil)
	}
	return waitForPairing(ctx, qrChannel, func(code string) {
		fmt.Fprintln(output, "Escanea este código QR desde WhatsApp > Dispositivos vinculados:")
		qrterminal.GenerateHalfBlock(code, qrterminal.L, output)
	})
}

func waitForPairing(
	ctx context.Context,
	qrChannel <-chan whatsmeow.QRChannelItem,
	printQR func(string),
) error {
	for event := range qrChannel {
		switch event.Event {
		case whatsmeow.QRChannelEventCode:
			if printQR != nil {
				printQR(event.Code)
			}
		case whatsmeow.QRChannelSuccess.Event:
			return nil
		default:
			return pairingEventError(event)
		}
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("WhatsApp login canceled: %w", err)
	}
	return errors.New("WhatsApp login channel closed before the account was linked")
}

func pairingEventError(event whatsmeow.QRChannelItem) error {
	if event.Error != nil {
		return fmt.Errorf("WhatsApp login event %s: %w", event.Event, event.Error)
	}
	return fmt.Errorf("WhatsApp login ended with event: %s", event.Event)
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
	logDecisions bool,
) {
	workers := make(chan struct{}, workerCount)
	client.AddEventHandler(func(raw any) {
		event, ok := raw.(*events.Message)
		if !ok || event.Info.Chat.String() != targetChatID {
			return
		}
		if event.Info.IsFromMe {
			if logDecisions {
				log.Printf("mensaje propio ignorado: id=%s", event.Info.ID)
			}
			return
		}
		if logDecisions {
			log.Printf("mensaje recibido del grupo configurado: id=%s", event.Info.ID)
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
			if logDecisions {
				log.Print(formatModerationDecision(message, decision))
			}
			if decision.Delete {
				log.Printf("deleted message %s for me: %s", event.Info.ID, decision.Reason)
			}
		}()
	})
}

func formatModerationDecision(message filter.Message, decision filter.Decision) string {
	return fmt.Sprintf(
		"decisión de moderación: id=%s tipo=%s eliminar=%t motivo=%s puntuación_sexual=%.3f puntuación_sexual_menores=%.3f dudoso=%t",
		message.ID,
		message.Kind,
		decision.Delete,
		decision.Reason,
		decision.Result.SexualScore,
		decision.Result.SexualMinorsScore,
		decision.Result.Uncertain,
	)
}
