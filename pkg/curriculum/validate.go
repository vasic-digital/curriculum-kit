package curriculum

import (
	"fmt"
	"sort"
)

// Severity separates "this catalog is wrong" from "this catalog could not be
// judged". The two are never folded together: an UNDETERMINED row is not a
// pass, and it is not a finding either.
type Severity string

const (
	// SeverityFinding is a real defect in the catalog.
	SeverityFinding Severity = "finding"
	// SeverityUndetermined is a question this package could not answer with the
	// evidence it was given.
	SeverityUndetermined Severity = "undetermined"
)

// Finding is one row of a validation report.
type Finding struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	// Path locates the row in the catalog, e.g. "areas[0].lessons[2].materials[1]".
	Path    string `json:"path"`
	Message string `json:"message"`
}

func (f Finding) String() string {
	return fmt.Sprintf("%s %s %s — %s", f.Severity, f.Code, f.Path, f.Message)
}

// Report is the whole result of a validation pass.
type Report struct {
	Findings []Finding `json:"findings"`
}

// Findings returns only the SeverityFinding rows.
func (r Report) FindingRows() []Finding { return r.filter(SeverityFinding) }

// Undetermined returns only the SeverityUndetermined rows.
func (r Report) Undetermined() []Finding { return r.filter(SeverityUndetermined) }

func (r Report) filter(s Severity) []Finding {
	out := []Finding{}
	for _, f := range r.Findings {
		if f.Severity == s {
			out = append(out, f)
		}
	}
	return out
}

// Verdict collapses the report to a three-valued exit code:
//
//	0  the catalog is coherent and everything asked was answerable
//	1  at least one real finding
//	2  no findings, but at least one question could not be answered
//
// Precedence is stated and asserted by a test: a finding OUTRANKS an
// undetermined row, so a catalog that is measurably broken can never hide
// behind "could not determine". A 2 is never a pass.
func (r Report) Verdict() int {
	if len(r.FindingRows()) > 0 {
		return 1
	}
	if len(r.Undetermined()) > 0 {
		return 2
	}
	return 0
}

// OK reports whether the verdict is 0. It exists so callers stop writing
// `err == nil`-shaped checks against a three-valued result.
func (r Report) OK() bool { return r.Verdict() == 0 }

// Options tunes what Validate is able to decide.
type Options struct {
	// KnownChapters is the set of video chapter ids the CONSUMER can resolve.
	//
	// This package does not model video files, so it cannot know whether a
	// VideoAnchor points at a chapter that exists. If this map is nil, every
	// video material produces an UNDETERMINED row rather than a silent pass —
	// absence of evidence is reported as absence of evidence. If it is supplied,
	// an anchor naming a chapter outside it is a real finding.
	KnownChapters map[ID]bool
}

// Rule codes. Kept as constants so a consumer can suppress or test one by name
// rather than by matching prose.
const (
	CodeEmptyCatalog       = "CK001" // the catalog holds no areas at all
	CodeDuplicateID        = "CK002"
	CodeAreaNoLessons      = "CK003"
	CodeLessonWrongArea    = "CK004"
	CodeAssessWrongArea    = "CK005"
	CodeAssessNoRequired   = "CK006"
	CodeRequiredNotInArea  = "CK007"
	CodeAssessNoQuestions  = "CK008"
	CodePassPercentRange   = "CK009"
	CodeVideoNoRange       = "CK010"
	CodeUnknownMaterial    = "CK011"
	CodeMaterialNoURI      = "CK012"
	CodeVideoOnNonVideo    = "CK013"
	CodeTooFewChoices      = "CK014"
	CodeCorrectNotAChoice  = "CK015"
	CodeSingleNotOne       = "CK016"
	CodeMultiNoCorrect     = "CK017"
	CodeShortHasChoices    = "CK018"
	CodeEmptyText          = "CK019"
	CodeUnknownQuestion    = "CK020"
	CodeAreaNoAssessment   = "CK021"
	CodeNonPositivePoints  = "CK022"
	CodeMissingAlt         = "CK023"
	CodeUnknownChapter     = "CK024"
	CodeChoiceHasAnswer    = "CK025"
	CodeChaptersUnresolved = "CK900" // undetermined, not a finding
)

// Validate checks a catalog with no external evidence. Every video anchor is
// therefore UNDETERMINED; use ValidateWith to supply the chapter registry.
func Validate(c Catalog) Report { return ValidateWith(c, Options{}) }

