// Package curriculum is a project-not-aware content model and assessment
// mechanism for structured learning: an AREA leads to LESSONS, each lesson
// carries MATERIALS, and the area ends in an ASSESSMENT that becomes available
// only once the lessons it names are complete.
//
// The package owns the MODEL and the MECHANISM. It owns no storage, no
// transport and no rendering: a consuming application decides how a catalog is
// authored, where progress is persisted, and what a lesson looks like on a
// screen. Every type here is a plain value with JSON tags, so a consumer can
// serialise it with encoding/json and store the bytes wherever it likes.
//
// Nothing in this package may name a concept belonging to a consuming
// application. See the module-local rules in CLAUDE.md.
package curriculum

import "time"

// ID is a caller-assigned stable identifier. This package never mints one: a
// consumer already has an identity scheme (a slug, a row id, a UUID) and a
// second one invented here would have to be mapped back to it.
type ID string

// --- materials -------------------------------------------------------------

// MaterialKind is the closed vocabulary of things that can be attached to a
// lesson. It is closed so that a renderer can switch on it exhaustively and a
// validator can reject a kind nobody will draw.
type MaterialKind string

const (
	// KindIllustration is a decorative or explanatory picture.
	KindIllustration MaterialKind = "illustration"
	// KindDiagram is a drawing of structure — boxes and arrows.
	KindDiagram MaterialKind = "diagram"
	// KindScheme is a drawing of a process or procedure — a sequence.
	KindScheme MaterialKind = "scheme"
	// KindGraph is a plot of data — axes and series.
	KindGraph MaterialKind = "graph"
	// KindVideo is a range inside a video chapter. It is the only kind that
	// carries a VideoAnchor, and it may never be missing one.
	KindVideo MaterialKind = "video"
	// KindDocument is a downloadable or linkable attachment.
	KindDocument MaterialKind = "document"
)

// MaterialKinds is the vocabulary as DATA, so an error can name the accepted
// values instead of describing them.
var MaterialKinds = []MaterialKind{
	KindIllustration, KindDiagram, KindScheme, KindGraph, KindVideo, KindDocument,
}

// ValidMaterialKind reports whether k is one of MaterialKinds.
func ValidMaterialKind(k MaterialKind) bool {
	for _, want := range MaterialKinds {
		if k == want {
			return true
		}
	}
	return false
}

// VideoAnchor points at a time RANGE inside a video chapter, and at the place
// in that chapter's transcript which the range corresponds to.
//
// It carries a range rather than a single position on purpose. A consumer that
// only receives a start cannot highlight the passage, cannot show a duration,
// and cannot tell whether playback has left the material — all three are things
// the range makes possible, and none of them can be recovered later.
//
// Times are integer MILLISECONDS, not time.Duration, because this struct is
// serialised: encoding/json renders a time.Duration as an opaque nanosecond
// integer that no other language's client will read correctly. Start and End
// helpers give Go callers the typed form without putting it on the wire.
type VideoAnchor struct {
	// ChapterID identifies the video chapter in the CONSUMER's own catalogue of
	// videos. This package does not model video files; it refers to them.
	ChapterID ID `json:"chapterId"`
	// StartMillis is the inclusive start of the range, from the chapter start.
	StartMillis int64 `json:"startMillis"`
	// EndMillis is the exclusive end of the range. It must exceed StartMillis:
	// a zero-length anchor is a position wearing a range's clothes.
	EndMillis int64 `json:"endMillis"`
	// TranscriptAnchor is an opaque handle the consumer resolves against its own
	// transcript so a viewer can be scrolled to the passage. It is a string
	// rather than a line number because a transcript is re-segmented whenever it
	// is re-run, and a line number silently rots when it is.
	TranscriptAnchor string `json:"transcriptAnchor,omitempty"`
}

// Start returns the typed start offset.
func (v VideoAnchor) Start() time.Duration { return time.Duration(v.StartMillis) * time.Millisecond }

