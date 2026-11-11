package agentsview

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/saitota/asq/internal/session"
	"github.com/saitota/asq/internal/source"
)

// TestSmokeAgainstRealAgentsview guards the contract this package assumes of
// the real agentsview CLI: the list JSON shape and the "<agent>:<id>" id
// namespacing. It only runs when ASQ_AGENTSVIEW_SMOKE=1 and agentsview is on
// PATH, because it reads the developer's actual session store.
func TestSmokeAgainstRealAgentsview(t *testing.T) {
	if os.Getenv("ASQ_AGENTSVIEW_SMOKE") != "1" {
		t.Skip("set ASQ_AGENTSVIEW_SMOKE=1 to run against the real agentsview CLI")
	}
	if _, err := exec.LookPath("agentsview"); err != nil {
		t.Skip("agentsview not on PATH")
	}
	listed, err := listAll("agentsview")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) == 0 {
		t.Skip("no sessions in the local agentsview store")
	}
	for _, item := range listed {
		if item.ID == "" || item.Agent == "" {
			t.Fatalf("listed session missing id or agent: %+v", item)
		}
		raw := item.rawID()
		if raw == "" || strings.Contains(raw, ":") {
			t.Fatalf("rawID for %s (%s) = %q", item.ID, item.Agent, raw)
		}
		if _, ok := source.Builtin().Get(session.Source(item.Agent)); ok {
			if _, _, err := source.Builtin().Resume(session.Source(item.Agent), raw); err != nil {
				t.Fatalf("%s session id %q would fail resume: %v", item.Agent, raw, err)
			}
		}
		if item.EndedAt == "" && item.StartedAt == "" {
			t.Fatalf("listed session missing timestamps: %+v", item)
		}
	}
	// Fetch the longest conversation so the ordinal paging contract
	// (last_ordinal advancing across full pages) is exercised for real.
	longest := listed[0]
	for _, item := range listed {
		if item.MessageCount > longest.MessageCount {
			longest = item
		}
	}
	messages, complete, err := fetchMessages("agentsview", longest.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !complete {
		t.Fatalf("paging did not complete for %s (%d listed messages, %d fetched)", longest.ID, longest.MessageCount, len(messages))
	}
	t.Logf("smoke: %d sessions; longest (%s) has %d user/assistant messages", len(listed), longest.ID, len(messages))
}
