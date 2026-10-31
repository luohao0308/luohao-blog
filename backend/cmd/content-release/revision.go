package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"go.einride.tech/aip/fieldmask"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"io"
	"strings"
	"time"
)

type revisionEntry struct {
	Slug      string          `json:"slug"`
	ID        string          `json:"id"`
	Expected  string          `json:"expectedUpdatedAt"`
	Source    string          `json:"sourceUrl"`
	Seed      string          `json:"seedPath,omitempty"`
	Before    json.RawMessage `json:"before"`
	After     json.RawMessage `json:"after"`
	Mask      []string        `json:"updateMask"`
	Preserved json.RawMessage `json:"preserved"`
}
type revisionDocument struct {
	Version     int               `json:"schemaVersion"`
	Base        string            `json:"baseSha"`
	Status      string            `json:"status"`
	Captured    string            `json:"capturedAt"`
	Sources     map[string]string `json:"sources"`
	Constraints []string          `json:"executionConstraints"`
	Projects    []revisionEntry   `json:"projects"`
	Articles    []revisionEntry   `json:"articles"`
}

var revisionProjectPreserved = []string{"cover_url", "screenshots", "repo_url", "live_url", "published", "sort", "featured", "year", "problem", "approach", "outcome"}
var revisionArticlePreserved = []string{"status", "published_at", "tags", "cover_url", "featured", "category", "created_at"}

func revisionKeys(raw json.RawMessage) (map[string]json.RawMessage, []string, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) == 0 {
		return nil, nil, errRelease
	}
	keys := []string{}
	for k := range fields {
		keys = append(keys, k)
	}
	return fields, keys, nil
}

// Comparison uses protobuf semantics for absent/default fields, repeated values,
// timestamps and optional messages, while retaining the explicitly named keys.
func compareRevisionFields(current proto.Message, raw json.RawMessage) error {
	_, keys, err := revisionKeys(raw)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if current.ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name(key)) == nil {
			return errRelease
		}
	}
	patch := current.ProtoReflect().New().Interface()
	if protojson.Unmarshal(raw, patch) != nil {
		return errRelease
	}
	merged := proto.Clone(current)
	fieldmask.Update(&fieldmaskpb.FieldMask{Paths: keys}, merged, patch)
	if !proto.Equal(merged, current) {
		return errRelease
	}
	return nil
}
func loadRevision(raw []byte, hash string, apply bool) (*bundle, error) {
	sum := sha256.Sum256(raw)
	if (apply || hash != "") && hash != hex.EncodeToString(sum[:]) {
		return nil, errRelease
	}
	var doc revisionDocument
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&doc) != nil || decoder.Decode(new(any)) != io.EOF || doc.Version != 1 || doc.Status != "proposal_only_not_applied" || doc.Base != "6c73272708fb7ef20e83a5c76e54adcea6ac1220" || len(doc.Projects) != 4 || len(doc.Articles) != 2 {
		return nil, errRelease
	}
	if _, err := time.Parse(time.RFC3339Nano, doc.Captured); err != nil {
		return nil, errRelease
	}
	out := &bundle{Version: 1}
	seen := map[string]bool{}
	ids := map[string]bool{}
	for _, group := range []struct {
		kind                      string
		entries                   []revisionEntry
		slugs, allowed, preserved []string
	}{
		{"project", doc.Projects, []string{"aigc", "forge-harness", "dev-workflow", "luohao-blog"}, []string{"name", "summary", "description", "stack", "role"}, revisionProjectPreserved},
		{"article", doc.Articles, []string{"aigc-backend-delivery", "forge-harness-project"}, []string{"title", "summary", "content_md"}, revisionArticlePreserved},
	} {
		for _, entry := range group.entries {
			key := group.kind + "/" + entry.Slug
			if !containsRevision(group.slugs, entry.Slug) || seen[key] || entry.ID == "" || ids[entry.ID] || entry.Source != "https://makerhao.cn/api/v1/"+group.kind+"s/"+entry.Slug {
				return nil, errRelease
			}
			seen[key] = true
			ids[entry.ID] = true
			if _, err := time.Parse(time.RFC3339Nano, entry.Expected); err != nil {
				return nil, errRelease
			}
			before, beforeKeys, err := revisionKeys(entry.Before)
			if err != nil {
				return nil, err
			}
			after, afterKeys, err := revisionKeys(entry.After)
			if err != nil {
				return nil, err
			}
			_, preserveKeys, err := revisionKeys(entry.Preserved)
			if err != nil || !same(group.preserved, preserveKeys) || !same(beforeKeys, entry.Mask) || !same(afterKeys, entry.Mask) {
				return nil, errRelease
			}
			masks := map[string]bool{}
			for _, path := range entry.Mask {
				if masks[path] || !containsRevision(group.allowed, path) || bytes.Equal(bytes.TrimSpace(after[path]), []byte("null")) || bytes.Equal(bytes.TrimSpace(before[path]), []byte("null")) {
					return nil, errRelease
				}
				masks[path] = true
			}
			var message proto.Message = &v1.Project{}
			if group.kind == "article" {
				message = &v1.Article{}
			}
			for _, fields := range []json.RawMessage{entry.Before, entry.After, entry.Preserved} {
				if protojson.Unmarshal(fields, message.ProtoReflect().New().Interface()) != nil {
					return nil, errRelease
				}
			}
			// Identity is provided separately; after contains only the approved mask.
			slug, _ := json.Marshal(entry.Slug)
			after["slug"] = slug
			fields, _ := json.Marshal(after)
			out.Operations = append(out.Operations, operation{Kind: "update_" + group.kind, Slug: entry.Slug, Expected: entry.Expected, Fields: fields, Mask: entry.Mask, RevisionID: entry.ID, Before: entry.Before, Preserved: entry.Preserved})
		}
	}
	return out, nil
}
func containsRevision(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
func revisionIdentity(current proto.Message, o operation) error {
	id := current.ProtoReflect().Descriptor().Fields().ByName("id")
	if id == nil || current.ProtoReflect().Get(id).String() != o.RevisionID {
		return errRelease
	}
	return nil
}
func checkRevisionResult(current proto.Message, o operation) error {
	if err := revisionIdentity(current, o); err != nil {
		return err
	}
	if err := compareRevisionFields(current, o.Fields); err != nil {
		return err
	}
	return compareRevisionFields(current, o.Preserved)
}

// Isolated mode only permits updates. It never reaches the creator branches.
func isRevisionOperation(o operation) bool {
	return o.RevisionID != "" && strings.HasPrefix(o.Kind, "update_")
}
