package curriculum

import (
	"strings"
	"testing"
)

// goodCatalog is synthetic and invented for this repository. No fixture here
// derives from any private corpus, recording, transcript or real person.
func goodCatalog() Catalog {
	return Catalog{
		ID: "cat", Title: "Knots", Version: "1.0.0",
		Areas: []Area{{
			ID: "area-1", Title: "Stopper knots", Ord: 1,
			Lessons: []Lesson{
				{ID: "les-1", AreaID: "area-1", Title: "The overhand", Ord: 1, Materials: []Material{
					{ID: "mat-1", Kind: KindDiagram, Title: "Overhand, step by step", URI: "m/1.svg", Alt: "Three drawings of a rope being tied."},
					{ID: "mat-2", Kind: KindVideo, Title: "Tied slowly", URI: "v/ch1", Video: &VideoAnchor{
						ChapterID: "ch1", StartMillis: 5000, EndMillis: 20000, TranscriptAnchor: "ch1-0003"}},
				}},
				{ID: "les-2", AreaID: "area-1", Title: "The figure eight", Ord: 2},
			},
			Assessment: &Assessment{
				ID: "asm-1", AreaID: "area-1", Title: "Stopper knots test",
				RequiredLessons: []ID{"les-1", "les-2"}, PassPercent: 60,
				Questions: []Question{
					{ID: "q-1", Kind: KindSingle, Prompt: "Which knot resists jamming better?", Points: 2,
						Choices:        []Choice{{ID: "c-1", Text: "The figure eight"}, {ID: "c-2", Text: "The overhand"}},
						CorrectChoices: []ID{"c-1"}, Explanation: "Its extra turn spreads the load."},
					{ID: "q-2", Kind: KindMulti, Prompt: "Which are stopper knots?", Points: 3,
						Choices:        []Choice{{ID: "c-3", Text: "Overhand"}, {ID: "c-4", Text: "Figure eight"}, {ID: "c-5", Text: "Clove hitch"}},
						CorrectChoices: []ID{"c-3", "c-4"}},
				},
			},
		}},
	}
}

func chapters(ids ...ID) Options {
	m := map[ID]bool{}
	for _, id := range ids {
		m[id] = true
	}
	return Options{KnownChapters: m}
}

func TestGoodCatalogValidatesClean(t *testing.T) {
	rep := ValidateWith(goodCatalog(), chapters("ch1"))
	if v := rep.Verdict(); v != 0 {
		t.Fatalf("verdict = %d, want 0; findings: %v", v, rep.Findings)
	}
	if !rep.OK() {
		t.Fatal("OK() = false on a clean report")
	}
}

// The non-vacuity control. This exact defect class — a check that iterates an
// empty collection and calls the silence a pass — has shipped in sibling
// repositories, so it is asserted here rather than assumed.
func TestEmptyCatalogIsNotAPass(t *testing.T) {
	rep := Validate(Catalog{ID: "c", Title: "t"})
	if rep.Verdict() != 1 {
		t.Fatalf("verdict = %d, want 1 for a catalog with no areas", rep.Verdict())
	}
	if !hasCode(rep, CodeEmptyCatalog) {
		t.Fatalf("want %s, got %v", CodeEmptyCatalog, rep.Findings)
	}
}

func TestZeroValueCatalogIsNotAPass(t *testing.T) {
	if rep := Validate(Catalog{}); rep.Verdict() == 0 {
		t.Fatal("the zero-value Catalog validated clean; an empty content set must never pass")
	}
}

func TestMissingChapterRegistryIsUndeterminedNotAPass(t *testing.T) {
	rep := Validate(goodCatalog()) // no Options — no chapter evidence
	if rep.Verdict() != 2 {
		t.Fatalf("verdict = %d, want 2 when no chapter registry is supplied", rep.Verdict())
	}
	if len(rep.FindingRows()) != 0 {
		t.Fatalf("undetermined rows must not be reported as findings: %v", rep.FindingRows())
	}
	if !hasCode(rep, CodeChaptersUnresolved) {
		t.Fatalf("want %s, got %v", CodeChaptersUnresolved, rep.Findings)
	}
}

func TestFindingOutranksUndetermined(t *testing.T) {
	c := goodCatalog()
	c.Areas[0].Lessons[0].AreaID = "not-this-area" // a real finding
	rep := Validate(c)                             // and no chapter registry — an undetermined row too
	if len(rep.Undetermined()) == 0 {
		t.Fatal("expected the report to also hold an undetermined row")
	}
	if rep.Verdict() != 1 {
		t.Fatalf("verdict = %d, want 1; a measurable finding must not hide behind an undetermined row", rep.Verdict())
	}
}

func TestUnknownChapterIsAFindingWhenTheRegistryIsSupplied(t *testing.T) {
	rep := ValidateWith(goodCatalog(), chapters("some-other-chapter"))
	if rep.Verdict() != 1 || !hasCode(rep, CodeUnknownChapter) {
		t.Fatalf("verdict = %d, findings %v; want 1 with %s", rep.Verdict(), rep.Findings, CodeUnknownChapter)
	}
}

