package curriculum

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestNoDependencies asserts the empty require set the go.mod comment block
// promises. A convenience dependency added here is inherited by every consumer.
func TestNoDependencies(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(b), "\n") {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "require") {
			t.Errorf("go.mod:%d introduces a dependency: %q", i+1, s)
		}
	}
}

// The module must not learn a consuming application's vocabulary. This is a
// text check rather than a compiler one, because the compiler cannot see a
// field NAME that leaks a consumer's domain.
func TestNoConsumerShapedVocabulary(t *testing.T) {
	// Terms belonging to the applications this module was extracted for. None
	// may appear in its source.
	//
	// They are spelled in halves so that THIS file is scanned like every other
	// one. Skipping the file that holds the list would have been the easy fix
	// and would have created the blind spot the list exists to close.
	banned := []string{
		"inter" + "view", "candi" + "date", "recru" + "it",
		"work" + "shop", "hir" + "ing", "empl" + "oyer",
	}
	var offenders []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		low := strings.ToLower(string(b))
		for _, term := range banned {
			if strings.Contains(low, term) {
				offenders = append(offenders, path+": "+term)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("consumer-shaped vocabulary in module source: %v", offenders)
	}
}

// The fixtures shipped with this PUBLIC module must be readable, parseable and
// clean. A fixture that no longer parses would quietly stop testing anything.
func TestShippedGoodFixturesValidateClean(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "good", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no good fixtures found — this test would otherwise pass having checked nothing")
	}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			f, err := os.Open(p)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			doc, err := DecodeDocument(f)
			if err != nil {
				t.Fatal(err)
			}
			rep := doc.Validate()
			if rep.Verdict() != 0 {
				t.Fatalf("verdict = %d; findings %v", rep.Verdict(), rep.Findings)
			}
		})
	}
}

func TestEveryMutationFixtureIsCaught(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "mutations", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no mutation fixtures found — a proof that exercises nothing proves nothing")
	}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			expect, err := os.ReadFile(strings.TrimSuffix(p, ".json") + ".expect")
			if err != nil {
				t.Fatalf("mutation has no .expect sibling: %v", err)
			}
			wantRC, wantCode := parseExpect(string(expect))

			f, err := os.Open(p)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			doc, derr := DecodeDocument(f)
			if derr != nil {
				if wantRC != 2 || wantCode != "parse" {
					t.Fatalf("fixture did not parse but expected rc=%d code=%s: %v", wantRC, wantCode, derr)
				}
				return
			}
			if wantCode == "parse" {
				t.Fatal("fixture was expected not to parse, but it did")
			}
			rep := doc.Validate()
			if rep.Verdict() != wantRC {
				t.Fatalf("verdict = %d, want %d; findings %v", rep.Verdict(), wantRC, rep.Findings)
			}
			if !hasCode(rep, wantCode) {
				t.Fatalf("want code %s, got %v", wantCode, rep.Findings)
			}
		})
	}
}

func parseExpect(s string) (rc int, code string) {
	for _, line := range strings.Split(s, "\n") {
		switch {
		case strings.HasPrefix(line, "rc="):
			if strings.TrimPrefix(line, "rc=") == "1" {
				rc = 1
			} else {
				rc = 2
			}
		case strings.HasPrefix(line, "code="):
			code = strings.TrimPrefix(line, "code=")
		}
	}
	return
}

// A misspelled key in a hand-authored content file is silently dropped by the
// default decoder, taking the material it was carrying with it.
func TestDecodeRejectsUnknownFields(t *testing.T) {
	_, err := DecodeDocument(strings.NewReader(`{"catalog":{"id":"c","aeras":[]}}`))
	if err == nil {
		t.Fatal("a misspelled key was accepted")
	}
}

// An absent chapter list and an empty one are the same JSON. Without
// ChaptersDeclared, "the consumer has no video chapters" would be read as "the
// consumer said nothing" and every clean catalog would come back undetermined.
func TestDeclaredButEmptyChapterListIsEvidence(t *testing.T) {
	d := Document{ChaptersDeclared: true}
	if d.Options().KnownChapters == nil {
		t.Fatal("a declared-but-empty registry was treated as no registry at all")
	}
	if (Document{}).Options().KnownChapters != nil {
		t.Fatal("an undeclared registry was treated as evidence")
	}
}

