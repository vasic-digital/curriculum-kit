package curriculum

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// Errors returned by the assessment mechanism. They are distinct values
// because they mean different things to a caller: a locked assessment is a
// legitimate state of a correct request, while a missing one is a fault in the
// content.
var (
	// ErrNoAssessment means the area carries no test at all.
	ErrNoAssessment = errors.New("area has no assessment")
	// ErrAssessmentLocked means the gate refused: required lessons are not
	// complete. It is returned INSTEAD of a result, never alongside a zero one,
	// so a caller cannot mistake a refusal for a score of zero.
	ErrAssessmentLocked = errors.New("assessment is locked")
	// ErrUnknownQuestion means a response named a question the assessment does
	// not contain.
	ErrUnknownQuestion = errors.New("response names a question not in this assessment")
	// ErrDuplicateResponse means two responses addressed the same question.
	ErrDuplicateResponse = errors.New("more than one response for the same question")
)

// Availability is the answer to "may this learner start the test yet?".
type Availability struct {
	Available bool `json:"available"`
	// MissingLessons names the required lessons that are not complete, in the
	// order the assessment requires them. It is populated whenever Available is
	// false, so a consumer can tell the learner WHAT to finish rather than only
	// that something is unfinished.
	MissingLessons []ID `json:"missingLessons,omitempty"`
	// RequiredLessons is the full gate, so a consumer can render "3 of 5".
	RequiredLessons []ID `json:"requiredLessons"`
}

// AvailabilityOf reports whether the area's assessment is open to this learner.
func AvailabilityOf(a Area, p *Progress) (Availability, error) {
	if a.Assessment == nil {
		return Availability{}, fmt.Errorf("%w: %s", ErrNoAssessment, a.ID)
	}
	av := Availability{RequiredLessons: append([]ID(nil), a.Assessment.RequiredLessons...)}
	for _, id := range a.Assessment.RequiredLessons {
		if p.LessonState(id) != StateComplete {
			av.MissingLessons = append(av.MissingLessons, id)
		}
	}
	// A gate over an EMPTY required set would be open from the first second.
	// Validate rejects such an assessment as content; the mechanism refuses to
	// open it as well, so a consumer that skipped validation still cannot serve
	// an end-of-area test to a learner who has studied nothing.
	av.Available = len(av.RequiredLessons) > 0 && len(av.MissingLessons) == 0
	return av, nil
}

// Response is one learner answer.
type Response struct {
	QuestionID ID `json:"questionId"`
	// Chosen names the selected choices by ID, in any order.
	Chosen []ID `json:"chosen,omitempty"`
	// Text is the free-text answer for a KindShort question. This package does
	// not read it; it carries it so a consumer can put it in front of a human or
	// its own grader.
	Text string `json:"text,omitempty"`
}

// QuestionOutcome is the per-question result.
type QuestionOutcome struct {
	QuestionID ID `json:"questionId"`
	// Graded is false for a question this package cannot mark — today, every
	// KindShort question. Correct is then meaningless and is left false.
	Graded    bool `json:"graded"`
	Correct   bool `json:"correct"`
	Points    int  `json:"points"`
	MaxPoints int  `json:"maxPoints"`
	// Answered is false when no response addressed the question.
	Answered bool `json:"answered"`
	// Explanation is the authored explanation, carried through so a consumer can
	// show a learner why — which is what makes a result something to act on
	// rather than a number.
	Explanation string `json:"explanation,omitempty"`
	// ExpectedChoices is the correct set, carried through for the same reason.
	ExpectedChoices []ID `json:"expectedChoices,omitempty"`
}

// Result is the outcome of one submission.
//
// THE SCORE IS REPORTED OVER THE MARKED SCOPE, AND THE SCOPE IS STATED.
//
// This package marks selection questions and does not mark free text. Two
// different figures therefore exist, and collapsing them is what made an
// earlier revision of this type report a perfect paper as a failure:
//
//	MarkedPoints / MarkedPercent — the questions this package ACTUALLY marked.
//	                               This is the score. Passed is decided on it.
//	MaxPoints    / Percent       — the whole paper, free text included. Because
//	                               free text contributes 0 earned points against
//	                               a non-zero denominator, Percent is a LOWER
//	                               BOUND on the whole paper and never a score.
//
// Reporting Percent as the result told a learner who answered every markable
// question correctly that they had scored 47% and failed. That is not caution,
// it is a wrong number: it divides earned points by a denominator that includes
// questions nothing ever attempted to earn points on. The honest report names
// its scope — "you scored X of Y on the N questions that were marked; these M
// are yours to review against the model answer" — which is what these fields
// let a consumer render.
//
// Determinate is UNCHANGED and still means "some answers were not machine
// marked". It is no longer allowed to force Passed to false, because a pass
// over the marked scope is a claim about the marked scope and is true.
type Result struct {
	AssessmentID ID  `json:"assessmentId"`
	Points       int `json:"points"`
	MaxPoints    int `json:"maxPoints"`
	// Percent is Points*100/MaxPoints, truncated. When Determinate is false it
	// is a LOWER BOUND on the whole paper, not a score. Render MarkedPercent.
	Percent     int `json:"percent"`
	PassPercent int `json:"passPercent"`
	// MarkedPoints is the points available on the questions this package marked
	// — the denominator of the score. It excludes every question left to
	// self-review, so it is the only denominator the learner was actually
	// scored against.
	MarkedPoints int `json:"markedPoints"`
	// MarkedPercent is Points*100/MarkedPoints, truncated: the score, over a
	// stated scope. It is 0 when MarkedPoints is 0, which is the case of a
	// paper with nothing markable on it — see Passed.
	MarkedPercent int `json:"markedPercent"`
	// Passed is decided on MarkedPercent against PassPercent.
	//
	// A paper with NOTHING markable on it (MarkedPoints == 0) can never pass.
	// That is not the old defect returning: there is no evidence at all to pass
	// on, so a pass would be a claim about a measurement that did not happen.
	// It differs from the old rule precisely because a paper with SOME markable
	// questions is now judged on them instead of being refused outright.
	Passed bool `json:"passed"`
	// Determinate is false when at least one question could not be graded by
	// this package — today, every KindShort question. It reports the SCOPE of
	// the mark, and a consumer must surface it: a learner shown a pass without
	// being told free text went unmarked has been misinformed just as surely as
	// one shown a lower bound as a score.
	Determinate bool `json:"determinate"`
	// Ungraded counts the questions in that state, so a consumer can say how
	// many answers are the learner's own to review.
	Ungraded int               `json:"ungraded"`
	Outcomes []QuestionOutcome `json:"outcomes"`
	At       time.Time         `json:"at"`
}