// ValidateWith checks a catalog against the evidence in opts.
func ValidateWith(c Catalog, opts Options) Report {
	v := &validator{opts: opts, seen: map[ID]string{}}

	// The non-vacuity rule, and it is first on purpose. A validator that loops
	// over a collection and reports "no problems" when the collection is empty
	// has measured nothing and said it was fine.
	if len(c.Areas) == 0 {
		v.find(CodeEmptyCatalog, "catalog",
			"catalog holds no areas — an empty content set is not a valid catalog, and reporting it as one would mean the check iterated nothing")
	}

	v.id(c.ID, "catalog", "catalog")
	v.text(c.Title, CodeEmptyText, "catalog", "catalog title is empty")

	for i, a := range c.Areas {
		v.area(a, fmt.Sprintf("areas[%d]", i))
	}

	sort.SliceStable(v.out, func(i, j int) bool { return v.out[i].Path < v.out[j].Path })
	return Report{Findings: v.out}
}

type validator struct {
	opts Options
	out  []Finding
	// seen maps every id in the catalog to the path that first claimed it. Ids
	// are unique catalog-wide, not merely within their own kind: a consumer that
	// flattens the tree into one table — most of them do — needs them to be.
	seen map[ID]string
}

func (v *validator) add(sev Severity, code, path, msg string) {
	v.out = append(v.out, Finding{Code: code, Severity: sev, Path: path, Message: msg})
}

func (v *validator) find(code, path, msg string) { v.add(SeverityFinding, code, path, msg) }

func (v *validator) undet(code, path, msg string) {
	v.add(SeverityUndetermined, code, path, msg)
}

func (v *validator) id(id ID, path, what string) {
	if id == "" {
		v.find(CodeEmptyText, path, what+" has an empty id")
		return
	}
	if prev, dup := v.seen[id]; dup {
		v.find(CodeDuplicateID, path, fmt.Sprintf("id %q is already used by %s — ids are unique catalog-wide", id, prev))
		return
	}
	v.seen[id] = path
}

func (v *validator) text(s, code, path, msg string) {
	if s == "" {
		v.find(code, path, msg)
	}
}

func (v *validator) area(a Area, path string) {
	v.id(a.ID, path, "area")
	v.text(a.Title, CodeEmptyText, path, "area title is empty")

	if len(a.Lessons) == 0 {
		v.find(CodeAreaNoLessons, path, "area has no lessons — an area with nothing to study cannot lead to a test")
	}
	for i, l := range a.Lessons {
		v.lesson(a, l, fmt.Sprintf("%s.lessons[%d]", path, i))
	}

	if a.Assessment == nil {
		v.find(CodeAreaNoAssessment, path,
			"area has no assessment — every area must end in a test its lessons lead to")
		return
	}
	v.assessment(a, *a.Assessment, path+".assessment")
}

func (v *validator) lesson(a Area, l Lesson, path string) {
	v.id(l.ID, path, "lesson")
	v.text(l.Title, CodeEmptyText, path, "lesson title is empty")
	if l.AreaID != a.ID {
		v.find(CodeLessonWrongArea, path,
			fmt.Sprintf("lesson claims area %q but is filed under area %q", l.AreaID, a.ID))
	}
	for i, m := range l.Materials {
		v.material(m, fmt.Sprintf("%s.materials[%d]", path, i))
	}
}

func (v *validator) material(m Material, path string) {
	v.id(m.ID, path, "material")
	v.text(m.Title, CodeEmptyText, path, "material title is empty")

	if !ValidMaterialKind(m.Kind) {
		v.find(CodeUnknownMaterial, path,
			fmt.Sprintf("material kind %q is not one of %v", m.Kind, MaterialKinds))
	}
	if m.URI == "" {
		v.find(CodeMaterialNoURI, path, "material has no uri — nothing can be shown for it")
	}
	if m.Kind.IsVisual() && m.Alt == "" {
		v.find(CodeMissingAlt, path,
			fmt.Sprintf("material of kind %q has no alt text — readers who cannot see it receive nothing", m.Kind))
	}

	if m.Kind != KindVideo {
		if m.Video != nil {
			v.find(CodeVideoOnNonVideo, path,
				fmt.Sprintf("material of kind %q carries a video anchor", m.Kind))
		}
		return
	}

	// KindVideo from here down.
	if m.Video == nil {
		v.find(CodeVideoNoRange, path,
			"video material has no time range — a consumer cannot seek into a chapter or scroll a transcript without one")
		return
	}
	an := *m.Video
	if an.ChapterID == "" {
		v.find(CodeVideoNoRange, path, "video anchor names no chapter")
	}
	if an.StartMillis < 0 {
		v.find(CodeVideoNoRange, path, fmt.Sprintf("video anchor starts at %dms, before the chapter begins", an.StartMillis))
	}
	if an.EndMillis <= an.StartMillis {
		v.find(CodeVideoNoRange, path,
			fmt.Sprintf("video anchor end %dms does not exceed start %dms — a zero-length range is a position, not a range", an.EndMillis, an.StartMillis))
	}

	switch {
	case v.opts.KnownChapters == nil:
		v.undet(CodeChaptersUnresolved, path,
			fmt.Sprintf("no chapter registry was supplied, so whether chapter %q exists could not be determined — this is not a pass", an.ChapterID))
	case !v.opts.KnownChapters[an.ChapterID]:
		v.find(CodeUnknownChapter, path,
			fmt.Sprintf("video anchor names chapter %q, which is not in the supplied registry", an.ChapterID))
	}
}

