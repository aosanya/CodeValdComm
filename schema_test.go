package codevaldcomm_test

import (
	"testing"

	codevaldcomm "github.com/aosanya/CodeValdComm"
)

func TestDefaultCommSchema_TypeDefinitions(t *testing.T) {
	schema := codevaldcomm.DefaultCommSchema()

	wantTypes := []string{"Channel", "Participant", "Message", "EditHistory", "Attachment"}
	typeMap := make(map[string]bool, len(schema.Types))
	for _, td := range schema.Types {
		typeMap[td.Name] = true
	}
	for _, name := range wantTypes {
		if !typeMap[name] {
			t.Errorf("DefaultCommSchema: missing TypeDefinition %q", name)
		}
	}
	if got, want := len(schema.Types), 5; got != want {
		t.Errorf("DefaultCommSchema: got %d TypeDefinitions, want %d", got, want)
	}
}

func TestDefaultCommSchema_StorageCollections(t *testing.T) {
	schema := codevaldcomm.DefaultCommSchema()

	wantCollections := map[string]string{
		"Channel":     "comm_groups",
		"Participant": "comm_participants",
		"Message":     "comm_messages",
		"EditHistory": "comm_edit_history",
		"Attachment":  "comm_attachments",
	}
	for _, td := range schema.Types {
		want, ok := wantCollections[td.Name]
		if !ok {
			continue
		}
		if td.StorageCollection != want {
			t.Errorf("TypeDefinition %q: StorageCollection = %q, want %q", td.Name, td.StorageCollection, want)
		}
	}
}

func TestDefaultCommSchema_ImmutableTypes(t *testing.T) {
	schema := codevaldcomm.DefaultCommSchema()
	immutable := map[string]bool{"EditHistory": true, "Attachment": true}

	for _, td := range schema.Types {
		wantImmutable := immutable[td.Name]
		if td.Immutable != wantImmutable {
			t.Errorf("TypeDefinition %q: Immutable = %v, want %v", td.Name, td.Immutable, wantImmutable)
		}
	}
}

func TestDefaultCommSchema_RelationshipDefinitions(t *testing.T) {
	schema := codevaldcomm.DefaultCommSchema()

	// Collect all relationship names across all types.
	allRels := make(map[string]bool)
	for _, td := range schema.Types {
		for _, rel := range td.Relationships {
			allRels[rel.Name] = true
		}
	}

	wantRels := []string{
		"has_member",
		"has_message",
		"is_reply_to",
		"has_attachment",
		"has_reaction",
		"read_by",
		"has_edit",
	}
	for _, name := range wantRels {
		if !allRels[name] {
			t.Errorf("DefaultCommSchema: missing RelationshipDefinition %q", name)
		}
	}
}

func TestDefaultCommSchema_SchemaID(t *testing.T) {
	schema := codevaldcomm.DefaultCommSchema()
	if schema.ID != "comm-schema-v1" {
		t.Errorf("DefaultCommSchema: ID = %q, want %q", schema.ID, "comm-schema-v1")
	}
	if schema.Version != 1 {
		t.Errorf("DefaultCommSchema: Version = %d, want 1", schema.Version)
	}
}