// End returns the typed end offset.
func (v VideoAnchor) End() time.Duration { return time.Duration(v.EndMillis) * time.Millisecond }

// Length returns End minus Start. It is negative or zero for an anchor that
// Validate rejects; callers should not reach it on a validated catalog.
func (v VideoAnchor) Length() time.Duration { return v.End() - v.Start() }

// Material is one thing attached to a lesson.
type Material struct {
	ID   ID           `json:"id"`
	Kind MaterialKind `json:"kind"`
	// Title is what a reader is offered. It is required: an untitled material is
	// a link a reader cannot decide whether to follow.
	Title string `json:"title"`
	// Caption is the optional explanatory line under the material.
	Caption string `json:"caption,omitempty"`
	// URI is where the material lives, resolved by the consumer. This package
	// does not fetch it and makes no claim about what is behind it.
	URI string `json:"uri"`
	// Alt is the text alternative. Required for the visual kinds, because a
	// picture with no alternative is content that some readers simply do not
	// receive.
	Alt string `json:"alt,omitempty"`
	// Video is set for, and only for, KindVideo.
	Video *VideoAnchor `json:"video,omitempty"`
}

// IsVisual reports whether this kind renders as an image a reader may not be
// able to see, and therefore owes an Alt.
func (k MaterialKind) IsVisual() bool {
	switch k {
	case KindIllustration, KindDiagram, KindScheme, KindGraph:
		return true
	}
	return false
}

// --- lessons ---------------------------------------------------------------

// Lesson is one unit of study inside an area.
type Lesson struct {
	ID ID `json:"id"`
	// AreaID names the lesson's owning area. It is carried on the lesson as well
	// as implied by nesting so that a consumer which flattens the tree into rows
	// — every consumer with a database does — keeps the edge, and so that
	// Validate can catch a lesson filed under an area it does not claim.
	AreaID ID     `json:"areaId"`
	Title  string `json:"title"`
	// Summary is the one-line orientation shown BESIDE the title in a list of
	// lessons. It is not the teaching content and must not be used as it: a
	// renderer that shows a summary where a body belongs shows a learner the
	// blurb for a lesson whose text they can then never reach.
	Summary string `json:"summary,omitempty"`
	// Body is the teaching content — the text a learner actually reads, and the
	// reason the lesson exists.
	//
	// It is a separate field from Summary because the two are answers to
	// different questions ("should I read this?" and "what does it teach?"), and
	// because a model that offers only the first makes the second unexpressible.
	// A catalog whose lessons carry titles, minutes and links but no body is a
	// table of contents presented as a course, and nothing in the type system
	// says so — this field is what makes the difference statable.
	//
	// It is OPAQUE MARKED-UP TEXT. This package neither parses nor renders it,
	// makes no claim about its dialect, and imposes no length: an author decides
	// what a lesson says, and a consumer decides how it is drawn. A body that is
	// genuinely short stays short — padding one to look substantial is a worse
	// defect than a thin lesson, because it is not visible as one.
	//
	// It is OPTIONAL here on purpose. A lesson that is wholly a set of materials
	// — one video range, nothing else — is a legitimate lesson, so requiring a
	// body for every lesson in every catalog is a POLICY a consuming curriculum
	// may adopt and this package may not impose. Validate therefore does not
	// report an empty body; a consumer that requires one enforces it in its own
	// gate, against its own content.
	Body             string     `json:"body,omitempty"`
	Ord              int        `json:"ord"`
	EstimatedMinutes int        `json:"estimatedMinutes,omitempty"`
	Materials        []Material `json:"materials,omitempty"`
}

// --- assessment ------------------------------------------------------------

// QuestionKind is the closed vocabulary of question forms.
type QuestionKind string

const (
	// KindSingle is one prompt with exactly one correct choice.
	KindSingle QuestionKind = "single"
	// KindMulti is one prompt with one or more correct choices; a response is
	// correct only when it is exactly the correct set.
	KindMulti QuestionKind = "multi"
	// KindShort is a free-text prompt. This package does NOT grade it — see
	// Result.Determinate.
	KindShort QuestionKind = "short"
)

