package timecontract

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStateFilesystemGate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "control")
	if _, e := ReadState(dir); e == nil {
		t.Fatal("missing allowed")
	}
	s := State{FormatVersion: 1, Status: "pending", Kind: "convert", OperationID: "one", Target: Target{"uuid", "fixture"}}
	if WriteState(dir, s) != nil {
		t.Fatal("write")
	}
	got, e := ReadState(dir)
	if e != nil || Hash(got) != Hash(s) {
		t.Fatal("state mismatch")
	}
	if os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"format_version":1,"status":"ready","operation_id":"one","unknown":true}`), 0600) != nil {
		t.Fatal("fixture")
	}
	if _, e = ReadState(dir); e == nil {
		t.Fatal("unknown field accepted")
	}
	if os.Remove(filepath.Join(dir, "state.json")) != nil {
		t.Fatal("fixture")
	}
	if os.Symlink("/dev/null", filepath.Join(dir, "state.json")) != nil {
		t.Fatal("symlink")
	}
	if _, e = ReadState(dir); e == nil {
		t.Fatal("symlink accepted")
	}
}
func TestOpenRejectsWithoutCredentialDiagnostics(t *testing.T) {
	_, e := OpenUTC(context.Background(), "user:private-placeholder@invalid(")
	if !errors.Is(e, ErrContract) {
		t.Fatal("not sanitized")
	}
}
func TestExpectedDoesNotMutateManifest(t *testing.T) {
	s := Snapshot{Tables: []Table{{Name: "a", Rows: []Row{{ID: "1", Time: "before", After: "after", Protected: "fixed"}}}}}
	out := expected(s)
	if out.Tables[0].Rows[0].Time != "after" || s.Tables[0].Rows[0].Time != "before" {
		t.Fatal("snapshot mutation")
	}
}

func TestStateUnknownKindRejected(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "control")
	h := Hash("valid")
	s := State{FormatVersion: 1, Target: Target{"uuid", "fixture"}, Status: "ready", OperationID: "valid", Kind: "unknown", ContractOperationID: "valid", ContractManifestSHA256: h, ReceiptSHA256: h}
	if WriteState(dir, s) != nil {
		t.Fatal("fixture")
	}
	if _, e := ReadState(dir); e == nil {
		t.Fatal("unknown kind accepted")
	}
}

func TestAuditedDatabaseNames(t *testing.T) {
	for _, v := range []string{"blog", "utc_contract_test_ab01"} {
		if !auditedDatabaseName(v) {
			t.Fatal("audited name rejected")
		}
	}
	for _, v := range []string{"", "Blog", "blog-other", "blog/other", "数据库"} {
		if auditedDatabaseName(v) {
			t.Fatal("unknown dictionary encoding accepted")
		}
	}
}