func (v *validator) assessment(a Area, as Assessment, path string) {
	v.id(as.ID, path, "assessment")
	v.text(as.Title, CodeEmptyText, path, "assessment title is empty")

	if as.AreaID != a.ID {
		v.find(CodeAssessWrongArea, path,
			fmt.Sprintf("assessment claims area %q but is filed under area %q", as.AreaID, a.ID))
	}
	if len(as.RequiredLessons) == 0 {
		v.find(CodeAssessNoRequired, path,
			"assessment requires no lessons — a test gated on nothing is available before any studying and is not an END-of-area test")
	}
	for i, id := range as.RequiredLessons {
		if _, ok := a.Lesson(id); !ok {
			v.find(CodeRequiredNotInArea, fmt.Sprintf("%s.requiredLessons[%d]", path, i),
				fmt.Sprintf("required lesson %q is not a lesson of area %q", id, a.ID))
		}
	}
	if as.PassPercent < 1 || as.PassPercent > 100 {
		v.find(CodePassPercentRange, path,
			fmt.Sprintf("passPercent %d is outside 1..100 — 0 would pass every attempt including an empty one", as.PassPercent))
	}
	if len(as.Questions) == 0 {
		v.find(CodeAssessNoQuestions, path,
			"assessment has no questions — it would score 0 of 0 and could never be answered")
	}
	for i, q := range as.Questions {
		v.question(q, fmt.Sprintf("%s.questions[%d]", path, i))
	}
}

func (v *validator) question(q Question, path string) {
	v.id(q.ID, path, "question")
	v.text(q.Prompt, CodeEmptyText, path, "question prompt is empty")

	if q.Points <= 0 {
		v.find(CodeNonPositivePoints, path,
			fmt.Sprintf("question is worth %d points — a non-positive weight cannot move a score", q.Points))
	}
	if !ValidQuestionKind(q.Kind) {
		v.find(CodeUnknownQuestion, path,
			fmt.Sprintf("question kind %q is not one of %v", q.Kind, QuestionKinds))
		return
	}

	if q.Kind == KindShort {
		if len(q.Choices) > 0 || len(q.CorrectChoices) > 0 {
			v.find(CodeShortHasChoices, path,
				"free-text question carries choices — it would be presented as a picker and graded as one")
		}
		return
	}

	// Answer is the free-text model answer. On a choice question it is a
	// category error with a real cost: Grade carries Answer into the outcome
	// unconditionally, so prose written for a question that HAS a key ships
	// beside that key and is liable to restate it. Explanation is the channel
	// for a choice question; this one is not.
	if q.Answer != "" {
		v.find(CodeChoiceHasAnswer, path,
			fmt.Sprintf("question of kind %q carries a free-text model answer — that channel is %q's; use explanation here", q.Kind, KindShort))
	}

	if len(q.Choices) < 2 {
		v.find(CodeTooFewChoices, path,
			fmt.Sprintf("question has %d choice(s); a choice question needs at least 2", len(q.Choices)))
	}
	have := map[ID]bool{}
	for i, c := range q.Choices {
		v.id(c.ID, fmt.Sprintf("%s.choices[%d]", path, i), "choice")
		v.text(c.Text, CodeEmptyText, fmt.Sprintf("%s.choices[%d]", path, i), "choice text is empty")
		have[c.ID] = true
	}
	for i, id := range q.CorrectChoices {
		if !have[id] {
			v.find(CodeCorrectNotAChoice, fmt.Sprintf("%s.correctChoices[%d]", path, i),
				fmt.Sprintf("correct choice %q is not one of this question's choices", id))
		}
	}
	switch q.Kind {
	case KindSingle:
		if len(q.CorrectChoices) != 1 {
			v.find(CodeSingleNotOne, path,
				fmt.Sprintf("single-answer question names %d correct choices; it must name exactly 1", len(q.CorrectChoices)))
		}
	case KindMulti:
		if len(q.CorrectChoices) == 0 {
			v.find(CodeMultiNoCorrect, path, "multi-answer question names no correct choice — it is unanswerable")
		}
	}
}
