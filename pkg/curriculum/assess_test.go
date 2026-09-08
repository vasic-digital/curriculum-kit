package curriculum

import (
	"errors"
	"testing"
	"time"
)

var at = time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)

func area1() Area { return goodCatalog().Areas[0] }

func TestAssessmentIsLockedUntilEveryRequiredLessonIsComplete(t *testing.T) {
	a := area1()
	p := NewProgress()

	av, err := AvailabilityOf(a, p)
	if err != nil {
		t.Fatal(err)
	}
	if av.Available {
		t.Fatal("assessment was available with no lesson complete")
	}
	if len(av.MissingLessons) != 2 {
		t.Fatalf("MissingLessons = %v, want both required lessons named", av.MissingLessons)
	}

	if err := p.MarkLesson("les-1", StateComplete); err != nil {
		t.Fatal(err)
	}
	av, _ = AvailabilityOf(a, p)
	if av.Available {
		t.Fatal("assessment was available with one of two lessons complete")
	}
	if len(av.MissingLessons) != 1 || av.MissingLessons[0] != "les-2" {
		t.Fatalf("MissingLessons = %v, want [les-2]", av.MissingLessons)
	}

	if err := p.MarkLesson("les-2", StateComplete); err != nil {
		t.Fatal(err)
	}
	av, _ = AvailabilityOf(a, p)
	if !av.Available {
		t.Fatalf("assessment still locked with both lessons complete: %v", av.MissingLessons)
	}
}

func TestInProgressDoesNotOpenTheGate(t *testing.T) {
	a := area1()
	p := NewProgress()
	_ = p.MarkLesson("les-1", StateComplete)
	_ = p.MarkLesson("les-2", StateInProgress)
	av, _ := AvailabilityOf(a, p)
	if av.Available {
		t.Fatal("an in-progress lesson opened the gate; only StateComplete may")
	}
}

func TestSubmitRefusesWhenLocked(t *testing.T) {
	a := area1()
	p := NewProgress()
	_, err := Submit(a, p, nil, at)
	if !errors.Is(err, ErrAssessmentLocked) {
		t.Fatalf("err = %v, want ErrAssessmentLocked", err)
	}
	if len(p.AttemptsFor("asm-1")) != 0 {
		t.Fatal("a refused submission was recorded as an attempt")
	}
}

// A refusal must not be reachable as a zero-valued Result: a caller that
// ignored the error would otherwise read "0%, not passed" and believe the
// learner sat the test and failed it.
func TestLockedSubmitReturnsNoResult(t *testing.T) {
	res, err := Submit(area1(), NewProgress(), nil, at)
	if err == nil {
		t.Fatal("want an error")
	}
	if res.AssessmentID != "" || res.MaxPoints != 0 || res.Determinate {
		t.Fatalf("a refused submission returned a populated result: %+v", res)
	}
}

func TestAreaWithNoAssessmentIsAnError(t *testing.T) {
	a := area1()
	a.Assessment = nil
	if _, err := AvailabilityOf(a, NewProgress()); !errors.Is(err, ErrNoAssessment) {
		t.Fatalf("err = %v, want ErrNoAssessment", err)
	}
}

// Validate rejects an empty required set as content; the mechanism must refuse
// it too, so a consumer that skipped validation cannot serve an end-of-area
// test to somebody who has studied nothing.
func TestEmptyGateDoesNotOpen(t *testing.T) {
	a := area1()
	a.Assessment.RequiredLessons = nil
	av, err := AvailabilityOf(a, NewProgress())
	if err != nil {
		t.Fatal(err)
	}
	if av.Available {
		t.Fatal("an assessment requiring no lessons was available; a gate over nothing must not open")
	}
}

func unlocked() (Area, *Progress) {
	a := area1()
	p := NewProgress()
	_ = p.MarkLesson("les-1", StateComplete)
	_ = p.MarkLesson("les-2", StateComplete)
	return a, p
}

func TestGradingScoresAndPasses(t *testing.T) {
	a, p := unlocked()
	res, err := Submit(a, p, []Response{
		{QuestionID: "q-1", Chosen: []ID{"c-1"}},
		{QuestionID: "q-2", Chosen: []ID{"c-4", "c-3"}}, // order must not matter
	}, at)
	if err != nil {
		t.Fatal(err)
	}
	if res.Points != 5 || res.MaxPoints != 5 || res.Percent != 100 {
		t.Fatalf("points %d/%d = %d%%, want 5/5 = 100%%", res.Points, res.MaxPoints, res.Percent)
	}
	if !res.Determinate {
		t.Fatal("Determinate = false with no free-text question in the assessment")
	}
	if !res.Passed {
		t.Fatal("Passed = false at 100%% against a 60%% threshold")
	}
	if got := p.AttemptsFor("asm-1"); len(got) != 1 {
		t.Fatalf("attempts = %d, want 1 recorded", len(got))
	}
}

