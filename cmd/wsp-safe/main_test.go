package main

import (
	"bytes"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/types"
)

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
