package syncengine

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// PantryMergeResult summarizes a deterministic conflict-free `.asterlab/pantry.jsonl` merge.
type PantryMergeResult struct {
	TotalLines        int `json:"totalLines"`
	SchemaVersion     int `json:"schemaVersion"`
	LocalRecords      int `json:"localRecords"`
	RemoteRecords     int `json:"remoteRecords"`
	DeduplicatedCount int `json:"deduplicatedCount"`
}

type pantryProbe struct {
	Type      string `json:"type"`
	Version   int    `json:"version"`
	ID        string `json:"id"`
	TxID      string `json:"tx_id"`
	TxIDCamel string `json:"txId"`
	Revision  int64  `json:"revision"`
	UpdatedAt string `json:"updatedAt"`
	Timestamp string `json:"timestamp"`
	CreatedAt string `json:"createdAt"`
	DateYMD   string `json:"dateYMD"`
}

type parsedPantryRecord struct {
	raw        string
	canon      string
	hash       string
	txID       string
	kind       string
	entityID   string
	revision   int64
	updatedAt  string
	sortTime   string
	isSchema   bool
	schemaVer  int
}

// MergePantryJSONL performs a deterministic, commutative, append-only deduplicated
// merge of two `.asterlab/pantry.jsonl` files:
//   - Any invalid JSON line immediately returns an error (never drops or wipes user data).
//   - Schema header lines (`type: "schema"`) converge to the highest schema version at line 1.
//   - Records carrying `tx_id` / `txId` (or append-only log/tx records) are deduplicated
//     by `tx_id` (and canonical SHA-256 hash) so concurrent additions from desktop & mobile
//     never overwrite one another.
//   - Snapshot entity records (`box`, `reagent`, `meta`, `vivarium` without `tx_id`)
//     deduplicate identical hashes and resolve differing revisions deterministically by
//     highest `revision`, then newest `updatedAt`, then canonical hash.
func MergePantryJSONL(localData, remoteData []byte) ([]byte, PantryMergeResult, error) {
	localRecs, err := parsePantryLines(localData, "local")
	if err != nil {
		return nil, PantryMergeResult{}, err
	}
	remoteRecs, err := parsePantryLines(remoteData, "remote")
	if err != nil {
		return nil, PantryMergeResult{}, err
	}

	res := PantryMergeResult{
		SchemaVersion: 1,
		LocalRecords:  len(localRecs),
		RemoteRecords: len(remoteRecs),
	}

	var schemaRec *parsedPantryRecord
	txByID := map[string]parsedPantryRecord{}
	entityByKey := map[string]parsedPantryRecord{}
	hashOnly := map[string]parsedPantryRecord{}
	totalSeen := 0

	ingest := func(r parsedPantryRecord) {
		totalSeen++
		if r.isSchema {
			if r.schemaVer > res.SchemaVersion {
				res.SchemaVersion = r.schemaVer
			}
			if schemaRec == nil || r.schemaVer >= schemaRec.schemaVer {
				cp := r
				schemaRec = &cp
			}
			return
		}
		if r.txID != "" {
			if existing, ok := txByID[r.txID]; ok {
				res.DeduplicatedCount++
				// Commutative tie-break if same tx_id appears twice with identical or differing payload.
				if r.hash < existing.hash {
					txByID[r.txID] = r
				}
				return
			}
			txByID[r.txID] = r
			return
		}
		if r.kind != "" && r.entityID != "" && isSnapshotEntityKind(r.kind) {
			key := r.kind + "\x00" + r.entityID
			if existing, ok := entityByKey[key]; ok {
				res.DeduplicatedCount++
				if shouldReplaceEntityRecord(existing, r) {
					entityByKey[key] = r
				}
				return
			}
			entityByKey[key] = r
			return
		}
		if _, ok := hashOnly[r.hash]; ok {
			res.DeduplicatedCount++
			return
		}
		hashOnly[r.hash] = r
	}

	for _, r := range localRecs {
		ingest(r)
	}
	for _, r := range remoteRecs {
		ingest(r)
	}

	var outLines []string
	if schemaRec != nil {
		outLines = append(outLines, schemaRec.canon)
	} else if totalSeen > 0 {
		outLines = append(outLines, fmt.Sprintf(`{"type":"schema","version":%d}`, res.SchemaVersion))
	}

	entities := make([]parsedPantryRecord, 0, len(entityByKey))
	for _, r := range entityByKey {
		entities = append(entities, r)
	}
	sort.Slice(entities, func(i, j int) bool {
		oi, oj := entityKindOrder(entities[i].kind), entityKindOrder(entities[j].kind)
		if oi != oj {
			return oi < oj
		}
		if entities[i].entityID != entities[j].entityID {
			return entities[i].entityID < entities[j].entityID
		}
		return entities[i].hash < entities[j].hash
	})
	for _, r := range entities {
		outLines = append(outLines, r.canon)
	}

	txs := make([]parsedPantryRecord, 0, len(txByID))
	for _, r := range txByID {
		txs = append(txs, r)
	}
	sort.Slice(txs, func(i, j int) bool {
		if txs[i].sortTime != txs[j].sortTime {
			return txs[i].sortTime < txs[j].sortTime
		}
		if txs[i].txID != txs[j].txID {
			return txs[i].txID < txs[j].txID
		}
		return txs[i].hash < txs[j].hash
	})
	for _, r := range txs {
		outLines = append(outLines, r.canon)
	}

	misc := make([]parsedPantryRecord, 0, len(hashOnly))
	for _, r := range hashOnly {
		misc = append(misc, r)
	}
	sort.Slice(misc, func(i, j int) bool {
		if misc[i].sortTime != misc[j].sortTime {
			return misc[i].sortTime < misc[j].sortTime
		}
		return misc[i].hash < misc[j].hash
	})
	for _, r := range misc {
		outLines = append(outLines, r.canon)
	}

	res.TotalLines = len(outLines)
	if len(outLines) == 0 {
		return nil, res, nil
	}
	var buf bytes.Buffer
	for _, line := range outLines {
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), res, nil
}