// The wire form is what a frontend consumes. correctChoices carries IDs, and
// the field that used to be an index-with-omitempty is gone: assert the shape.
func TestQuestionWireFormNamesAnswersByID(t *testing.T) {
	q := Question{ID: "q", Kind: KindSingle, Points: 1, Prompt: "p",
		Choices:        []Choice{{ID: "first", Text: "a"}, {ID: "second", Text: "b"}},
		CorrectChoices: []ID{"first"}}
	b, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"correctChoices":["first"]`) {
		t.Fatalf("wire form = %s", s)
	}
	if strings.Contains(s, "correctIndex") {
		t.Fatalf("an index-based answer field reappeared: %s", s)
	}
}

func TestVideoAnchorTypedAccessors(t *testing.T) {
	v := VideoAnchor{StartMillis: 1500, EndMillis: 4500}
	if v.Start() != 1500*time.Millisecond || v.End() != 4500*time.Millisecond {
		t.Fatalf("start %v end %v", v.Start(), v.End())
	}
	if v.Length() != 3*time.Second {
		t.Fatalf("length = %v, want 3s", v.Length())
	}
}

// Milliseconds, not time.Duration: a Duration marshals as an opaque nanosecond
// integer no other language's client reads correctly.
func TestVideoAnchorWireFormIsMilliseconds(t *testing.T) {
	b, err := json.Marshal(VideoAnchor{ChapterID: "ch", StartMillis: 1500, EndMillis: 4500, TranscriptAnchor: "seg-1"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"chapterId":"ch","startMillis":1500,"endMillis":4500,"transcriptAnchor":"seg-1"}`
	if string(b) != want {
		t.Fatalf("wire form = %s, want %s", b, want)
	}
}

// A LESSON'S TEACHING CONTENT SURVIVES THE ROUND TRIP, AND IS NOT ITS SUMMARY.
//
// The defect this pins is not a crash. A model with a Summary and no Body reads
// as complete — every lesson has a title, an estimate and links — while the text
// the lesson exists to deliver has nowhere to live, so an authoring pipeline
// that HAS that text drops it with nothing anywhere saying so. That is exactly
// how it was lost: the material was parsed, measured for a reading estimate, and
// then discarded because there was no field to put it in.
func TestLessonBodyIsCarriedAndIsDistinctFromSummary(t *testing.T) {
	const body = "Two paragraphs of teaching text.\n\nThe second one."
	in := Document{Catalog: Catalog{
		ID: "c", Title: "T", Version: "1",
		Areas: []Area{{ID: "a", Title: "A", Lessons: []Lesson{{
			ID: "l", AreaID: "a", Title: "L", Summary: "one line", Body: body,
		}}}},
	}}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"body"`) {
		t.Fatalf("Lesson.Body did not reach the wire: %s", b)
	}
	out, err := DecodeDocument(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	got := out.Catalog.Areas[0].Lessons[0]
	if got.Body != body {
		t.Fatalf("body did not round-trip: got %q want %q", got.Body, body)
	}
	// The two fields are independent. If Body were merely an alias for Summary,
	// this assertion would pass by accident; it is here so that folding them
	// together later fails loudly instead of silently truncating every lesson to
	// its blurb.
	if got.Summary == got.Body {
		t.Fatalf("summary and body collapsed into one value: %q", got.Summary)
	}

	// A lesson carrying only a summary has an EMPTY body, not a copy of it.
	// This is the state the consumer's own gate exists to catch, so the model
	// must represent it faithfully rather than papering over it.
	only, err := DecodeDocument(strings.NewReader(
		`{"catalog":{"id":"c","title":"T","version":"1","areas":[{"id":"a","title":"A",` +
			`"lessons":[{"id":"l","areaId":"a","title":"L","summary":"one line","ord":0}],` +
			`"assessment":null}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := only.Catalog.Areas[0].Lessons[0].Body; got != "" {
		t.Fatalf("absent body decoded as %q, not empty", got)
	}
}

// Validate deliberately does NOT report an empty body — requiring one is a
// consumer's policy, not this package's (see Lesson.Body). This test states that
// choice so that adding such a rule later has to change a test that explains
// why it was absent, rather than looking like an oversight being corrected.
func TestValidateDoesNotRequireALessonBody(t *testing.T) {
	c := Catalog{ID: "c", Title: "T", Version: "1", Areas: []Area{{
		ID: "a", Title: "A",
		Lessons: []Lesson{{ID: "l", AreaID: "a", Title: "L", Ord: 0}},
		Assessment: &Assessment{
			ID: "as", AreaID: "a", Title: "Test", RequiredLessons: []ID{"l"}, PassPercent: 50,
			Questions: []Question{{
				ID: "q", Kind: KindSingle, Prompt: "P", Points: 1,
				Choices:        []Choice{{ID: "c1", Text: "x"}, {ID: "c2", Text: "y"}},
				CorrectChoices: []ID{"c1"},
			}},
		},
	}}}
	for _, f := range ValidateWith(c, Options{KnownChapters: map[ID]bool{}}).Findings {
		if strings.Contains(strings.ToLower(f.Message), "body") {
			t.Fatalf("Validate reported a body rule it does not own: %v", f)
		}
	}
}
