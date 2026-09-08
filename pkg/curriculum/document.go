package curriculum

import (
	"encoding/json"
	"fmt"
	"io"
)

// Document is the on-disk / on-the-wire form: a catalog plus the evidence a
// validator needs in order to reach a determinate verdict about it.
//
// It is a separate type from Catalog because KnownChapters is NOT content. It
// is a statement about the consuming application's video library, supplied so
// that this package can decide something it otherwise could only report as
// undetermined.
type Document struct {
	Catalog Catalog `json:"catalog"`
	// KnownChapters is the consumer's video chapter registry. When it is absent,
	// every video anchor is UNDETERMINED — see Options.KnownChapters.
	KnownChapters []ID `json:"knownChapters,omitempty"`
	// ChaptersDeclared distinguishes "the consumer has no video chapters" from
	// "the consumer did not tell us". Without it, an empty list and an absent
	// list are the same JSON, and the first would silently become the second.
	ChaptersDeclared bool `json:"chaptersDeclared,omitempty"`
}

// Options builds the validator options this document implies.
func (d Document) Options() Options {
	if !d.ChaptersDeclared && len(d.KnownChapters) == 0 {
		return Options{}
	}
	set := map[ID]bool{}
	for _, id := range d.KnownChapters {
		set[id] = true
	}
	return Options{KnownChapters: set}
}

// Validate checks the document's catalog with the document's own evidence.
func (d Document) Validate() Report { return ValidateWith(d.Catalog, d.Options()) }

// DecodeDocument reads a Document. Unknown fields are rejected: in a hand-
// authored content file a misspelled key is silently dropped by the default
// decoder, and the material it was carrying disappears with no error anywhere.
func DecodeDocument(r io.Reader) (Document, error) {
	var d Document
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return Document{}, fmt.Errorf("decoding curriculum document: %w", err)
	}
	return d, nil
}