// Attempt converts a result into the record kept in Progress.
func (r Result) Attempt() Attempt {
	return Attempt{
		AssessmentID:  r.AssessmentID,
		At:            r.At,
		Points:        r.Points,
		MaxPoints:     r.MaxPoints,
		Percent:       r.Percent,
		MarkedPoints:  r.MarkedPoints,
		MarkedPercent: r.MarkedPercent,
		Passed:        r.Passed,
		Determinate:   r.Determinate,
	}
}

// Grade marks a submission WITHOUT consulting the gate or touching progress.
// Use Submit for the gated path; Grade exists for a consumer that has already
// decided the test is open (a review screen, a re-mark against new answers).
func Grade(as Assessment, responses []Response, now time.Time) (Result, error) {
	byID := map[ID]Question{}
	for _, q := range as.Questions {
		byID[q.ID] = q
	}
	given := map[ID]Response{}
	for _, r := range responses {
		if _, ok := byID[r.QuestionID]; !ok {
			return Result{}, fmt.Errorf("%w: %s", ErrUnknownQuestion, r.QuestionID)
		}
		if _, dup := given[r.QuestionID]; dup {
			return Result{}, fmt.Errorf("%w: %s", ErrDuplicateResponse, r.QuestionID)
		}
		given[r.QuestionID] = r
	}

	res := Result{
		AssessmentID: as.ID,
		PassPercent:  as.PassPercent,
		MaxPoints:    as.MaxPoints(),
		Determinate:  true,
		At:           now,
	}
	for _, q := range as.Questions {
		r, answered := given[q.ID]
		out := QuestionOutcome{
			QuestionID:      q.ID,
			MaxPoints:       q.Points,
			Answered:        answered,
			Explanation:     q.Explanation,
			ExpectedChoices: append([]ID(nil), q.CorrectChoices...),
		}
		if q.Kind == KindShort {
			// Not a failure and not a pass: this package has no way to judge
			// prose, and pretending otherwise is the whole defect class the
			// three-valued rule exists to prevent.
			//
			// Its points are deliberately NOT added to MarkedPoints. A question
			// nothing tried to mark must not sit in the denominator of the mark.
			out.Graded = false
			res.Ungraded++
			res.Determinate = false
			res.Outcomes = append(res.Outcomes, out)
			continue
		}
		out.Graded = true
		res.MarkedPoints += q.Points
		if answered && sameSet(r.Chosen, q.CorrectChoices) {
			out.Correct = true
			out.Points = q.Points
			res.Points += q.Points
		}
		res.Outcomes = append(res.Outcomes, out)
	}

	if res.MaxPoints > 0 {
		res.Percent = res.Points * 100 / res.MaxPoints
	}
	if res.MarkedPoints > 0 {
		res.MarkedPercent = res.Points * 100 / res.MarkedPoints
	}
	// The pass is decided over the MARKED scope and the threshold.
	//
	// An assessment with nothing markable on it has MarkedPoints 0 and can
	// never pass, which is why MarkedPercent stays 0 rather than being defined
	// as 100 for an empty denominator — 100% of nothing is not a pass, it is an
	// absence of evidence. Determinate is deliberately NOT a term here: it
	// describes the scope of the mark, and a consumer must render it, but a
	// correct answer to every marked question is a pass over those questions
	// and refusing to say so was reporting a wrong number rather than a careful
	// one.
	res.Passed = res.MarkedPoints > 0 && res.MarkedPercent >= as.PassPercent
	return res, nil
}

// Submit is the gated path: it refuses unless the area's assessment is
// available to this learner, grades the submission, and records the attempt.
//
// It mutates p. Pass p.Clone() to score a hypothetical.
func Submit(a Area, p *Progress, responses []Response, now time.Time) (Result, error) {
	av, err := AvailabilityOf(a, p)
	if err != nil {
		return Result{}, err
	}
	if !av.Available {
		return Result{}, fmt.Errorf("%w: %d of %d required lesson(s) not complete: %v",
			ErrAssessmentLocked, len(av.MissingLessons), len(av.RequiredLessons), av.MissingLessons)
	}
	res, err := Grade(*a.Assessment, responses, now)
	if err != nil {
		return Result{}, err
	}
	p.RecordAttempt(res.Attempt())
	return res, nil
}

// sameSet reports whether two id lists hold exactly the same members,
// ignoring order and repeats. A multi-answer question is correct only on an
// exact match: crediting a superset would let "select every choice" pass.
func sameSet(a, b []ID) bool {
	norm := func(in []ID) []ID {
		seen := map[ID]bool{}
		out := []ID{}
		for _, v := range in {
			if !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
		return out
	}
	x, y := norm(a), norm(b)
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}
