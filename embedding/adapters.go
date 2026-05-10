package embedding

import "sort"

// DocumentsFromMap converts an ID-to-text map to sorted documents.
func DocumentsFromMap(textByID map[string]string) []Document {
	ids := make([]string, 0, len(textByID))
	for id := range textByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	docs := make([]Document, 0, len(ids))
	for _, id := range ids {
		docs = append(docs, Document{ID: id, Text: textByID[id]})
	}
	return docs
}

// DocumentsFromRecords converts records to documents while preserving metadata.
func DocumentsFromRecords(records []Record) []Document {
	docs := make([]Document, len(records))
	for i, record := range records {
		docs[i] = Document{
			ID:       record.ID,
			Text:     record.Text,
			Metadata: cloneMetadata(record.Metadata),
		}
	}
	return docs
}

// Record is a lightweight adapter shape for downstream text records.
type Record struct {
	ID       string
	Text     string
	Metadata map[string]string
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return nil
	}
	clone := make(map[string]string, len(metadata))
	for key, value := range metadata {
		clone[key] = value
	}
	return clone
}