func parsePantryLines(data []byte, side string) ([]parsedPantryRecord, error) {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	var out []parsedPantryRecord
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		var rawObj any
		if err := json.Unmarshal([]byte(line), &rawObj); err != nil {
			return nil, fmt.Errorf("pantry.jsonl (%s) line %d invalid JSON: %w", side, lineNo, err)
		}
		canonBytes, err := json.Marshal(rawObj)
		if err != nil {
			return nil, fmt.Errorf("pantry.jsonl (%s) line %d canonicalize: %w", side, lineNo, err)
		}
		var p pantryProbe
		_ = json.Unmarshal(canonBytes, &p)
		txID := strings.TrimSpace(p.TxID)
		if txID == "" {
			txID = strings.TrimSpace(p.TxIDCamel)
		}
		kind := strings.ToLower(strings.TrimSpace(p.Type))
		sortTime := firstNonEmpty(p.Timestamp, p.CreatedAt, p.UpdatedAt, p.DateYMD)
		rec := parsedPantryRecord{
			raw:       line,
			canon:     string(canonBytes),
			hash:      SHA256Bytes(canonBytes),
			txID:      txID,
			kind:      kind,
			entityID:  strings.TrimSpace(p.ID),
			revision:  p.Revision,
			updatedAt: strings.TrimSpace(p.UpdatedAt),
			sortTime:  strings.TrimSpace(sortTime),
			isSchema:  kind == "schema",
			schemaVer: p.Version,
		}
		out = append(out, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func isSnapshotEntityKind(kind string) bool {
	switch kind {
	case "box", "reagent", "meta", "vivarium":
		return true
	default:
		return false
	}
}

func entityKindOrder(kind string) int {
	switch kind {
	case "box":
		return 1
	case "reagent":
		return 2
	case "meta":
		return 3
	case "vivarium":
		return 4
	default:
		return 9
	}
}

func shouldReplaceEntityRecord(cur, cand parsedPantryRecord) bool {
	if cand.hash == cur.hash {
		return false
	}
	if cand.revision != cur.revision {
		return cand.revision > cur.revision
	}
	if cand.updatedAt != cur.updatedAt {
		return cand.updatedAt > cur.updatedAt
	}
	return cand.hash > cur.hash
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