// Table over every rule the validator claims to enforce. Each row mutates the
// good catalog in exactly one way and names the code that must come back.
func TestEachRuleCatchesItsOwnDefect(t *testing.T) {
	cases := []struct {
		name string
		code string
		mut  func(*Catalog)
	}{
		{"area with no lessons", CodeAreaNoLessons, func(c *Catalog) { c.Areas[0].Lessons = nil }},
		{"lesson filed under a missing area", CodeLessonWrongArea, func(c *Catalog) { c.Areas[0].Lessons[0].AreaID = "gone" }},
		{"assessment filed under a missing area", CodeAssessWrongArea, func(c *Catalog) { c.Areas[0].Assessment.AreaID = "gone" }},
		{"test gated on nothing", CodeAssessNoRequired, func(c *Catalog) { c.Areas[0].Assessment.RequiredLessons = nil }},
		{"gate names a foreign lesson", CodeRequiredNotInArea, func(c *Catalog) {
			c.Areas[0].Assessment.RequiredLessons = []ID{"les-elsewhere"}
		}},
		{"test with no questions", CodeAssessNoQuestions, func(c *Catalog) { c.Areas[0].Assessment.Questions = nil }},
		{"threshold of zero", CodePassPercentRange, func(c *Catalog) { c.Areas[0].Assessment.PassPercent = 0 }},
		{"threshold above 100", CodePassPercentRange, func(c *Catalog) { c.Areas[0].Assessment.PassPercent = 101 }},
		{"video material with no anchor", CodeVideoNoRange, func(c *Catalog) { c.Areas[0].Lessons[0].Materials[1].Video = nil }},
		{"anchor of zero length", CodeVideoNoRange, func(c *Catalog) {
			c.Areas[0].Lessons[0].Materials[1].Video.EndMillis = c.Areas[0].Lessons[0].Materials[1].Video.StartMillis
		}},
		{"anchor that runs backwards", CodeVideoNoRange, func(c *Catalog) {
			c.Areas[0].Lessons[0].Materials[1].Video.EndMillis = 1
		}},
		{"anchor with no chapter", CodeVideoNoRange, func(c *Catalog) {
			c.Areas[0].Lessons[0].Materials[1].Video.ChapterID = ""
		}},
		{"unknown material kind", CodeUnknownMaterial, func(c *Catalog) { c.Areas[0].Lessons[0].Materials[0].Kind = "moodboard" }},
		{"material with no uri", CodeMaterialNoURI, func(c *Catalog) { c.Areas[0].Lessons[0].Materials[0].URI = "" }},
		{"visual material with no alt", CodeMissingAlt, func(c *Catalog) { c.Areas[0].Lessons[0].Materials[0].Alt = "" }},
		{"video anchor on a diagram", CodeVideoOnNonVideo, func(c *Catalog) {
			c.Areas[0].Lessons[0].Materials[0].Video = &VideoAnchor{ChapterID: "ch1", EndMillis: 10}
		}},
		{"choice question with one choice", CodeTooFewChoices, func(c *Catalog) {
			c.Areas[0].Assessment.Questions[0].Choices = c.Areas[0].Assessment.Questions[0].Choices[:1]
			c.Areas[0].Assessment.Questions[0].CorrectChoices = []ID{"c-1"}
		}},
		{"answer that is not a choice", CodeCorrectNotAChoice, func(c *Catalog) {
			c.Areas[0].Assessment.Questions[0].CorrectChoices = []ID{"c-99"}
		}},
		{"single-answer question with two answers", CodeSingleNotOne, func(c *Catalog) {
			c.Areas[0].Assessment.Questions[0].CorrectChoices = []ID{"c-1", "c-2"}
		}},
		{"multi question with no answer", CodeMultiNoCorrect, func(c *Catalog) {
			c.Areas[0].Assessment.Questions[1].CorrectChoices = nil
		}},
		{"free text carrying choices", CodeShortHasChoices, func(c *Catalog) {
			c.Areas[0].Assessment.Questions[0].Kind = KindShort
		}},
		{"unknown question kind", CodeUnknownQuestion, func(c *Catalog) { c.Areas[0].Assessment.Questions[0].Kind = "essay" }},
		{"question worth nothing", CodeNonPositivePoints, func(c *Catalog) { c.Areas[0].Assessment.Questions[0].Points = 0 }},
		{"duplicate id", CodeDuplicateID, func(c *Catalog) { c.Areas[0].Lessons[1].ID = "les-1" }},
		{"area with no test", CodeAreaNoAssessment, func(c *Catalog) { c.Areas[0].Assessment = nil }},
		{"untitled area", CodeEmptyText, func(c *Catalog) { c.Areas[0].Title = "" }},
	}

	if len(cases) == 0 {
		t.Fatal("the mutation table is empty — this test would pass having asserted nothing")
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := goodCatalog()
			tc.mut(&c)
			rep := ValidateWith(c, chapters("ch1"))
			if rep.Verdict() != 1 {
				t.Fatalf("verdict = %d, want 1; findings %v", rep.Verdict(), rep.Findings)
			}
			if !hasCode(rep, tc.code) {
				t.Fatalf("want code %s, got %v", tc.code, rep.Findings)
			}
		})
	}
}

func TestFindingPathLocatesTheRow(t *testing.T) {
	c := goodCatalog()
	c.Areas[0].Lessons[0].Materials[1].Video = nil
	rep := ValidateWith(c, chapters("ch1"))
	for _, f := range rep.Findings {
		if f.Code == CodeVideoNoRange {
			if !strings.Contains(f.Path, "areas[0].lessons[0].materials[1]") {
				t.Fatalf("path = %q, want it to locate the material", f.Path)
			}
			return
		}
	}
	t.Fatalf("no %s finding: %v", CodeVideoNoRange, rep.Findings)
}

func hasCode(r Report, code string) bool {
	for _, f := range r.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}
