package curriculum

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// LessonState is the closed vocabulary of what a learner has done with one
// lesson. It is closed because the gate reads it: a state outside the set is a
// value nothing can act on, and accepting it silently produces a row that can
// only ever move a denominator.
type LessonState string

const (
	// StateNotStarted is the zero value, so an absent row reads correctly.
	StateNotStarted LessonState = "not-started"
	// StateInProgress means opened but not finished. It does NOT satisfy a gate.
	StateInProgress LessonState = "in-progress"
	// StateComplete is the only state that satisfies an assessment gate.
	StateComplete LessonState = "complete"
)

// LessonStates is the vocabulary as DATA, so an error can name it.
var LessonStates = []LessonState{StateNotStarted, StateInProgress, StateComplete}

// ErrUnknownLessonState is returned by MarkLesson for a state outside
// LessonStates.
var ErrUnknownLessonState = errors.New("unknown lesson state")

// Attempt is one recorded run at an assessment.
type Attempt struct {
	AssessmentID ID        `json:"assessmentId"`
	At           time.Time `json:"at"`
	Points       int       `json:"points"`
	MaxPoints    int       `json:"maxPoints"`
	Percent      int       `json:"percent"`
	// MarkedPoints and MarkedPercent are the score and its denominator over the
	// questions that were actually marked — see Result, where the distinction
	// and the reason for it are set out. MarkedPercent is the figure to render;
	// Percent is a lower bound on the whole paper.
	//
	// A record written before this distinction existed decodes with
	// MarkedPoints 0. That is why BestAttempt requires MarkedPoints > 0 rather
	// than treating a zero as "scored nothing": an absent figure and a zero
	// score are different facts, and such a record simply carries no
	// marked-scope reading to rank.
	MarkedPoints  int  `json:"markedPoints"`
	MarkedPercent int  `json:"markedPercent"`
	Passed        bool `json:"passed"`
	// Determinate is false when the attempt contained questions this package
	// cannot grade. Percent is then a LOWER BOUND on the whole paper — see
	// Result. It does NOT force Passed to false.
	Determinate bool `json:"determinate"`
}

// Progress is one learner's state. It is a plain serialisable value with no
// storage of its own: a consumer persists it by whatever means it already has
// — a JSON column, a key-value store, browser storage — and hands it back.
// This package deliberately owns no persistence, because a persistence choice
// made here would be inherited by every consumer.
//
// The zero value is usable: an absent lesson reads as StateNotStarted.
type Progress struct {
	Lessons  map[ID]LessonState `json:"lessons,omitempty"`
	Attempts map[ID][]Attempt   `json:"attempts,omitempty"`
}

// NewProgress returns an empty, ready-to-use Progress.
func NewProgress() *Progress {
	return &Progress{Lessons: map[ID]LessonState{}, Attempts: map[ID][]Attempt{}}
}

// MarkLesson sets a lesson's state. An unrecognised state is rejected rather
// than stored: a value the gate cannot read is not progress, and writing it
// would leave the learner's record saying something no reader can act on.
func (p *Progress) MarkLesson(id ID, st LessonState) error {
	if id == "" {
		return errors.New("lesson id is empty")
	}
	if !validLessonState(st) {
		names := make([]string, 0, len(LessonStates))
		for _, s := range LessonStates {
			names = append(names, string(s))
		}
		return fmt.Errorf("%w %q (want one of: %s)", ErrUnknownLessonState, st, strings.Join(names, ", "))
	}
	if p.Lessons == nil {
		p.Lessons = map[ID]LessonState{}
	}
	p.Lessons[id] = st
	return nil
}

func validLessonState(st LessonState) bool {
	for _, want := range LessonStates {
		if st == want {
			return true
		}
	}
	return false
}

// LessonState returns the recorded state, or StateNotStarted when there is
// none.
func (p *Progress) LessonState(id ID) LessonState {
	if p == nil || p.Lessons == nil {
		return StateNotStarted
	}
	if st, ok := p.Lessons[id]; ok {
		return st
	}
	return StateNotStarted
}

// RecordAttempt appends an attempt.
func (p *Progress) RecordAttempt(a Attempt) {
	if p.Attempts == nil {
		p.Attempts = map[ID][]Attempt{}
	}
	p.Attempts[a.AssessmentID] = append(p.Attempts[a.AssessmentID], a)
}

// AttemptsFor returns every recorded attempt at one assessment, in the order
// they were recorded.
func (p *Progress) AttemptsFor(id ID) []Attempt {
	if p == nil || p.Attempts == nil {
		return nil
	}
	return p.Attempts[id]
}

// BestAttempt returns the highest-scoring attempt, ranked on MarkedPercent.
//
// IT USED TO SKIP EVERY INDETERMINATE ATTEMPT, and that was the Result defect
// one level down. On any assessment carrying a single free-text question every
// attempt is indeterminate, so the filter did not select a careful best result
// — it selected nothing, for ever, and a learner who had answered every marked
// question correctly had no best attempt at all. Ranking on the marked scope
// reports what was measured instead of discarding it.
//
// An attempt with MarkedPoints == 0 is skipped, and the two reasons are worth
// separating: an assessment with nothing markable on it carries no reading to
// rank, and a record written before this field existed decodes as 0 and
// likewise carries none. Neither is a score of zero.
//
// Determinate is still carried on the returned attempt, because the caller must
// be able to say that free text went unmarked.
func (p *Progress) BestAttempt(id ID) (Attempt, bool) {
	var best Attempt
	found := false
	for _, a := range p.AttemptsFor(id) {
		if a.MarkedPoints <= 0 {
			continue
		}
		if !found || a.MarkedPercent > best.MarkedPercent {
			best, found = a, true
		}
	}
	return best, found
}

// Clone returns a deep copy, so a consumer can evaluate a hypothetical without
// mutating the record it is holding.
func (p *Progress) Clone() *Progress {
	out := NewProgress()
	if p == nil {
		return out
	}
	for k, v := range p.Lessons {
		out.Lessons[k] = v
	}
	for k, v := range p.Attempts {
		out.Attempts[k] = append([]Attempt(nil), v...)
	}
	return out
}

// AreaCompletion is a per-area tally a consumer can render directly.
type AreaCompletion struct {
	AreaID           ID  `json:"areaId"`
	TotalLessons     int `json:"totalLessons"`
	CompleteLessons  int `json:"completeLessons"`
	InProgressLesson int `json:"inProgressLessons"`
	Percent          int `json:"percent"`
}

// Completion tallies one area's lessons.
func Completion(a Area, p *Progress) AreaCompletion {
	c := AreaCompletion{AreaID: a.ID, TotalLessons: len(a.Lessons)}
	for _, l := range a.Lessons {
		switch p.LessonState(l.ID) {
		case StateComplete:
			c.CompleteLessons++
		case StateInProgress:
			c.InProgressLesson++
		}
	}
	if c.TotalLessons > 0 {
		c.Percent = c.CompleteLessons * 100 / c.TotalLessons
	}
	return c
}