// QuestionKinds is the vocabulary as DATA.
var QuestionKinds = []QuestionKind{KindSingle, KindMulti, KindShort}

// ValidQuestionKind reports whether k is one of QuestionKinds.
func ValidQuestionKind(k QuestionKind) bool {
	for _, want := range QuestionKinds {
		if k == want {
			return true
		}
	}
	return false
}

// Choice is one selectable answer.
type Choice struct {
	ID   ID     `json:"id"`
	Text string `json:"text"`
}

// Question is one item of an assessment.
type Question struct {
	ID     ID           `json:"id"`
	Kind   QuestionKind `json:"kind"`
	Prompt string       `json:"prompt"`
	// Choices is empty for KindShort.
	Choices []Choice `json:"choices,omitempty"`
	// CorrectChoices names the correct choices BY ID, never by position.
	//
	// This is a deliberate departure from the index-based form. An index is a
	// number whose zero value is both "the first choice" and "absent", so any
	// encoder that elides zero values silently turns "the answer is the first
	// choice" into "there is no answer", and every such item becomes
	// unanswerable with nothing in the payload saying so. An ID has no such
	// collision: absent is absent. Reordering the choices also does not move the
	// answer, which an index does.
	CorrectChoices []ID `json:"correctChoices,omitempty"`
	// Points is this question's weight. It must be positive.
	Points      int    `json:"points"`
	Explanation string `json:"explanation,omitempty"`
	Ord         int    `json:"ord"`
}

// Assessment is the end-of-area test.
type Assessment struct {
	ID     ID     `json:"id"`
	AreaID ID     `json:"areaId"`
	Title  string `json:"title"`
	// RequiredLessons are the lessons that must be complete before this
	// assessment becomes available. It must be non-empty: a test gated on
	// nothing is available from the first second of the course, which is not a
	// test at the END of anything.
	RequiredLessons []ID `json:"requiredLessons"`
	// PassPercent is the inclusive threshold, 1..100.
	PassPercent int        `json:"passPercent"`
	Questions   []Question `json:"questions"`
}

// --- area and catalog ------------------------------------------------------

// Area is a subject area: lessons plus the test they lead to.
type Area struct {
	ID      ID     `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary,omitempty"`
	Ord     int    `json:"ord"`
	// Lessons is the ordered study material.
	Lessons []Lesson `json:"lessons"`
	// Assessment is the end-of-area test. It is a pointer so that an area under
	// construction can be represented, and Validate can then say so out loud
	// rather than the absence passing unnoticed.
	Assessment *Assessment `json:"assessment"`
}

// Catalog is the whole content set.
type Catalog struct {
	ID      ID     `json:"id"`
	Title   string `json:"title"`
	Version string `json:"version"`
	Areas   []Area `json:"areas"`
}

// Area returns the area with the given id.
func (c Catalog) Area(id ID) (Area, bool) {
	for _, a := range c.Areas {
		if a.ID == id {
			return a, true
		}
	}
	return Area{}, false
}

// Lesson returns the lesson with the given id, from anywhere in the catalog.
func (c Catalog) Lesson(id ID) (Lesson, bool) {
	for _, a := range c.Areas {
		for _, l := range a.Lessons {
			if l.ID == id {
				return l, true
			}
		}
	}
	return Lesson{}, false
}

// Lesson returns the area's lesson with the given id.
func (a Area) Lesson(id ID) (Lesson, bool) {
	for _, l := range a.Lessons {
		if l.ID == id {
			return l, true
		}
	}
	return Lesson{}, false
}

// MaxPoints is the total weight of the assessment's questions.
func (a Assessment) MaxPoints() int {
	total := 0
	for _, q := range a.Questions {
		total += q.Points
	}
	return total
}