func TestPartialCreditAndThreshold(t *testing.T) {
	a, p := unlocked()
	res, _ := Submit(a, p, []Response{
		{QuestionID: "q-1", Chosen: []ID{"c-1"}}, // 2 of 5
	}, at)
	if res.Points != 2 || res.Percent != 40 {
		t.Fatalf("points %d, percent %d; want 2 and 40", res.Points, res.Percent)
	}
	if res.Passed {
		t.Fatal("Passed = true at 40%% against a 60%% threshold")
	}
	if !res.Outcomes[1].Graded || res.Outcomes[1].Answered {
		t.Fatalf("unanswered question outcome = %+v; want graded and unanswered", res.Outcomes[1])
	}
}

// A superset must not pass: crediting it would let "select everything" score.
func TestMultiRequiresTheExactSet(t *testing.T) {
	a, p := unlocked()
	res, _ := Submit(a, p, []Response{
		{QuestionID: "q-2", Chosen: []ID{"c-3", "c-4", "c-5"}},
	}, at)
	if res.Outcomes[1].Correct {
		t.Fatal("selecting every choice was marked correct")
	}
}

func TestDuplicateChoiceIdsInAResponseAreNormalised(t *testing.T) {
	a, p := unlocked()
	res, _ := Submit(a, p, []Response{
		{QuestionID: "q-2", Chosen: []ID{"c-3", "c-3", "c-4"}},
	}, at)
	if !res.Outcomes[1].Correct {
		t.Fatal("a repeated selection of the same choice was marked wrong")
	}
}

func TestUnknownAndDuplicateResponsesAreRejected(t *testing.T) {
	a, p := unlocked()
	if _, err := Submit(a, p, []Response{{QuestionID: "q-nope"}}, at); !errors.Is(err, ErrUnknownQuestion) {
		t.Fatalf("err = %v, want ErrUnknownQuestion", err)
	}
	if _, err := Submit(a, p, []Response{{QuestionID: "q-1"}, {QuestionID: "q-1"}}, at); !errors.Is(err, ErrDuplicateResponse) {
		t.Fatalf("err = %v, want ErrDuplicateResponse", err)
	}
}

// The three-valued rule, applied to grading: a free-text question is an
// absence of evidence, and a pass must never be awarded over one.
func TestFreeTextMakesTheResultIndeterminateAndNeverPassing(t *testing.T) {
	a, p := unlocked()
	a.Assessment.Questions = append(a.Assessment.Questions, Question{
		ID: "q-3", Kind: KindShort, Prompt: "Explain the difference.", Points: 1,
	})
	res, err := Submit(a, p, []Response{
		{QuestionID: "q-1", Chosen: []ID{"c-1"}},
		{QuestionID: "q-2", Chosen: []ID{"c-3", "c-4"}},
		{QuestionID: "q-3", Text: "Some prose."},
	}, at)
	if err != nil {
		t.Fatal(err)
	}
	if res.Determinate {
		t.Fatal("Determinate = true with an ungradable question present")
	}
	if res.Ungraded != 1 {
		t.Fatalf("Ungraded = %d, want 1", res.Ungraded)
	}
	if res.Passed {
		t.Fatal("Passed = true over a question nothing graded — an unmeasured pass")
	}
	if res.Percent != 83 { // 5 of 6, truncated
		t.Fatalf("Percent = %d, want 83 as a LOWER BOUND", res.Percent)
	}
	if res.Outcomes[2].Graded {
		t.Fatal("the free-text outcome claims to have been graded")
	}
}

func TestAnEmptyAssessmentCanNeverPass(t *testing.T) {
	res, err := Grade(Assessment{ID: "asm-empty", PassPercent: 1}, nil, at)
	if err != nil {
		t.Fatal(err)
	}
	if res.Passed {
		t.Fatal("an assessment with no questions passed; 0 of 0 is not a score")
	}
	if res.MaxPoints != 0 || res.Percent != 0 {
		t.Fatalf("res = %+v, want zero points and zero percent", res)
	}
}

func TestResultCarriesWhatALearnerCanActOn(t *testing.T) {
	a, p := unlocked()
	res, _ := Submit(a, p, []Response{{QuestionID: "q-1", Chosen: []ID{"c-2"}}}, at)
	o := res.Outcomes[0]
	if o.Correct {
		t.Fatal("the wrong choice was marked correct")
	}
	if o.Explanation == "" {
		t.Fatal("the outcome carries no explanation; a bare score is not something to act on")
	}
	if len(o.ExpectedChoices) != 1 || o.ExpectedChoices[0] != "c-1" {
		t.Fatalf("ExpectedChoices = %v, want [c-1]", o.ExpectedChoices)
	}
}

func TestGradeDoesNotTouchProgress(t *testing.T) {
	a, p := unlocked()
	if _, err := Grade(*a.Assessment, nil, at); err != nil {
		t.Fatal(err)
	}
	if len(p.AttemptsFor("asm-1")) != 0 {
		t.Fatal("Grade recorded an attempt; only Submit may")
	}
}
