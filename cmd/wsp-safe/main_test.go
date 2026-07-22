package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"

	"github.com/cgc/wsp-safe/internal/filter"
)

type fakePairingClient struct {
	channel   chan whatsmeow.QRChannelItem
	connected bool
	phone     string
	pairCode  string
}

func (client *fakePairingClient) GetQRChannel(context.Context) (<-chan whatsmeow.QRChannelItem, error) {
	return client.channel, nil
}

func (client *fakePairingClient) Connect() error {
	client.connected = true
	return nil
}

func (client *fakePairingClient) PairPhone(
	_ context.Context,
	phone string,
	_ bool,
	_ whatsmeow.PairClientType,
	_ string,
) (string, error) {
	client.phone = phone
	return client.pairCode, nil
}

func TestPrintGroups(t *testing.T) {
	jid, err := types.ParseJID("120363000000000000@g.us")
	if err != nil {
		t.Fatalf("ParseJID() error = %v", err)
	}
	var output bytes.Buffer
	printGroups(&output, []*types.GroupInfo{{
		JID:       jid,
		GroupName: types.GroupName{Name: "My group"},
	}})
	if got := output.String(); !strings.Contains(got, "120363000000000000@g.us\tMy group") {
		t.Errorf("output = %q", got)
	}
}

func TestIsListGroupsMode(t *testing.T) {
	if !isListGroupsMode([]string{"--list-groups"}, func(string) string { return "" }) {
		t.Error("CLI flag did not enable list-groups mode")
	}
	if !isListGroupsMode(nil, func(key string) string {
		if key == "WSP_MODE" {
			return "list-groups"
		}
		return ""
	}) {
		t.Error("WSP_MODE did not enable list-groups mode")
	}
}

func TestConnectUnlinkedWithPhoneCode(t *testing.T) {
	channel := make(chan whatsmeow.QRChannelItem, 2)
	channel <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "qr-no-utilizado"}
	channel <- whatsmeow.QRChannelSuccess
	close(channel)
	client := &fakePairingClient{channel: channel, pairCode: "ABCD-EFGH"}
	var output bytes.Buffer

	err := connectUnlinked(context.Background(), client, "+56 9 1234 5678", &output)

	if err != nil {
		t.Fatalf("connectUnlinked() error = %v", err)
	}
	if !client.connected {
		t.Error("Connect() no fue ejecutado")
	}
	if client.phone != "+56 9 1234 5678" {
		t.Errorf("PairPhone() teléfono = %q", client.phone)
	}
	if !strings.Contains(output.String(), "ABCD-EFGH") {
		t.Errorf("salida = %q; falta el código de vinculación", output.String())
	}
}

func TestFormatModerationDecisionDoesNotExposeContent(t *testing.T) {
	message := filter.Message{
		ID:       "mensaje-123",
		SenderID: "remitente-privado",
		Kind:     filter.KindText,
		Text:     "contenido privado",
	}
	decision := filter.Decision{
		Delete: true,
		Reason: filter.ReasonSexual,
		Result: filter.Result{SexualScore: 0.91},
	}

	entry := formatModerationDecision(message, decision)

	for _, expected := range []string{"mensaje-123", "text", "eliminar=true", "sexual_content", "puntuación_sexual=0.910"} {
		if !strings.Contains(entry, expected) {
			t.Errorf("registro = %q; falta %q", entry, expected)
		}
	}
	for _, privateValue := range []string{"remitente-privado", "contenido privado"} {
		if strings.Contains(entry, privateValue) {
			t.Errorf("registro = %q; expone %q", entry, privateValue)
		}
	}
}
